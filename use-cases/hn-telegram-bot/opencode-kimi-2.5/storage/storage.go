package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"modernc.org/sqlite"
)

// Article represents a Hacker News article with its metadata
type Article struct {
	ID            int64
	Title         string
	URL           string
	Summary       string
	Tags          []string
	HNScore       int
	FetchedAt     time.Time
	SentAt        *time.Time
	TelegramMsgID int64
}

// TagWeight represents a learned preference for a content category
type TagWeight struct {
	Name   string
	Weight float64
	Count  int
}

// Storage handles all database operations
type Storage struct {
	db *sql.DB
}

// New creates a new Storage instance with initialized schema
func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return storage, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// createSchema creates the database tables if they don't exist
func (s *Storage) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		summary TEXT,
		tags TEXT,
		hn_score INTEGER,
		fetched_at DATETIME NOT NULL,
		sent_at DATETIME,
		telegram_msg_id INTEGER
	);

	CREATE TABLE IF NOT EXISTS likes (
		article_id INTEGER PRIMARY KEY,
		liked_at DATETIME NOT NULL,
		FOREIGN KEY (article_id) REFERENCES articles(id)
	);

	CREATE TABLE IF NOT EXISTS tag_weights (
		tag TEXT PRIMARY KEY,
		weight REAL NOT NULL,
		count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at);
	CREATE INDEX IF NOT EXISTS idx_articles_telegram_msg_id ON articles(telegram_msg_id);
	CREATE INDEX IF NOT EXISTS idx_likes_liked_at ON likes(liked_at);
	CREATE INDEX IF NOT EXISTS idx_tag_weights_weight ON tag_weights(weight DESC);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// SaveArticle saves or updates an article
func (s *Storage) SaveArticle(article *Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		INSERT INTO articles (id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			url = excluded.url,
			summary = excluded.summary,
			tags = excluded.tags,
			hn_score = excluded.hn_score,
			fetched_at = excluded.fetched_at
	`

	_, err = s.db.Exec(query,
		article.ID, article.Title, article.URL, article.Summary,
		string(tagsJSON), article.HNScore, article.FetchedAt,
		article.SentAt, article.TelegramMsgID,
	)

	if err != nil {
		return fmt.Errorf("failed to save article: %w", err)
	}

	return nil
}

// GetArticle retrieves an article by ID
func (s *Storage) GetArticle(id int64) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id FROM articles WHERE id = ?`

	row := s.db.QueryRow(query, id)
	return s.scanArticle(row)
}

// MarkArticleSent marks an article as sent with timestamp and message ID
func (s *Storage) MarkArticleSent(articleID int64, msgID int64, sentAt time.Time) error {
	result, err := s.db.Exec(
		"UPDATE articles SET sent_at = ?, telegram_msg_id = ? WHERE id = ?",
		sentAt, msgID, articleID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark article as sent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("article not found: %d", articleID)
	}

	return nil
}

// GetRecentArticleIDs returns IDs of articles sent within the last N days
func (s *Storage) GetRecentArticleIDs(days int) ([]int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	query := `SELECT id FROM articles WHERE sent_at >= ? ORDER BY sent_at DESC`
	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent articles: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan article ID: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// FindArticleByMessageID finds an article by its Telegram message ID
func (s *Storage) FindArticleByMessageID(msgID int64) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id FROM articles WHERE telegram_msg_id = ?`

	row := s.db.QueryRow(query, msgID)
	article, err := s.scanArticle(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find article by message ID: %w", err)
	}

	return article, nil
}

// RecordLike records that a user liked an article (no check for existing)
func (s *Storage) RecordLike(articleID int64) error {
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, articleID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to record like: %w", err)
	}
	return nil
}

// RecordLikeWithCheck records a like and returns true if it was new
func (s *Storage) RecordLikeWithCheck(articleID int64) (bool, error) {
	// Try to insert, if it fails due to duplicate key, article was already liked
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, articleID, time.Now())
	if err != nil {
		// Check if it's a unique constraint violation
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == 1555 { // SQLITE_CONSTRAINT_PRIMARYKEY
			return false, nil
		}
		return false, fmt.Errorf("failed to record like: %w", err)
	}
	return true, nil
}

// GetLikeCount returns the total number of likes
func (s *Storage) GetLikeCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM likes").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get like count: %w", err)
	}
	return count, nil
}

// UpsertTagWeight creates or updates a tag weight
func (s *Storage) UpsertTagWeight(tag string, weight float64, count int) error {
	query := `
		INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)
		ON CONFLICT(tag) DO UPDATE SET
			weight = excluded.weight,
			count = excluded.count
	`
	_, err := s.db.Exec(query, tag, weight, count)
	if err != nil {
		return fmt.Errorf("failed to upsert tag weight: %w", err)
	}
	return nil
}

// GetTagWeight retrieves weight and count for a specific tag
func (s *Storage) GetTagWeight(tag string) (float64, int, error) {
	var weight float64
	var count int

	query := `SELECT weight, count FROM tag_weights WHERE tag = ?`
	err := s.db.QueryRow(query, tag).Scan(&weight, &count)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get tag weight: %w", err)
	}

	return weight, count, nil
}

// GetAllTagWeights returns all tag weights
func (s *Storage) GetAllTagWeights() (map[string]float64, error) {
	query := `SELECT tag, weight FROM tag_weights`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag weights: %w", err)
	}
	defer rows.Close()

	weights := make(map[string]float64)
	for rows.Next() {
		var tag string
		var weight float64
		if err := rows.Scan(&tag, &weight); err != nil {
			return nil, fmt.Errorf("failed to scan tag weight: %w", err)
		}
		weights[tag] = weight
	}

	return weights, rows.Err()
}

// GetTopTags returns the top N tags by weight
func (s *Storage) GetTopTags(n int) ([]TagWeight, error) {
	query := `SELECT tag, weight, count FROM tag_weights ORDER BY weight DESC LIMIT ?`
	rows, err := s.db.Query(query, n)
	if err != nil {
		return nil, fmt.Errorf("failed to query top tags: %w", err)
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Name, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tw)
	}

	return tags, rows.Err()
}

// SetSetting stores a setting value
func (s *Storage) SetSetting(key, value string) error {
	query := `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := s.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}
	return nil
}

// GetSetting retrieves a setting value
func (s *Storage) GetSetting(key string) (string, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = ?`
	err := s.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting: %w", err)
	}
	return value, nil
}

// GetSentArticleCount returns the count of sent articles
func (s *Storage) GetSentArticleCount() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM articles WHERE sent_at IS NOT NULL`
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get sent article count: %w", err)
	}
	return count, nil
}

// scanArticle scans an article from a database row
func (s *Storage) scanArticle(row *sql.Row) (*Article, error) {
	var article Article
	var tagsJSON string
	var sentAt sql.NullTime
	var msgID sql.NullInt64

	err := row.Scan(
		&article.ID, &article.Title, &article.URL, &article.Summary,
		&tagsJSON, &article.HNScore, &article.FetchedAt,
		&sentAt, &msgID,
	)
	if err != nil {
		return nil, err
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	if sentAt.Valid {
		article.SentAt = &sentAt.Time
	}
	if msgID.Valid {
		article.TelegramMsgID = msgID.Int64
	}

	return &article, nil
}
