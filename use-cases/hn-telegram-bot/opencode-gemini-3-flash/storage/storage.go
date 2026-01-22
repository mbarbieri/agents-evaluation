package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Article struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	URL           string     `json:"url"`
	Summary       string     `json:"summary"`
	Tags          []string   `json:"tags"`
	Score         int        `json:"score"`
	FetchedAt     time.Time  `json:"fetched_at"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	TelegramMsgID int        `json:"telegram_msg_id,omitempty"`
}

type TagWeight struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Count  int     `json:"count"`
}

type Storage struct {
	db *sql.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT NOT NULL,
			tags TEXT NOT NULL,
			score INTEGER NOT NULL,
			fetched_at DATETIME NOT NULL,
			sent_at DATETIME,
			telegram_msg_id INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_msg_id ON articles(telegram_msg_id)`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY REFERENCES articles(id),
			liked_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			name TEXT PRIMARY KEY,
			weight REAL NOT NULL DEFAULT 1.0,
			occurrence_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("failed to execute schema query (%s): %w", q, err)
		}
	}
	return nil
}

func (s *Storage) SaveArticle(ctx context.Context, a *Article) error {
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return err
	}

	query := `INSERT INTO articles (id, title, url, summary, tags, score, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			url = excluded.url,
			summary = excluded.summary,
			tags = excluded.tags,
			score = excluded.score`

	_, err = s.db.ExecContext(ctx, query, a.ID, a.Title, a.URL, a.Summary, string(tagsJSON), a.Score, a.FetchedAt)
	return err
}

func (s *Storage) GetArticle(ctx context.Context, id int64) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, telegram_msg_id FROM articles WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var a Article
	var tagsStr string
	var sentAt sql.NullTime
	var msgID sql.NullInt64

	err := row.Scan(&a.ID, &a.Title, &a.URL, &a.Summary, &tagsStr, &a.Score, &a.FetchedAt, &sentAt, &msgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsStr), &a.Tags); err != nil {
		return nil, err
	}
	if sentAt.Valid {
		a.SentAt = &sentAt.Time
	}
	if msgID.Valid {
		a.TelegramMsgID = int(msgID.Int64)
	}
	return &a, nil
}

func (s *Storage) MarkArticleSent(ctx context.Context, id int64, msgID int) error {
	query := `UPDATE articles SET sent_at = ?, telegram_msg_id = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, time.Now(), msgID, id)
	return err
}

func (s *Storage) GetArticleByMessageID(ctx context.Context, msgID int) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, telegram_msg_id FROM articles WHERE telegram_msg_id = ?`
	row := s.db.QueryRowContext(ctx, query, msgID)

	var a Article
	var tagsStr string
	var sentAt sql.NullTime
	var mID sql.NullInt64

	err := row.Scan(&a.ID, &a.Title, &a.URL, &a.Summary, &tagsStr, &a.Score, &a.FetchedAt, &sentAt, &mID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsStr), &a.Tags); err != nil {
		return nil, err
	}
	if sentAt.Valid {
		a.SentAt = &sentAt.Time
	}
	if mID.Valid {
		a.TelegramMsgID = int(mID.Int64)
	}
	return &a, nil
}

func (s *Storage) IsArticleLiked(ctx context.Context, id int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM likes WHERE article_id = ?)`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&exists)
	return exists, err
}

func (s *Storage) LikeArticle(ctx context.Context, id int64) error {
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?) ON CONFLICT(article_id) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, id, time.Now())
	return err
}

func (s *Storage) GetTotalLikes(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM likes`
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (s *Storage) UpdateTagWeight(ctx context.Context, name string, weight float64, countIncr int) error {
	query := `INSERT INTO tag_weights (name, weight, occurrence_count)
		VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			weight = excluded.weight,
			occurrence_count = occurrence_count + excluded.occurrence_count`
	_, err := s.db.ExecContext(ctx, query, name, weight, countIncr)
	return err
}

func (s *Storage) GetAllTagWeights(ctx context.Context) (map[string]float64, error) {
	query := `SELECT name, weight FROM tag_weights`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weights := make(map[string]float64)
	for rows.Next() {
		var name string
		var weight float64
		if err := rows.Scan(&name, &weight); err != nil {
			return nil, err
		}
		weights[name] = weight
	}
	return weights, nil
}

func (s *Storage) GetTopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	query := `SELECT name, weight, occurrence_count FROM tag_weights ORDER BY weight DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Name, &tw.Weight, &tw.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tw)
	}
	return tags, nil
}

func (s *Storage) SetSetting(ctx context.Context, key, value string) error {
	query := `INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := s.db.ExecContext(ctx, query, key, value)
	return err
}

func (s *Storage) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = ?`
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Storage) GetRecentArticleIDs(ctx context.Context, days int) ([]int64, error) {
	query := `SELECT id FROM articles WHERE sent_at >= ?`
	rows, err := s.db.QueryContext(ctx, query, time.Now().AddDate(0, 0, -days))
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
	return ids, nil
}
