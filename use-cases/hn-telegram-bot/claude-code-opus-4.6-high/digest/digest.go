package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
)

// HNClient fetches stories and items from Hacker News.
type HNClient interface {
	TopStories(ctx context.Context, limit int) ([]int, error)
	GetItem(ctx context.Context, id int) (*HNItem, error)
}

// HNItem represents a Hacker News item with fields needed by the digest.
type HNItem struct {
	ID          int
	Title       string
	URL         string
	Score       int
	Descendants int
}

// ContentScraper extracts readable content from URLs.
type ContentScraper interface {
	Scrape(ctx context.Context, url string) (string, error)
}

// ArticleSummarizer generates summaries and tags.
type ArticleSummarizer interface {
	Summarize(ctx context.Context, title, content string) (*SummaryResult, error)
}

// SummaryResult holds summary and tags from AI.
type SummaryResult struct {
	Summary string
	Tags    []string
}

// RankableArticle holds data needed for ranking.
type RankableArticle struct {
	ID      int
	Tags    []string
	HNScore int
	Score   float64
}

// ArticleSender sends formatted articles to the user.
type ArticleSender interface {
	SendHTML(chatID int64, text string) (int, error)
}

// Storage provides persistence operations for the digest.
type Storage interface {
	GetRecentSentArticleIDs(days int) ([]int, error)
	SaveArticle(article *StoredArticle) error
	MarkSent(articleID int, telegramMsgID int) error
	ApplyDecay(decayRate, minWeight float64) error
	GetTagWeights() ([]TagWeightEntry, error)
}

// StoredArticle is the article representation for storage.
type StoredArticle struct {
	ID        int
	Title     string
	URL       string
	Summary   string
	Tags      string
	Score     int
	FetchedAt int64
}

// TagWeightEntry represents a tag weight from storage.
type TagWeightEntry struct {
	Tag    string
	Weight float64
}

// Config holds digest workflow configuration.
type Config struct {
	ChatID       int64
	ArticleCount int
	DecayRate    float64
	MinWeight    float64
}

// Runner orchestrates the end-to-end digest workflow.
type Runner struct {
	hn         HNClient
	scraper    ContentScraper
	summarizer ArticleSummarizer
	sender     ArticleSender
	storage    Storage
	config     Config
}

// NewRunner creates a Runner with all dependencies.
func NewRunner(hn HNClient, scraper ContentScraper, summarizer ArticleSummarizer, sender ArticleSender, storage Storage, cfg Config) *Runner {
	return &Runner{
		hn:         hn,
		scraper:    scraper,
		summarizer: summarizer,
		sender:     sender,
		storage:    storage,
		config:     cfg,
	}
}

// UpdateConfig updates the digest configuration.
func (r *Runner) UpdateConfig(cfg Config) {
	r.config = cfg
}

// processedArticle holds all data for a fully processed article.
type processedArticle struct {
	id          int
	title       string
	url         string
	summary     string
	tags        []string
	hnScore     int
	descendants int
}

// Run executes the complete digest workflow.
func (r *Runner) Run(ctx context.Context) error {
	slog.Info("digest cycle starting", "article_count", r.config.ArticleCount)

	// 1. Apply decay
	if err := r.storage.ApplyDecay(r.config.DecayRate, r.config.MinWeight); err != nil {
		slog.Error("failed to apply decay", "error", err)
	}

	// 2. Fetch 2x stories
	fetchCount := r.config.ArticleCount * 2
	storyIDs, err := r.hn.TopStories(ctx, fetchCount)
	if err != nil {
		return fmt.Errorf("fetching top stories: %w", err)
	}
	slog.Info("fetched story IDs", "count", len(storyIDs))

	// 3. Filter recently sent
	recentIDs, err := r.storage.GetRecentSentArticleIDs(7)
	if err != nil {
		slog.Error("failed to get recent article IDs", "error", err)
	}
	recentSet := make(map[int]bool, len(recentIDs))
	for _, id := range recentIDs {
		recentSet[id] = true
	}

	var filteredIDs []int
	for _, id := range storyIDs {
		if !recentSet[id] {
			filteredIDs = append(filteredIDs, id)
		}
	}
	slog.Info("filtered stories", "before", len(storyIDs), "after", len(filteredIDs))

	// 4. Process each story
	var processed []processedArticle
	for _, id := range filteredIDs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		item, err := r.hn.GetItem(ctx, id)
		if err != nil {
			slog.Error("failed to fetch item", "id", id, "error", err)
			continue
		}

		// Scrape content (fallback to title)
		content := item.Title
		if item.URL != "" {
			scraped, err := r.scraper.Scrape(ctx, item.URL)
			if err != nil {
				slog.Warn("scrape failed, using title", "url", item.URL, "error", err)
			} else if scraped != "" {
				content = scraped
			}
		}

		// Summarize
		result, err := r.summarizer.Summarize(ctx, item.Title, content)
		if err != nil {
			slog.Warn("summarization failed, skipping article", "id", id, "error", err)
			continue
		}

		processed = append(processed, processedArticle{
			id:          item.ID,
			title:       item.Title,
			url:         item.URL,
			summary:     result.Summary,
			tags:        result.Tags,
			hnScore:     item.Score,
			descendants: item.Descendants,
		})
	}
	slog.Info("processed articles", "count", len(processed))

	// 5. Rank articles
	tagWeights, err := r.storage.GetTagWeights()
	if err != nil {
		slog.Error("failed to get tag weights", "error", err)
	}
	weightMap := make(map[string]float64, len(tagWeights))
	for _, tw := range tagWeights {
		weightMap[tw.Tag] = tw.Weight
	}

	rankable := make([]RankableArticle, len(processed))
	for i, p := range processed {
		rankable[i] = RankableArticle{
			ID:      p.id,
			Tags:    p.tags,
			HNScore: p.hnScore,
		}
	}

	ranked := rankArticles(rankable, weightMap)

	// Build index map for ranked -> processed lookup
	processedMap := make(map[int]processedArticle, len(processed))
	for _, p := range processed {
		processedMap[p.id] = p
	}

	// 6. Send top N
	count := min(r.config.ArticleCount, len(ranked))

	sent := 0
	for i := range count {
		article := processedMap[ranked[i].ID]

		msg := FormatArticle(article.title, article.summary, article.hnScore, article.descendants, article.id, article.url)

		msgID, err := r.sender.SendHTML(r.config.ChatID, msg)
		if err != nil {
			slog.Error("failed to send article", "id", article.id, "error", err)
			continue
		}

		// 7. Persist
		tagsJSON, _ := json.Marshal(article.tags)
		if err := r.storage.SaveArticle(&StoredArticle{
			ID:      article.id,
			Title:   article.title,
			URL:     article.url,
			Summary: article.summary,
			Tags:    string(tagsJSON),
			Score:   article.hnScore,
		}); err != nil {
			slog.Error("failed to save article", "id", article.id, "error", err)
		}

		if err := r.storage.MarkSent(article.id, msgID); err != nil {
			slog.Error("failed to mark article sent", "id", article.id, "error", err)
		}
		sent++
	}

	slog.Info("digest cycle complete", "sent", sent)
	return nil
}

// FormatArticle formats an article for Telegram using HTML.
func FormatArticle(title, summary string, score, comments, id int, url string) string {
	title = escapeHTML(title)
	summary = escapeHTML(summary)
	hnURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", id)

	var sb strings.Builder
	fmt.Fprintf(&sb, "📰 <b>%s</b>\n\n", title)
	fmt.Fprintf(&sb, "<i>%s</i>\n\n", summary)
	fmt.Fprintf(&sb, "⬆️ %d points | 💬 %d comments\n", score, comments)
	fmt.Fprintf(&sb, "🔗 <a href=\"%s\">Article</a> | <a href=\"%s\">HN Discussion</a>", url, hnURL)
	return sb.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// rankArticles scores and sorts articles by blended preference score.
// Formula: score = (tag_score * 0.7) + (hn_score * 0.3) where hn_score = log10(score+1)
func rankArticles(articles []RankableArticle, weights map[string]float64) []RankableArticle {
	for i := range articles {
		tagScore := 0.0
		for _, tag := range articles[i].Tags {
			if w, ok := weights[tag]; ok {
				tagScore += w
			}
		}
		hnScore := math.Log10(float64(articles[i].HNScore) + 1)
		articles[i].Score = (tagScore * 0.7) + (hnScore * 0.3)
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Score > articles[j].Score
	})

	return articles
}
