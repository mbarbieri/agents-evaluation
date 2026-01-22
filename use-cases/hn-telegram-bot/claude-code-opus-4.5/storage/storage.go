package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

// Article represents a Hacker News article with metadata.
type Article struct {
	ID            int64
	Title         string
	URL           string
	Summary       string
	Tags          []string
	HNScore       int
	FetchedAt     time.Time
	SentAt        *time.Time
	TelegramMsgID *int64
}

// TagWeight represents a tag's learned preference weight.
type TagWeight struct {
	Tag    string
	Weight float64
	Count  int
}

// DB wraps the SQLite database connection and provides storage operations.
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection and initializes the schema.
func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		summary TEXT,
		tags TEXT NOT NULL DEFAULT '[]',
		hn_score INTEGER DEFAULT 0,
		fetched_at DATETIME NOT NULL,
		sent_at DATETIME,
		telegram_msg_id INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at);
	CREATE INDEX IF NOT EXISTS idx_articles_telegram_msg_id ON articles(telegram_msg_id);

	CREATE TABLE IF NOT EXISTS likes (
		article_id INTEGER PRIMARY KEY REFERENCES articles(id),
		liked_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tag_weights (
		tag TEXT PRIMARY KEY,
		weight REAL NOT NULL DEFAULT 1.0,
		count INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// SaveArticle inserts or updates an article.
func (db *DB) SaveArticle(ctx context.Context, article *Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
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
		fetched_at = excluded.fetched_at,
		sent_at = excluded.sent_at,
		telegram_msg_id = excluded.telegram_msg_id
	`

	_, err = db.conn.ExecContext(ctx, query,
		article.ID,
		article.Title,
		article.URL,
		article.Summary,
		string(tagsJSON),
		article.HNScore,
		article.FetchedAt,
		article.SentAt,
		article.TelegramMsgID,
	)
	return err
}

// GetArticle retrieves an article by HN ID.
func (db *DB) GetArticle(ctx context.Context, id int64) (*Article, error) {
	query := `
	SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id
	FROM articles WHERE id = ?
	`

	article := &Article{}
	var tagsJSON string
	var sentAt sql.NullTime
	var telegramMsgID sql.NullInt64

	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&article.ID,
		&article.Title,
		&article.URL,
		&article.Summary,
		&tagsJSON,
		&article.HNScore,
		&article.FetchedAt,
		&sentAt,
		&telegramMsgID,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if article.Tags == nil {
		article.Tags = []string{}
	}

	if sentAt.Valid {
		article.SentAt = &sentAt.Time
	}
	if telegramMsgID.Valid {
		article.TelegramMsgID = &telegramMsgID.Int64
	}

	return article, nil
}

// GetArticleByMessageID retrieves an article by its Telegram message ID.
func (db *DB) GetArticleByMessageID(ctx context.Context, msgID int64) (*Article, error) {
	query := `
	SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id
	FROM articles WHERE telegram_msg_id = ?
	`

	article := &Article{}
	var tagsJSON string
	var sentAt sql.NullTime
	var telegramMsgID sql.NullInt64

	err := db.conn.QueryRowContext(ctx, query, msgID).Scan(
		&article.ID,
		&article.Title,
		&article.URL,
		&article.Summary,
		&tagsJSON,
		&article.HNScore,
		&article.FetchedAt,
		&sentAt,
		&telegramMsgID,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if article.Tags == nil {
		article.Tags = []string{}
	}

	if sentAt.Valid {
		article.SentAt = &sentAt.Time
	}
	if telegramMsgID.Valid {
		article.TelegramMsgID = &telegramMsgID.Int64
	}

	return article, nil
}

// GetRecentlySentArticleIDs returns IDs of articles sent within the given duration.
func (db *DB) GetRecentlySentArticleIDs(ctx context.Context, within time.Duration) ([]int64, error) {
	cutoff := time.Now().Add(-within)
	query := `SELECT id FROM articles WHERE sent_at > ?`

	rows, err := db.conn.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MarkArticleSent updates an article with sent timestamp and message ID.
func (db *DB) MarkArticleSent(ctx context.Context, articleID int64, telegramMsgID int64) error {
	query := `UPDATE articles SET sent_at = ?, telegram_msg_id = ? WHERE id = ?`
	_, err := db.conn.ExecContext(ctx, query, time.Now(), telegramMsgID, articleID)
	return err
}

// IsArticleLiked checks if an article has been liked.
func (db *DB) IsArticleLiked(ctx context.Context, articleID int64) (bool, error) {
	query := `SELECT 1 FROM likes WHERE article_id = ?`
	var dummy int
	err := db.conn.QueryRowContext(ctx, query, articleID).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// LikeArticle records a like for an article (idempotent).
func (db *DB) LikeArticle(ctx context.Context, articleID int64) error {
	query := `INSERT OR IGNORE INTO likes (article_id, liked_at) VALUES (?, ?)`
	_, err := db.conn.ExecContext(ctx, query, articleID, time.Now())
	return err
}

// GetLikeCount returns the total number of liked articles.
func (db *DB) GetLikeCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM likes`
	var count int
	err := db.conn.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// GetTagWeight returns the weight for a tag, or 1.0 if not found.
func (db *DB) GetTagWeight(ctx context.Context, tag string) (float64, error) {
	query := `SELECT weight FROM tag_weights WHERE tag = ?`
	var weight float64
	err := db.conn.QueryRowContext(ctx, query, tag).Scan(&weight)
	if err == sql.ErrNoRows {
		return 1.0, nil
	}
	return weight, err
}

// GetAllTagWeights returns all tag weights as a map.
func (db *DB) GetAllTagWeights(ctx context.Context) (map[string]float64, error) {
	query := `SELECT tag, weight FROM tag_weights`
	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weights := make(map[string]float64)
	for rows.Next() {
		var tag string
		var weight float64
		if err := rows.Scan(&tag, &weight); err != nil {
			return nil, err
		}
		weights[tag] = weight
	}
	return weights, rows.Err()
}

// BoostTagWeight increases a tag's weight by the given amount.
func (db *DB) BoostTagWeight(ctx context.Context, tag string, boost float64) error {
	query := `
	INSERT INTO tag_weights (tag, weight, count)
	VALUES (?, 1.0 + ?, 1)
	ON CONFLICT(tag) DO UPDATE SET
		weight = weight + ?,
		count = count + 1
	`
	_, err := db.conn.ExecContext(ctx, query, tag, boost, boost)
	return err
}

// ApplyTagDecay reduces all tag weights by decay rate with a minimum floor.
func (db *DB) ApplyTagDecay(ctx context.Context, decayRate, minWeight float64) error {
	query := `
	UPDATE tag_weights SET weight = MAX(weight * (1.0 - ?), ?)
	`
	_, err := db.conn.ExecContext(ctx, query, decayRate, minWeight)
	return err
}

// GetTopTags returns the top N tags by weight.
func (db *DB) GetTopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	query := `SELECT tag, weight, count FROM tag_weights ORDER BY weight DESC LIMIT ?`
	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tw)
	}
	return tags, rows.Err()
}

// GetSetting retrieves a setting value by key.
func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	var value string
	err := db.conn.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return value, err
}

// SetSetting stores or updates a setting.
func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	query := `
	INSERT INTO settings (key, value) VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
	_, err := db.conn.ExecContext(ctx, query, key, value)
	return err
}
