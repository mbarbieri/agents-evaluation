package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Article represents a stored HN article.
type Article struct {
	ID            int    // HN item ID (primary key)
	Title         string
	URL           string
	Summary       string
	Tags          string // JSON array stored as text
	Score         int    // HN score at fetch time
	FetchedAt     int64  // Unix timestamp
	SentAt        int64  // Unix timestamp, 0 if not sent
	TelegramMsgID int    // Telegram message ID for reaction tracking
}

// TagWeight represents learned preference for a tag.
type TagWeight struct {
	Tag    string
	Weight float64
	Count  int
}

// Store provides SQLite-backed persistence for articles, likes, tag weights, and settings.
type Store struct {
	db *sql.DB
}

const createTablesSQL = `
CREATE TABLE IF NOT EXISTS articles (
	id INTEGER PRIMARY KEY,
	title TEXT,
	url TEXT,
	summary TEXT,
	tags TEXT,
	score INTEGER,
	fetched_at INTEGER,
	sent_at INTEGER,
	telegram_msg_id INTEGER
);

CREATE TABLE IF NOT EXISTS likes (
	article_id INTEGER PRIMARY KEY,
	liked_at INTEGER
);

CREATE TABLE IF NOT EXISTS tag_weights (
	tag TEXT PRIMARY KEY,
	weight REAL DEFAULT 1.0,
	count INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT
);
`

// New opens the SQLite database at dbPath, creates tables if they don't exist, and returns a Store.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: set WAL mode: %w", err)
	}

	if _, err := db.Exec(createTablesSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: create tables: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// SaveArticle inserts or replaces an article in the database.
func (s *Store) SaveArticle(a *Article) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO articles (id, title, url, summary, tags, score, fetched_at, sent_at, telegram_msg_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Title, a.URL, a.Summary, a.Tags, a.Score, a.FetchedAt, a.SentAt, a.TelegramMsgID,
	)
	if err != nil {
		return fmt.Errorf("storage: save article %d: %w", a.ID, err)
	}
	return nil
}

// GetArticleBySentMsgID looks up an article by its Telegram message ID.
// Returns nil if no article is found.
func (s *Store) GetArticleBySentMsgID(msgID int) (*Article, error) {
	var a Article
	err := s.db.QueryRow(
		`SELECT id, title, url, summary, tags, score, fetched_at, sent_at, telegram_msg_id
		 FROM articles WHERE telegram_msg_id = ?`, msgID,
	).Scan(&a.ID, &a.Title, &a.URL, &a.Summary, &a.Tags, &a.Score, &a.FetchedAt, &a.SentAt, &a.TelegramMsgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get article by sent msg id %d: %w", msgID, err)
	}
	return &a, nil
}

// GetRecentSentArticleIDs returns the IDs of articles sent within the last N days.
func (s *Store) GetRecentSentArticleIDs(days int) ([]int, error) {
	cutoff := time.Now().Unix() - int64(days)*86400
	rows, err := s.db.Query(
		`SELECT id FROM articles WHERE sent_at > ?`, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: get recent sent article ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("storage: scan article id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate article ids: %w", err)
	}
	return ids, nil
}

// MarkSent updates an article's sent_at timestamp and telegram_msg_id.
func (s *Store) MarkSent(articleID int, telegramMsgID int) error {
	_, err := s.db.Exec(
		`UPDATE articles SET sent_at = ?, telegram_msg_id = ? WHERE id = ?`,
		time.Now().Unix(), telegramMsgID, articleID,
	)
	if err != nil {
		return fmt.Errorf("storage: mark sent article %d: %w", articleID, err)
	}
	return nil
}

// IsLiked checks whether an article has a like record.
func (s *Store) IsLiked(articleID int) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM likes WHERE article_id = ?`, articleID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("storage: is liked %d: %w", articleID, err)
	}
	return count > 0, nil
}

// RecordLike inserts a like record for the article with the current timestamp.
// Uses INSERT OR IGNORE so repeated calls for the same article are idempotent.
func (s *Store) RecordLike(articleID int) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO likes (article_id, liked_at) VALUES (?, ?)`,
		articleID, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("storage: record like for article %d: %w", articleID, err)
	}
	return nil
}

// GetTagWeights returns all tag weights from the database.
func (s *Store) GetTagWeights() ([]TagWeight, error) {
	rows, err := s.db.Query(`SELECT tag, weight, count FROM tag_weights`)
	if err != nil {
		return nil, fmt.Errorf("storage: get tag weights: %w", err)
	}
	defer rows.Close()

	var weights []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("storage: scan tag weight: %w", err)
		}
		weights = append(weights, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate tag weights: %w", err)
	}
	return weights, nil
}

// GetTopTagWeights returns the top N tag weights ordered by weight descending.
func (s *Store) GetTopTagWeights(limit int) ([]TagWeight, error) {
	rows, err := s.db.Query(
		`SELECT tag, weight, count FROM tag_weights ORDER BY weight DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: get top tag weights: %w", err)
	}
	defer rows.Close()

	var weights []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("storage: scan top tag weight: %w", err)
		}
		weights = append(weights, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate top tag weights: %w", err)
	}
	return weights, nil
}

// UpsertTagWeight inserts or replaces a tag weight record.
func (s *Store) UpsertTagWeight(tag string, weight float64, count int) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)`,
		tag, weight, count,
	)
	if err != nil {
		return fmt.Errorf("storage: upsert tag weight %q: %w", tag, err)
	}
	return nil
}

// ApplyDecay reduces all tag weights by the given decay rate, enforcing a minimum weight.
// Formula: weight = MAX(weight * (1 - decayRate), minWeight)
func (s *Store) ApplyDecay(decayRate, minWeight float64) error {
	_, err := s.db.Exec(
		`UPDATE tag_weights SET weight = MAX(weight * (1 - ?), ?)`,
		decayRate, minWeight,
	)
	if err != nil {
		return fmt.Errorf("storage: apply decay: %w", err)
	}
	return nil
}

// GetLikeCount returns the total number of liked articles.
func (s *Store) GetLikeCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM likes`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("storage: get like count: %w", err)
	}
	return count, nil
}

// GetSetting returns the value for the given settings key.
// Returns an empty string if the key is not found.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage: get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting inserts or replaces a setting key-value pair.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("storage: set setting %q: %w", key, err)
	}
	return nil
}

// GetTagWeight returns the tag weight for a specific tag.
// Returns nil if the tag is not found.
func (s *Store) GetTagWeight(tag string) (*TagWeight, error) {
	var tw TagWeight
	err := s.db.QueryRow(
		`SELECT tag, weight, count FROM tag_weights WHERE tag = ?`, tag,
	).Scan(&tw.Tag, &tw.Weight, &tw.Count)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get tag weight %q: %w", tag, err)
	}
	return &tw, nil
}
