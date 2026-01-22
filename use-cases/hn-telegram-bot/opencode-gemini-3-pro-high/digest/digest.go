package digest

import (
	"fmt"
	"log"
	"time"

	"hn-telegram-bot/hn"
	"hn-telegram-bot/ranker"
	"hn-telegram-bot/storage"
)

type HNClient interface {
	GetTopStories() ([]int, error)
	GetItem(id int) (*hn.Item, error)
}

type Scraper interface {
	Scrape(url string) (string, error)
}

type Summarizer interface {
	Summarize(text string) (string, []string, error)
}

type Storage interface {
	ApplyTagDecay(rate, min float64) error
	GetRecentSentArticleIDs(d time.Duration) ([]int, error)
	GetTagWeights() (map[string]float64, error)
	SaveArticle(a storage.Article) error
	MarkArticleSent(id, msgID int) error
}

type Sender interface {
	SendArticle(a storage.Article) (int, error)
}

type Digest struct {
	storage      Storage
	hn           HNClient
	scraper      Scraper
	summarizer   Summarizer
	sender       Sender
	ArticleCount int
	TagDecayRate float64
	MinTagWeight float64
}

func New(store Storage, hn HNClient, scraper Scraper, sum Summarizer, sender Sender) *Digest {
	return &Digest{
		storage:      store,
		hn:           hn,
		scraper:      scraper,
		summarizer:   sum,
		sender:       sender,
		ArticleCount: 30, // Default
		TagDecayRate: 0.02,
		MinTagWeight: 0.1,
	}
}

func (d *Digest) Run() error {
	log.Println("Starting digest cycle...")

	// 1. Apply Decay
	if err := d.storage.ApplyTagDecay(d.TagDecayRate, d.MinTagWeight); err != nil {
		log.Printf("Failed to apply decay: %v", err)
		// Continue anyway?
	}

	// 2. Fetch Stories
	ids, err := d.hn.GetTopStories()
	if err != nil {
		return fmt.Errorf("failed to fetch top stories: %w", err)
	}

	// Limit to 2x count
	limit := d.ArticleCount * 2
	if len(ids) > limit {
		ids = ids[:limit]
	}

	// 3. Filter Recent
	recentIDs, err := d.storage.GetRecentSentArticleIDs(7 * 24 * time.Hour)
	if err != nil {
		log.Printf("Failed to get recent articles: %v", err)
		recentIDs = []int{}
	}
	recentMap := make(map[int]bool)
	for _, id := range recentIDs {
		recentMap[id] = true
	}

	var candidates []storage.Article

	// 4. Process Stories
	for _, id := range ids {
		if recentMap[id] {
			continue
		}

		item, err := d.hn.GetItem(id)
		if err != nil {
			log.Printf("Failed to fetch item %d: %v", id, err)
			continue
		}

		if item.Type != "story" || item.Deleted || item.Dead {
			continue
		}

		// Scrape
		content, err := d.scraper.Scrape(item.URL)
		if err != nil {
			log.Printf("Scrape failed for %s: %v. Using title.", item.URL, err)
			content = item.Title // Fallback
		}
		if content == "" {
			content = item.Title
		}

		// Summarize
		summary, tags, err := d.summarizer.Summarize(content)
		if err != nil {
			log.Printf("Summarize failed for %d: %v. Skipping.", id, err)
			continue
		}

		candidates = append(candidates, storage.Article{
			ID:        item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Summary:   summary,
			Tags:      tags,
			Score:     item.Score,
			FetchedAt: time.Now(),
		})
	}

	// 5. Rank
	weights, err := d.storage.GetTagWeights()
	if err != nil {
		log.Printf("Failed to get weights: %v", err)
		weights = make(map[string]float64)
	}

	ranked := ranker.Rank(candidates, weights)

	// 6. Send Top N
	sentCount := 0
	for _, art := range ranked {
		if sentCount >= d.ArticleCount {
			break
		}

		msgID, err := d.sender.SendArticle(art)
		if err != nil {
			log.Printf("Failed to send article %d: %v", art.ID, err)
			continue
		}

		// 7. Persist
		art.MsgID = msgID
		art.SentAt = time.Now()

		if err := d.storage.SaveArticle(art); err != nil {
			log.Printf("Failed to save article %d: %v", art.ID, err)
		}
		if err := d.storage.MarkArticleSent(art.ID, msgID); err != nil {
			log.Printf("Failed to mark article %d sent: %v", art.ID, err)
		}

		sentCount++
	}

	log.Printf("Digest completed. Sent %d articles.", sentCount)
	return nil
}
