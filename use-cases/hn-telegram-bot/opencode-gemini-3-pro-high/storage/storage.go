package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Article represents a Hacker News story
type Article struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Summary   string    `json:"summary"`
	Tags      []string  `json:"tags"`
	Score     int       `json:"score"`
	FetchedAt time.Time `json:"fetched_at"`
	SentAt    time.Time `json:"sent_at"`
	MsgID     int       `json:"msg_id"`
}

// DB handles database operations
type DB struct {
	sqlDB *sql.DB
}

// New initializes the database connection and schema
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &DB{sqlDB: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (d *DB) Close() error {
	return d.sqlDB.Close()
}

func (d *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT,
			url TEXT,
			summary TEXT,
			tags TEXT,
			score INTEGER,
			fetched_at DATETIME,
			sent_at DATETIME,
			telegram_msg_id INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL DEFAULT 1.0,
			count INTEGER DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}

	for _, query := range queries {
		if _, err := d.sqlDB.Exec(query); err != nil {
			return fmt.Errorf("failed to init schema: %w", err)
		}
	}
	return nil
}

// Settings

func (d *DB) SetSetting(key, value string) error {
	query := `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := d.sqlDB.Exec(query, key, value)
	return err
}

func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.sqlDB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Articles

func (d *DB) SaveArticle(a Article) error {
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return err
	}
	query := `INSERT INTO articles (id, title, url, summary, tags, score, fetched_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)
			  ON CONFLICT(id) DO UPDATE SET 
			  title=excluded.title, url=excluded.url, summary=excluded.summary, 
			  tags=excluded.tags, score=excluded.score`
	_, err = d.sqlDB.Exec(query, a.ID, a.Title, a.URL, a.Summary, string(tagsJSON), a.Score, a.FetchedAt)
	return err
}

func (d *DB) MarkArticleSent(id, msgID int) error {
	query := `UPDATE articles SET sent_at = ?, telegram_msg_id = ? WHERE id = ?`
	_, err := d.sqlDB.Exec(query, time.Now(), msgID, id)
	return err
}

func (d *DB) GetRecentSentArticleIDs(duration time.Duration) ([]int, error) {
	cutoff := time.Now().Add(-duration)
	rows, err := d.sqlDB.Query(`SELECT id FROM articles WHERE sent_at > ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (d *DB) GetArticleByMsgID(msgID int) (*Article, error) {
	var a Article
	var tagsJSON string
	var sentAt sql.NullTime

	// We need all fields to reconstruct full object if needed, or at least tags for boosting
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, telegram_msg_id 
			  FROM articles WHERE telegram_msg_id = ?`

	row := d.sqlDB.QueryRow(query, msgID)
	err := row.Scan(&a.ID, &a.Title, &a.URL, &a.Summary, &tagsJSON, &a.Score, &a.FetchedAt, &sentAt, &a.MsgID)
	if err != nil {
		return nil, err
	}

	if sentAt.Valid {
		a.SentAt = sentAt.Time
	}
	if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
		return nil, err
	}
	return &a, nil
}

// Likes & Tags

func (d *DB) IsArticleLiked(articleID int) (bool, error) {
	var count int
	err := d.sqlDB.QueryRow(`SELECT count(*) FROM likes WHERE article_id = ?`, articleID).Scan(&count)
	return count > 0, err
}

func (d *DB) AddLike(articleID int) error {
	_, err := d.sqlDB.Exec(`INSERT INTO likes (article_id, liked_at) VALUES (?, ?)`, articleID, time.Now())
	return err
}

func (d *DB) BoostTag(tag string, initialWeight, boostAmount float64) error {
	// If tag doesn't exist, insert with initialWeight + boostAmount (because this is the first like)
	// If tag exists, add boostAmount to current weight

	// Check existence first to handle default logic cleanly or use upsert with math
	// SQLite upsert:
	// INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, 1)
	// ON CONFLICT(tag) DO UPDATE SET weight = weight + ?, count = count + 1

	// Wait, spec says:
	// "If tag doesn't exist in tag_weights, insert with weight = 1.0 + boost_amount"
	// "If tag exists, add boost_amount to current weight"

	startWeight := 1.0 // Default per spec
	if initialWeight != 0 {
		startWeight = initialWeight
	}

	query := `INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, 1)
			  ON CONFLICT(tag) DO UPDATE SET weight = weight + ?, count = count + 1`

	// Param 1: tag
	// Param 2: startWeight + boostAmount (new insert value)
	// Param 3: boostAmount (update adder)

	_, err := d.sqlDB.Exec(query, tag, startWeight+boostAmount, boostAmount)
	return err
}

func (d *DB) GetTagWeights() (map[string]float64, error) {
	rows, err := d.sqlDB.Query(`SELECT tag, weight FROM tag_weights`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weights := make(map[string]float64)
	for rows.Next() {
		var tag string
		var w float64
		if err := rows.Scan(&tag, &w); err != nil {
			return nil, err
		}
		weights[tag] = w
	}
	return weights, nil
}

func (d *DB) ApplyTagDecay(decayRate, minWeight float64) error {
	// formula: new_weight = max(current_weight * (1 - decay_rate), min_weight)
	// SQLite supports MAX()

	query := `UPDATE tag_weights SET weight = MAX(weight * (1 - ?), ?)`
	_, err := d.sqlDB.Exec(query, decayRate, minWeight)
	return err
}

func (d *DB) GetTotalLikes() (int, error) {
	var count int
	err := d.sqlDB.QueryRow(`SELECT count(*) FROM likes`).Scan(&count)
	return count, err
}
