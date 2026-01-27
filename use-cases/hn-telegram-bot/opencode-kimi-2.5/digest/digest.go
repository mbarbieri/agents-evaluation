package digest

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"hn-telegram-bot/bot"
	"hn-telegram-bot/config"
	"hn-telegram-bot/hn"
	"hn-telegram-bot/ranker"
	"hn-telegram-bot/scraper"
	"hn-telegram-bot/storage"
	"hn-telegram-bot/summarizer"
)

// Dependencies holds all external dependencies for the digest workflow
type Dependencies struct {
	Storage    *storage.Storage
	HNClient   *hn.Client
	Scraper    *scraper.Scraper
	Summarizer *summarizer.Client
	Bot        *bot.Bot
	Logger     *slog.Logger
}

// Service orchestrates the digest workflow
type Service struct {
	deps   *Dependencies
	config *config.Config
}

// NewService creates a new digest service
func NewService(deps *Dependencies, cfg *config.Config) *Service {
	return &Service{
		deps:   deps,
		config: cfg,
	}
}

// Run executes the full digest workflow
func (s *Service) Run(ctx context.Context) error {
	s.deps.Logger.Info("Starting digest workflow")

	// Step 1: Apply decay to tag weights
	if err := s.applyDecay(); err != nil {
		s.deps.Logger.Error("Failed to apply decay", "error", err)
		// Continue anyway, non-fatal
	}

	// Step 2: Fetch top stories from HN
	targetCount := s.config.ArticleCount * 2 // Fetch extra for filtering
	storyIDs, err := s.deps.HNClient.GetTopStories(targetCount)
	if err != nil {
		return fmt.Errorf("failed to fetch top stories: %w", err)
	}

	s.deps.Logger.Info("Fetched stories from HN", "count", len(storyIDs))

	// Step 3: Filter out recent articles (sent in last 7 days)
	recentIDs, err := s.deps.Storage.GetRecentArticleIDs(7)
	if err != nil {
		s.deps.Logger.Error("Failed to get recent article IDs", "error", err)
		// Continue with empty filter
		recentIDs = []int64{}
	}

	recentIDSet := make(map[int64]bool)
	for _, id := range recentIDs {
		recentIDSet[id] = true
	}

	// Step 4: Process each story
	var processedArticles []ranker.Article
	for _, storyID := range storyIDs {
		if recentIDSet[storyID] {
			continue // Skip recently sent articles
		}

		article, err := s.processStory(ctx, storyID)
		if err != nil {
			s.deps.Logger.Error("Failed to process story",
				"story_id", storyID,
				"error", err,
			)
			continue // Skip failed articles
		}

		if article != nil {
			processedArticles = append(processedArticles, *article)
		}
	}

	s.deps.Logger.Info("Processed articles", "count", len(processedArticles))

	if len(processedArticles) == 0 {
		s.deps.Logger.Info("No articles to send")
		return nil
	}

	// Step 5: Get tag weights for ranking
	tagWeights, err := s.deps.Storage.GetAllTagWeights()
	if err != nil {
		s.deps.Logger.Error("Failed to get tag weights", "error", err)
		tagWeights = make(map[string]float64)
	}

	// Step 6: Rank articles
	r := ranker.New(tagWeights, s.config.TagDecayRate, s.config.MinTagWeight)
	rankedArticles := r.Rank(processedArticles)

	// Step 7: Send top N articles
	sendCount := s.config.ArticleCount
	if len(rankedArticles) < sendCount {
		sendCount = len(rankedArticles)
	}

	s.deps.Logger.Info("Sending articles", "count", sendCount)

	sentCount := 0
	for i := 0; i < sendCount; i++ {
		ra := rankedArticles[i]

		msgID, err := s.deps.Bot.SendArticle(
			ra.Title,
			ra.Summary,
			ra.HNScore,
			0, // Comment count not available in our model
			ra.URL,
			fmt.Sprintf("https://news.ycombinator.com/item?id=%d", ra.ID),
		)

		if err != nil {
			s.deps.Logger.Error("Failed to send article",
				"article_id", ra.ID,
				"error", err,
			)
			continue
		}

		// Persist article as sent
		sentAt := time.Now()
		if err := s.deps.Storage.MarkArticleSent(ra.ID, msgID, sentAt); err != nil {
			s.deps.Logger.Error("Failed to mark article as sent",
				"article_id", ra.ID,
				"error", err,
			)
			// Continue even if persistence fails
		}

		sentCount++
	}

	s.deps.Logger.Info("Digest workflow completed", "sent", sentCount)
	return nil
}

// processStory fetches and processes a single story
func (s *Service) processStory(ctx context.Context, storyID int64) (*ranker.Article, error) {
	// Fetch story details
	story, err := s.deps.HNClient.GetItem(storyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	// Skip non-story items
	if story.Type != "story" || story.URL == "" {
		return nil, nil
	}

	// Scrape article content
	content, err := s.deps.Scraper.Scrape(story.URL, 4000)
	if err != nil {
		s.deps.Logger.Warn("Failed to scrape article, using title",
			"story_id", storyID,
			"error", err,
		)
		content = story.Title
	}

	// Summarize content
	summary, err := s.deps.Summarizer.Summarize(content)
	if err != nil {
		return nil, fmt.Errorf("failed to summarize: %w", err)
	}

	// Save article to storage
	storageArticle := &storage.Article{
		ID:        story.ID,
		Title:     story.Title,
		URL:       story.URL,
		Summary:   summary.Summary,
		Tags:      summary.Tags,
		HNScore:   story.Score,
		FetchedAt: time.Now(),
	}

	if err := s.deps.Storage.SaveArticle(storageArticle); err != nil {
		s.deps.Logger.Error("Failed to save article", "error", err)
		// Continue even if save fails
	}

	return &ranker.Article{
		ID:      story.ID,
		Title:   story.Title,
		URL:     story.URL,
		Summary: summary.Summary,
		Tags:    summary.Tags,
		HNScore: story.Score,
	}, nil
}

// applyDecay reduces all tag weights by the decay rate
func (s *Service) applyDecay() error {
	tagWeights, err := s.deps.Storage.GetAllTagWeights()
	if err != nil {
		return fmt.Errorf("failed to get tag weights: %w", err)
	}

	decayRate := s.config.TagDecayRate
	minWeight := s.config.MinTagWeight

	for tag, weight := range tagWeights {
		newWeight := weight * (1 - decayRate)
		if newWeight < minWeight {
			newWeight = minWeight
		}

		// Get current count (we don't change it during decay)
		_, count, _ := s.deps.Storage.GetTagWeight(tag)

		if err := s.deps.Storage.UpsertTagWeight(tag, newWeight, count); err != nil {
			s.deps.Logger.Error("Failed to update tag weight during decay",
				"tag", tag,
				"error", err,
			)
		}
	}

	return nil
}

// UpdateSettings updates the digest service settings
func (s *Service) UpdateSettings(digestTime string, articleCount int) error {
	if digestTime != "" {
		s.config.DigestTime = digestTime
	}
	if articleCount > 0 {
		s.config.ArticleCount = articleCount
	}

	return nil
}

// GetSettings returns current settings
func (s *Service) GetSettings() (string, int) {
	return s.config.DigestTime, s.config.ArticleCount
}

// LoadSettingsFromStorage loads settings from storage into config
func (s *Service) LoadSettingsFromStorage() error {
	if dt, err := s.deps.Storage.GetSetting("digest_time"); err == nil && dt != "" {
		s.config.DigestTime = dt
	}

	if ac, err := s.deps.Storage.GetSetting("article_count"); err == nil && ac != "" {
		if count, err := strconv.Atoi(ac); err == nil {
			s.config.ArticleCount = count
		}
	}

	return nil
}
