package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"modernc.org/sqlite"
)

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      []string
	HNScore   int
	FetchedAt time.Time
	SentAt    time.Time
	MessageID int
}

type TagWeight struct {
	Weight float64
	Count  int
}

type TopTag struct {
	Tag    string
	Weight float64
}

type Storage struct {
	db *sql.DB
}

func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		summary TEXT,
		tags TEXT,
		hn_score INTEGER,
		fetched_at DATETIME,
		sent_at DATETIME,
		message_id INTEGER
	);

	CREATE TABLE IF NOT EXISTS likes (
		article_id INTEGER PRIMARY KEY,
		liked_at DATETIME NOT NULL
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
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) SaveArticle(article Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
	INSERT INTO articles (id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		url = excluded.url,
		summary = excluded.summary,
		tags = excluded.tags,
		hn_score = excluded.hn_score,
		fetched_at = excluded.fetched_at,
		sent_at = excluded.sent_at,
		message_id = excluded.message_id
	`

	_, err = s.db.Exec(query, article.ID, article.Title, article.URL, article.Summary, tagsJSON, article.HNScore, article.FetchedAt, article.SentAt, article.MessageID)
	if err != nil {
		return fmt.Errorf("failed to save article: %w", err)
	}

	return nil
}

func (s *Storage) GetArticle(id int64) (Article, error) {
	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id FROM articles WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var article Article
	var tagsJSON string
	var fetchedAt, sentAt sql.NullTime

	err := row.Scan(&article.ID, &article.Title, &article.URL, &article.Summary, &tagsJSON, &article.HNScore, &fetchedAt, &sentAt, &article.MessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Article{}, fmt.Errorf("article not found")
		}
		return Article{}, fmt.Errorf("failed to get article: %w", err)
	}

	if fetchedAt.Valid {
		article.FetchedAt = fetchedAt.Time
	}
	if sentAt.Valid {
		article.SentAt = sentAt.Time
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
			return Article{}, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return article, nil
}

func (s *Storage) GetArticleByMessageID(messageID int) (Article, error) {
	query := `SELECT id, title, url, summary, tags, hn_score, fetched_at, sent_at, message_id FROM articles WHERE message_id = ?`
	row := s.db.QueryRow(query, messageID)

	var article Article
	var tagsJSON string
	var fetchedAt, sentAt sql.NullTime

	err := row.Scan(&article.ID, &article.Title, &article.URL, &article.Summary, &tagsJSON, &article.HNScore, &fetchedAt, &sentAt, &article.MessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Article{}, fmt.Errorf("article not found")
		}
		return Article{}, fmt.Errorf("failed to get article: %w", err)
	}

	if fetchedAt.Valid {
		article.FetchedAt = fetchedAt.Time
	}
	if sentAt.Valid {
		article.SentAt = sentAt.Time
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
			return Article{}, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return article, nil
}

func (s *Storage) GetRecentArticles(cutoff time.Time) ([]int64, error) {
	query := `SELECT id FROM articles WHERE sent_at >= ?`
	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent articles: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan article id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recent articles: %w", err)
	}

	return ids, nil
}

func (s *Storage) LikeArticle(articleID int64) error {
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, articleID, time.Now())
	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == 2067 {
			return nil
		}
		return fmt.Errorf("failed to like article: %w", err)
	}
	return nil
}

func (s *Storage) IsArticleLiked(articleID int64) (bool, error) {
	query := `SELECT 1 FROM likes WHERE article_id = ?`
	row := s.db.QueryRow(query, articleID)

	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if article is liked: %w", err)
	}

	return true, nil
}

func (s *Storage) GetLikeCount() (int, error) {
	query := `SELECT COUNT(*) FROM likes`
	row := s.db.QueryRow(query)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get like count: %w", err)
	}

	return count, nil
}

func (s *Storage) SetTagWeight(tag string, weight float64, count int) error {
	query := `INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)
	ON CONFLICT(tag) DO UPDATE SET weight = excluded.weight, count = excluded.count`
	_, err := s.db.Exec(query, tag, weight, count)
	if err != nil {
		return fmt.Errorf("failed to set tag weight: %w", err)
	}
	return nil
}

func (s *Storage) GetTagWeight(tag string) (float64, int, error) {
	query := `SELECT weight, count FROM tag_weights WHERE tag = ?`
	row := s.db.QueryRow(query, tag)

	var weight float64
	var count int

	err := row.Scan(&weight, &count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, fmt.Errorf("tag not found")
		}
		return 0, 0, fmt.Errorf("failed to get tag weight: %w", err)
	}

	return weight, count, nil
}

func (s *Storage) GetAllTagWeights() (map[string]TagWeight, error) {
	query := `SELECT tag, weight, count FROM tag_weights`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tag weights: %w", err)
	}
	defer rows.Close()

	tags := make(map[string]TagWeight)
	for rows.Next() {
		var tag string
		var weight float64
		var count int
		if err := rows.Scan(&tag, &weight, &count); err != nil {
			return nil, fmt.Errorf("failed to scan tag weight: %w", err)
		}
		tags[tag] = TagWeight{Weight: weight, Count: count}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag weights: %w", err)
	}

	return tags, nil
}

func (s *Storage) DecayTagWeights(decayRate, minWeight float64) error {
	query := `UPDATE tag_weights SET weight = MAX(weight * ?, ?)`
	_, err := s.db.Exec(query, 1-decayRate, minWeight)
	if err != nil {
		return fmt.Errorf("failed to decay tag weights: %w", err)
	}
	return nil
}

func (s *Storage) SetSetting(key, value string) error {
	query := `INSERT INTO settings (key, value) VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := s.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}
	return nil
}

func (s *Storage) GetSetting(key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	row := s.db.QueryRow(query, key)

	var value string
	err := row.Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("setting not found")
		}
		return "", fmt.Errorf("failed to get setting: %w", err)
	}

	return value, nil
}

func (s *Storage) GetTopTags(limit int) ([]TopTag, error) {
	query := `SELECT tag, weight FROM tag_weights ORDER BY weight DESC LIMIT ?`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top tags: %w", err)
	}
	defer rows.Close()

	var tags []TopTag
	for rows.Next() {
		var tag string
		var weight float64
		if err := rows.Scan(&tag, &weight); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, TopTag{Tag: tag, Weight: weight})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}
