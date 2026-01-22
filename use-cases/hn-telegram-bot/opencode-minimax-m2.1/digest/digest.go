package digest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"hn-bot/bot"
	"hn-bot/config"
	"hn-bot/hn"
	"hn-bot/ranker"
	"hn-bot/scraper"
	"hn-bot/storage"
	"hn-bot/summarizer"
)

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      []string
	TagsJSON  string
	Score     int
	FetchedAt time.Time
	SentAt    time.Time
	MessageID int64
}

type Digest struct {
	cfg        *config.Config
	storage    *storage.Storage
	hnClient   *hn.Client
	scraper    *scraper.Scraper
	summarizer *summarizer.Summarizer
	ranker     *ranker.Ranker
	logger     *slog.Logger
}

func NewDigest(
	cfg *config.Config,
	storage *storage.Storage,
	logger *slog.Logger,
) *Digest {
	return &Digest{
		cfg:        cfg,
		storage:    storage,
		hnClient:   hn.NewClient(""),
		scraper:    scraper.NewScraper(cfg.FetchTimeoutSecs),
		summarizer: summarizer.NewSummarizer("", cfg.GeminiModel, cfg.GeminiAPIKey),
		ranker:     ranker.NewRanker(),
		logger:     logger,
	}
}

func (d *Digest) Run() ([]Article, error) {
	d.logger.Info("Starting digest generation")

	if err := d.storage.DecayAllTags(d.cfg.TagDecayRate, d.cfg.MinTagWeight); err != nil {
		d.logger.Warn("Failed to decay tags", "error", err)
	}

	storyIDs, err := d.hnClient.GetTopStories()
	if err != nil {
		return nil, fmt.Errorf("failed to get top stories: %w", err)
	}

	recentArticles, err := d.storage.GetRecentArticles(7)
	if err != nil {
		d.logger.Warn("Failed to get recent articles", "error", err)
		recentArticles = nil
	}

	recentIDs := make(map[int64]bool)
	for _, a := range recentArticles {
		recentIDs[a.ID] = true
	}

	fetchCount := d.cfg.ArticleCount * 2
	if len(storyIDs) < fetchCount {
		fetchCount = len(storyIDs)
	}

	var articles []Article
	for _, id := range storyIDs[:fetchCount] {
		if recentIDs[id] {
			continue
		}

		item, err := d.hnClient.GetItem(id)
		if err != nil {
			d.logger.Debug("Failed to get item", "id", id, "error", err)
			continue
		}

		if item.Type != "story" || item.URL == "" {
			continue
		}

		article, err := d.processArticle(item)
		if err != nil {
			d.logger.Debug("Failed to process article", "id", id, "error", err)
			continue
		}

		articles = append(articles, *article)
	}

	tagWeights, err := d.getTagWeights()
	if err != nil {
		d.logger.Warn("Failed to get tag weights", "error", err)
		tagWeights = make(map[string]float64)
	}

	rankedArticles := make([]ranker.Article, len(articles))
	for i, a := range articles {
		rankedArticles[i] = ranker.Article{
			ID:      a.ID,
			Title:   a.Title,
			URL:     a.URL,
			Summary: a.Summary,
			Tags:    a.Tags,
			Score:   a.Score,
		}
	}

	rankedArticles = d.ranker.Rank(rankedArticles, tagWeights)

	result := make([]Article, 0, d.cfg.ArticleCount)
	for _, a := range rankedArticles {
		if len(result) >= d.cfg.ArticleCount {
			break
		}
		tagsJSON, _ := json.Marshal(a.Tags)
		result = append(result, Article{
			ID:       a.ID,
			Title:    a.Title,
			URL:      a.URL,
			Summary:  a.Summary,
			Tags:     a.Tags,
			TagsJSON: string(tagsJSON),
			Score:    a.Score,
		})
	}

	d.logger.Info("Digest generation complete", "articles_count", len(result))
	return result, nil
}

func (d *Digest) processArticle(item *hn.Item) (*Article, error) {
	var content string
	var err error

	if item.URL != "" {
		content, err = d.scraper.Scrape(item.URL)
		if err != nil {
			d.logger.Debug("Failed to scrape article, using title", "url", item.URL, "error", err)
			content = item.Title
		}
	} else {
		content = item.Title
	}

	var summary string
	var tags []string

	if content != "" {
		result, err := d.summarizer.Summarize(item.Title, content)
		if err != nil {
			d.logger.Debug("Failed to summarize article", "error", err)
			summary = ""
			tags = []string{}
		} else {
			summary = result.Summary
			tags = result.Tags
		}
	}

	tagsJSON, _ := json.Marshal(tags)

	return &Article{
		ID:        item.ID,
		Title:     item.Title,
		URL:       item.URL,
		Summary:   summary,
		Tags:      tags,
		TagsJSON:  string(tagsJSON),
		Score:     item.Score,
		FetchedAt: time.Now(),
	}, nil
}

func (d *Digest) getTagWeights() (map[string]float64, error) {
	tags, err := d.storage.GetTagsByWeight()
	if err != nil {
		return nil, err
	}

	weights := make(map[string]float64)
	for _, t := range tags {
		weights[t.Name] = t.Weight
	}

	return weights, nil
}

func (d *Digest) SaveArticle(article *Article, messageID int64) error {
	tagsJSON, _ := json.Marshal(article.Tags)

	now := time.Now()
	storageArticle := &storage.Article{
		ID:        article.ID,
		Title:     article.Title,
		URL:       article.URL,
		Summary:   article.Summary,
		Tags:      string(tagsJSON),
		Score:     article.Score,
		FetchedAt: article.FetchedAt,
		SentAt:    storage.SentAtTime(now),
		MessageID: storage.MessageID(messageID),
	}

	return d.storage.SaveArticle(storageArticle)
}

func (d *Digest) BoostTags(article *Article) error {
	for _, tag := range article.Tags {
		current, err := d.storage.GetTagWeight(tag)
		if err != nil {
			d.storage.UpsertTagWeight(tag, 1.0+d.cfg.TagBoostOnLike, 1)
		} else {
			d.storage.UpsertTagWeight(tag, current.Weight+d.cfg.TagBoostOnLike, current.Count+1)
		}
	}
	return nil
}

func (d *Digest) GetSettings() (string, int, error) {
	digestTime := d.cfg.DigestTime
	articleCount := d.cfg.ArticleCount

	if storedTime, err := d.storage.GetSetting("digest_time"); err == nil && storedTime != "" {
		digestTime = storedTime
	}
	if storedCount, err := d.storage.GetSetting("article_count"); err == nil && storedCount != "" {
		fmt.Sscanf(storedCount, "%d", &articleCount)
	}

	return digestTime, articleCount, nil
}

func (d *Digest) UpdateSetting(key, value string) error {
	switch key {
	case "time":
		if err := d.storage.SetSetting("digest_time", value); err != nil {
			return err
		}
	case "count":
		if err := d.storage.SetSetting("article_count", value); err != nil {
			return err
		}
	}
	return nil
}

func (d *Digest) GetStats() ([]struct {
	Name   string
	Weight float64
}, int, error) {
	tags, err := d.storage.GetTagsByWeight()
	if err != nil {
		return nil, 0, err
	}

	likeCount, err := d.storage.GetLikeCount()
	if err != nil {
		return nil, 0, err
	}

	result := make([]struct {
		Name   string
		Weight float64
	}, len(tags))
	for i, t := range tags {
		result[i].Name = t.Name
		result[i].Weight = t.Weight
	}

	return result, likeCount, nil
}

func (d *Digest) FormatArticleMessage(article *Article, messageID int64) string {
	return bot.FormatArticleMessage(
		"📰",
		article.Title,
		article.Summary,
		article.Score,
		0,
		article.URL,
		article.ID,
	)
}
