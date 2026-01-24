package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Article struct {
	HNID              int
	Title             string
	URL               string
	Summary           string
	Tags              []string
	HNScore           int
	FetchedAt         time.Time
	SentAt            time.Time
	TelegramMessageID int
}

type TagWeight struct {
	Tag         string
	Weight      float64
	Occurrences int
}

type Storage struct {
	db *sql.DB
}

func NewStorage(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
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
			hn_id INTEGER PRIMARY KEY,
			title TEXT,
			url TEXT,
			summary TEXT,
			tags TEXT,
			hn_score INTEGER,
			fetched_at DATETIME,
			sent_at DATETIME,
			telegram_message_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at DATETIME,
			FOREIGN KEY(article_id) REFERENCES articles(hn_id)
		)`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL DEFAULT 1.0,
			occurrences INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}
	return nil
}

func (s *Storage) SaveArticle(a *Article) error {
	tagsJSON, _ := json.Marshal(a.Tags)
	query := `INSERT INTO articles (hn_id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_message_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hn_id) DO UPDATE SET
			title=excluded.title,
			url=excluded.url,
			summary=excluded.summary,
			tags=excluded.tags,
			hn_score=excluded.hn_score,
			sent_at=excluded.sent_at,
			telegram_message_id=excluded.telegram_message_id`

	_, err := s.db.Exec(query, a.HNID, a.Title, a.URL, a.Summary, string(tagsJSON), a.HNScore, a.FetchedAt, a.SentAt, a.TelegramMessageID)
	return err
}

func (s *Storage) GetArticle(hnID int) (*Article, error) {
	var a Article
	var tagsJSON string
	var sentAt, fetchedAt sql.NullTime

	query := `SELECT hn_id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_message_id FROM articles WHERE hn_id = ?`
	err := s.db.QueryRow(query, hnID).Scan(&a.HNID, &a.Title, &a.URL, &a.Summary, &tagsJSON, &a.HNScore, &fetchedAt, &sentAt, &a.TelegramMessageID)
	if err != nil {
		return nil, err
	}

	if fetchedAt.Valid {
		a.FetchedAt = fetchedAt.Time
	}
	if sentAt.Valid {
		a.SentAt = sentAt.Time
	}

	json.Unmarshal([]byte(tagsJSON), &a.Tags)
	return &a, nil
}

func (s *Storage) GetArticleByMessageID(msgID int) (*Article, error) {
	var a Article
	var tagsJSON string
	var sentAt, fetchedAt sql.NullTime

	query := `SELECT hn_id, title, url, summary, tags, hn_score, fetched_at, sent_at, telegram_message_id FROM articles WHERE telegram_message_id = ?`
	err := s.db.QueryRow(query, msgID).Scan(&a.HNID, &a.Title, &a.URL, &a.Summary, &tagsJSON, &a.HNScore, &fetchedAt, &sentAt, &a.TelegramMessageID)
	if err != nil {
		return nil, err
	}

	if fetchedAt.Valid {
		a.FetchedAt = fetchedAt.Time
	}
	if sentAt.Valid {
		a.SentAt = sentAt.Time
	}

	json.Unmarshal([]byte(tagsJSON), &a.Tags)
	return &a, nil
}

func (s *Storage) MarkLiked(articleID int) error {
	query := `INSERT INTO likes (article_id, liked_at) VALUES (?, ?) ON CONFLICT(article_id) DO NOTHING`
	_, err := s.db.Exec(query, articleID, time.Now())
	return err
}

func (s *Storage) IsLiked(articleID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM likes WHERE article_id = ?)`
	err := s.db.QueryRow(query, articleID).Scan(&exists)
	return exists, err
}

func (s *Storage) UpdateTagWeight(tag string, weight float64, occurrences int) error {
	query := `INSERT INTO tag_weights (tag, weight, occurrences) VALUES (?, ?, ?)
		ON CONFLICT(tag) DO UPDATE SET weight=excluded.weight, occurrences=excluded.occurrences`
	_, err := s.db.Exec(query, tag, weight, occurrences)
	return err
}

func (s *Storage) GetTagWeights() (map[string]TagWeight, error) {
	rows, err := s.db.Query(`SELECT tag, weight, occurrences FROM tag_weights`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weights := make(map[string]TagWeight)
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Occurrences); err != nil {
			return nil, err
		}
		weights[tw.Tag] = tw
	}
	return weights, nil
}

func (s *Storage) SetSetting(key, value string) error {
	query := `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`
	_, err := s.db.Exec(query, key, value)
	return err
}

func (s *Storage) GetSetting(key string) (string, error) {
	var val string
	query := `SELECT value FROM settings WHERE key = ?`
	err := s.db.QueryRow(query, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *Storage) GetRecentHNIDs(days int) ([]int, error) {
	rows, err := s.db.Query(`SELECT hn_id FROM articles WHERE sent_at > ?`, time.Now().AddDate(0, 0, -days))
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
