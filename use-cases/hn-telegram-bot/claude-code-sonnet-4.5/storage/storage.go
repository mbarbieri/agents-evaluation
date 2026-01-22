package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Article struct {
	ID            int
	Title         string
	URL           string
	Summary       string
	Tags          []string
	HNScore       int
	FetchedAt     time.Time
	SentAt        *time.Time
	TelegramMsgID int
}

type TagWeight struct {
	Tag    string
	Weight float64
}

type Storage interface {
	SaveArticle(article *Article) error
	GetArticleByID(id int) (*Article, error)
	GetArticleByTelegramMsgID(msgID int) (*Article, error)
	GetRecentlySentArticles(days int) ([]int, error)

	IsLiked(articleID int) (bool, error)
	RecordLike(articleID int) error
	GetLikeCount() (int, error)

	GetTagWeight(tag string) (float64, error)
	SetTagWeight(tag string, weight float64) error
	IncrementTagOccurrence(tag string) error
	GetTopTags(limit int) ([]TagWeight, error)
	GetAllTagWeights() ([]TagWeight, error)
	ApplyDecay(decayRate float64, minWeight float64) error

	GetSetting(key string) (string, error)
	SetSetting(key string, value string) error

	Close() error
}

type SQLiteStorage struct {
	db *sql.DB
}

func New(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &SQLiteStorage{db: db}
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		summary TEXT NOT NULL,
		tags TEXT NOT NULL,
		hn_score INTEGER NOT NULL,
		fetched_at INTEGER NOT NULL,
		sent_at INTEGER,
		telegram_msg_id INTEGER
	);

	CREATE TABLE IF NOT EXISTS likes (
		article_id INTEGER PRIMARY KEY,
		liked_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tag_weights (
		tag TEXT PRIMARY KEY,
		weight REAL NOT NULL,
		occurrence_count INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at);
	CREATE INDEX IF NOT EXISTS idx_articles_telegram_msg_id ON articles(telegram_msg_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStorage) SaveArticle(article *Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return err
	}

	var sentAt *int64
	if article.SentAt != nil {
		ts := article.SentAt.Unix()
		sentAt = &ts
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

	_, err = s.db.Exec(query, article.ID, article.Title, article.URL, article.Summary,
		string(tagsJSON), article.HNScore, article.FetchedAt.Unix(), sentAt, article.TelegramMsgID)
	return err
}

func (s *SQLiteStorage) GetArticleByID(id int) (*Article, error) {
	query := `
	SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id
	FROM articles WHERE id = ?
	`

	var article Article
	var tagsJSON string
	var fetchedAt int64
	var sentAt *int64

	err := s.db.QueryRow(query, id).Scan(
		&article.ID, &article.Title, &article.URL, &article.Summary,
		&tagsJSON, &article.HNScore, &fetchedAt, &sentAt, &article.TelegramMsgID,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return nil, err
	}

	article.FetchedAt = time.Unix(fetchedAt, 0)
	if sentAt != nil {
		t := time.Unix(*sentAt, 0)
		article.SentAt = &t
	}

	return &article, nil
}

func (s *SQLiteStorage) GetArticleByTelegramMsgID(msgID int) (*Article, error) {
	query := `
	SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_msg_id
	FROM articles WHERE telegram_msg_id = ?
	`

	var article Article
	var tagsJSON string
	var fetchedAt int64
	var sentAt *int64

	err := s.db.QueryRow(query, msgID).Scan(
		&article.ID, &article.Title, &article.URL, &article.Summary,
		&tagsJSON, &article.HNScore, &fetchedAt, &sentAt, &article.TelegramMsgID,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return nil, err
	}

	article.FetchedAt = time.Unix(fetchedAt, 0)
	if sentAt != nil {
		t := time.Unix(*sentAt, 0)
		article.SentAt = &t
	}

	return &article, nil
}

func (s *SQLiteStorage) GetRecentlySentArticles(days int) ([]int, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	query := `SELECT id FROM articles WHERE sent_at IS NOT NULL AND sent_at > ?`

	rows, err := s.db.Query(query, cutoff)
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

	return ids, rows.Err()
}

func (s *SQLiteStorage) IsLiked(articleID int) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM likes WHERE article_id = ?", articleID).Scan(&count)
	return count > 0, err
}

func (s *SQLiteStorage) RecordLike(articleID int) error {
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?) ON CONFLICT(article_id) DO NOTHING`
	_, err := s.db.Exec(query, articleID, time.Now().Unix())
	return err
}

func (s *SQLiteStorage) GetLikeCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM likes").Scan(&count)
	return count, err
}

func (s *SQLiteStorage) GetTagWeight(tag string) (float64, error) {
	var weight float64
	err := s.db.QueryRow("SELECT weight FROM tag_weights WHERE tag = ?", tag).Scan(&weight)
	if err == sql.ErrNoRows {
		return 1.0, nil
	}
	return weight, err
}

func (s *SQLiteStorage) SetTagWeight(tag string, weight float64) error {
	query := `
	INSERT INTO tag_weights (tag, weight, occurrence_count)
	VALUES (?, ?, 0)
	ON CONFLICT(tag) DO UPDATE SET weight = excluded.weight
	`
	_, err := s.db.Exec(query, tag, weight)
	return err
}

func (s *SQLiteStorage) IncrementTagOccurrence(tag string) error {
	query := `
	INSERT INTO tag_weights (tag, weight, occurrence_count)
	VALUES (?, 1.0, 1)
	ON CONFLICT(tag) DO UPDATE SET occurrence_count = occurrence_count + 1
	`
	_, err := s.db.Exec(query, tag)
	return err
}

func (s *SQLiteStorage) GetTopTags(limit int) ([]TagWeight, error) {
	query := `SELECT tag, weight FROM tag_weights ORDER BY weight DESC LIMIT ?`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight); err != nil {
			return nil, err
		}
		tags = append(tags, tw)
	}

	return tags, rows.Err()
}

func (s *SQLiteStorage) GetAllTagWeights() ([]TagWeight, error) {
	query := `SELECT tag, weight FROM tag_weights`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight); err != nil {
			return nil, err
		}
		tags = append(tags, tw)
	}

	return tags, rows.Err()
}

func (s *SQLiteStorage) ApplyDecay(decayRate float64, minWeight float64) error {
	query := `UPDATE tag_weights SET weight = MAX(weight * ?, ?)`
	_, err := s.db.Exec(query, 1.0-decayRate, minWeight)
	return err
}

func (s *SQLiteStorage) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStorage) SetSetting(key string, value string) error {
	query := `
	INSERT INTO settings (key, value)
	VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
	_, err := s.db.Exec(query, key, value)
	return err
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
