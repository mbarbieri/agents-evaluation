package storage

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	_ "modernc.org/sqlite"
)

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      string
	Score     int
	FetchedAt time.Time
	SentAt    sql.NullTime
	MessageID sql.NullInt64
}

type Like struct {
	ArticleID int64
	LikedAt   time.Time
}

type TagWeight struct {
	Name   string
	Weight float64
	Count  int
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
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
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
			fetched_at INTEGER NOT NULL,
			sent_at INTEGER,
			message_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag_name TEXT PRIMARY KEY,
			weight REAL NOT NULL DEFAULT 1.0,
			count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_message_id ON articles(message_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

func (s *Storage) SaveArticle(article *Article) error {
	query := `INSERT OR REPLACE INTO articles (id, title, url, summary, tags, score, fetched_at, sent_at, message_id)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var sentAt int64
	if article.SentAt.Valid {
		sentAt = article.SentAt.Time.Unix()
	}

	var messageID int64
	if article.MessageID.Valid {
		messageID = article.MessageID.Int64
	}

	_, err := s.db.Exec(query,
		article.ID,
		article.Title,
		article.URL,
		article.Summary,
		article.Tags,
		article.Score,
		article.FetchedAt.Unix(),
		sentAt,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("failed to save article: %w", err)
	}
	return nil
}

func (s *Storage) GetArticle(id int64) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, message_id
			  FROM articles WHERE id = ?`

	var article Article
	var sentAt int64
	var fetchedAt int64
	var messageID int64

	err := s.db.QueryRow(query, id).Scan(
		&article.ID,
		&article.Title,
		&article.URL,
		&article.Summary,
		&article.Tags,
		&article.Score,
		&fetchedAt,
		&sentAt,
		&messageID,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("article not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article: %w", err)
	}

	article.FetchedAt = time.Unix(fetchedAt, 0)
	if sentAt != 0 {
		article.SentAt = sql.NullTime{Time: time.Unix(sentAt, 0), Valid: true}
	}
	if messageID != 0 {
		article.MessageID = sql.NullInt64{Int64: messageID, Valid: true}
	}

	return &article, nil
}

func (s *Storage) GetArticleByMessageID(messageID int64) (*Article, error) {
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, message_id
			  FROM articles WHERE message_id = ?`

	var article Article
	var sentAt int64
	var fetchedAt int64
	var msgID int64

	err := s.db.QueryRow(query, messageID).Scan(
		&article.ID,
		&article.Title,
		&article.URL,
		&article.Summary,
		&article.Tags,
		&article.Score,
		&fetchedAt,
		&sentAt,
		&msgID,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("article not found for message_id: %d", messageID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article by message_id: %w", err)
	}

	article.FetchedAt = time.Unix(fetchedAt, 0)
	if sentAt != 0 {
		article.SentAt = sql.NullTime{Time: time.Unix(sentAt, 0), Valid: true}
	}
	article.MessageID = sql.NullInt64{Int64: msgID, Valid: true}

	return &article, nil
}

func (s *Storage) GetRecentArticles(days int) ([]Article, error) {
	query := `SELECT id, title, url, summary, tags, score, fetched_at, sent_at, message_id
			  FROM articles WHERE sent_at IS NOT NULL
			  AND sent_at > ? ORDER BY sent_at DESC`

	threshold := time.Now().AddDate(0, 0, -days).Unix()

	rows, err := s.db.Query(query, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent articles: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var article Article
		var sentAt int64
		var fetchedAt int64
		var messageID int64

		if err := rows.Scan(
			&article.ID,
			&article.Title,
			&article.URL,
			&article.Summary,
			&article.Tags,
			&article.Score,
			&fetchedAt,
			&sentAt,
			&messageID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan article: %w", err)
		}

		article.FetchedAt = time.Unix(fetchedAt, 0)
		if sentAt != 0 {
			article.SentAt = sql.NullTime{Time: time.Unix(sentAt, 0), Valid: true}
		}
		if messageID != 0 {
			article.MessageID = sql.NullInt64{Int64: messageID, Valid: true}
		}

		articles = append(articles, article)
	}

	return articles, nil
}

func (s *Storage) LikeArticle(articleID int64) error {
	query := `INSERT OR IGNORE INTO likes (article_id, liked_at) VALUES (?, ?)`

	_, err := s.db.Exec(query, articleID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to like article: %w", err)
	}
	return nil
}

func (s *Storage) IsArticleLiked(articleID int64) (bool, error) {
	query := `SELECT 1 FROM likes WHERE article_id = ?`

	var exists int
	err := s.db.QueryRow(query, articleID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if article liked: %w", err)
	}
	return true, nil
}

func (s *Storage) GetLike(articleID int64) (*Like, error) {
	query := `SELECT article_id, liked_at FROM likes WHERE article_id = ?`

	var like Like
	var likedAt int64

	err := s.db.QueryRow(query, articleID).Scan(&like.ArticleID, &likedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("like not found for article: %d", articleID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get like: %w", err)
	}

	like.LikedAt = time.Unix(likedAt, 0)
	return &like, nil
}

func (s *Storage) GetLikeCount() (int, error) {
	query := `SELECT COUNT(*) FROM likes`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get like count: %w", err)
	}
	return count, nil
}

func (s *Storage) UpsertTagWeight(tag string, weight float64, count int) error {
	query := `INSERT INTO tag_weights (tag_name, weight, count)
			  VALUES (?, ?, ?)
			  ON CONFLICT(tag_name) DO UPDATE SET
			  weight = excluded.weight, count = excluded.count`

	_, err := s.db.Exec(query, tag, weight, count)
	if err != nil {
		return fmt.Errorf("failed to upsert tag weight: %w", err)
	}
	return nil
}

func (s *Storage) GetTagWeight(tag string) (*TagWeight, error) {
	query := `SELECT tag_name, weight, count FROM tag_weights WHERE tag_name = ?`

	var tw TagWeight
	err := s.db.QueryRow(query, tag).Scan(&tw.Name, &tw.Weight, &tw.Count)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tag weight not found: %s", tag)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tag weight: %w", err)
	}
	return &tw, nil
}

func (s *Storage) GetTagsByWeight() ([]TagWeight, error) {
	query := `SELECT tag_name, weight, count FROM tag_weights ORDER BY weight DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag weights: %w", err)
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Name, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tag weight: %w", err)
		}
		tags = append(tags, tw)
	}

	return tags, nil
}

func (s *Storage) DecayAllTags(decayRate, minWeight float64) error {
	query := `UPDATE tag_weights SET weight = CASE
			  WHEN weight * (1 - ?) < ? THEN ?
			  ELSE weight * (1 - ?)
			  END`

	_, err := s.db.Exec(query, decayRate, minWeight, minWeight, decayRate)
	if err != nil {
		return fmt.Errorf("failed to decay tags: %w", err)
	}
	return nil
}

func (s *Storage) SetSetting(key, value string) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`

	_, err := s.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}
	return nil
}

func (s *Storage) GetSetting(key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`

	var value string
	err := s.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting: %w", err)
	}
	return value, nil
}

func RoundToDecimal(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}

func SentAtTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

func MessageID(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: true}
}
