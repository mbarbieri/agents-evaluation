package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Storage handles database operations
type Storage struct {
	db *sql.DB
}

// Article represents a Hacker News article
type Article struct {
	ID        int
	Title     string
	URL       string
	Summary   string
	Tags      []string
	HNScore   int
	FetchedAt time.Time
	SentAt    *time.Time
	MessageID *int
}

// TagWeight represents a tag with its learned weight
type TagWeight struct {
	Tag    string
	Weight float64
	Count  int
}

// New creates a new Storage instance
func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}

	return storage, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// initTables creates the database tables if they don't exist
func (s *Storage) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT,
			tags TEXT, -- JSON array
			hn_score INTEGER,
			fetched_at DATETIME NOT NULL,
			sent_at DATETIME,
			message_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL NOT NULL,
			count INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// SaveArticle inserts or updates an article
func (s *Storage) SaveArticle(article Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return err
	}

	query := `INSERT OR REPLACE INTO articles (id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.Exec(query, article.ID, article.Title, article.URL, article.Summary, string(tagsJSON),
		article.HNScore, article.FetchedAt, article.SentAt, article.MessageID)
	return err
}

// GetArticle retrieves an article by ID
func (s *Storage) GetArticle(id int) (Article, error) {
	var article Article
	var tagsJSON string
	var sentAt sql.NullTime
	var messageID sql.NullInt64

	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id FROM articles WHERE id = ?`
	row := s.db.QueryRow(query, id)
	err := row.Scan(&article.ID, &article.Title, &article.URL, &article.Summary, &tagsJSON,
		&article.HNScore, &article.FetchedAt, &sentAt, &messageID)
	if err != nil {
		return article, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return article, err
	}

	if sentAt.Valid {
		article.SentAt = &sentAt.Time
	}
	if messageID.Valid {
		msgID := int(messageID.Int64)
		article.MessageID = &msgID
	}

	return article, nil
}

// UpdateArticleSent updates the sent timestamp and message ID for an article
func (s *Storage) UpdateArticleSent(id int, sentAt time.Time, messageID int) error {
	query := `UPDATE articles SET sent_at = ?, message_id = ? WHERE id = ?`
	_, err := s.db.Exec(query, sentAt, messageID, id)
	return err
}

// GetRecentArticles returns articles sent within the last N days
func (s *Storage) GetRecentArticles(days int) ([]Article, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id FROM articles
	          WHERE sent_at IS NOT NULL AND sent_at > ?`

	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var article Article
		var tagsJSON string
		var sentAt sql.NullTime
		var messageID sql.NullInt64

		err := rows.Scan(&article.ID, &article.Title, &article.URL, &article.Summary, &tagsJSON,
			&article.HNScore, &article.FetchedAt, &sentAt, &messageID)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
			return nil, err
		}

		if sentAt.Valid {
			article.SentAt = &sentAt.Time
		}
		if messageID.Valid {
			msgID := int(messageID.Int64)
			article.MessageID = &msgID
		}

		articles = append(articles, article)
	}

	return articles, rows.Err()
}

// AddLike records a like for an article (idempotent)
func (s *Storage) AddLike(articleID int) error {
	query := `INSERT OR IGNORE INTO likes (article_id, liked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, articleID, time.Now())
	return err
}

// IsLiked checks if an article has been liked
func (s *Storage) IsLiked(articleID int) (bool, error) {
	var count int
	query := `SELECT count(*) FROM likes WHERE article_id = ?`
	err := s.db.QueryRow(query, articleID).Scan(&count)
	return count > 0, err
}

// GetTagWeights returns all tag weights
func (s *Storage) GetTagWeights() (map[string]TagWeight, error) {
	query := `SELECT tag, weight, count FROM tag_weights`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weights := make(map[string]TagWeight)
	for rows.Next() {
		var tw TagWeight
		err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count)
		if err != nil {
			return nil, err
		}
		weights[tw.Tag] = tw
	}

	return weights, rows.Err()
}

// UpdateTagWeight inserts or updates a tag weight
func (s *Storage) UpdateTagWeight(tag string, weight float64, count int) error {
	query := `INSERT OR REPLACE INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, tag, weight, count)
	return err
}

// DecayTagWeights applies decay to all tag weights
func (s *Storage) DecayTagWeights(decayRate, minWeight float64) error {
	query := `UPDATE tag_weights SET weight = MAX(weight * ?, ?)`
	_, err := s.db.Exec(query, 1-decayRate, minWeight)
	return err
}

// GetSetting retrieves a setting value
func (s *Storage) GetSetting(key string) (string, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = ?`
	err := s.db.QueryRow(query, key).Scan(&value)
	return value, err
}

// SetSetting stores a setting value
func (s *Storage) SetSetting(key, value string) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
	_, err := s.db.Exec(query, key, value)
	return err
}
