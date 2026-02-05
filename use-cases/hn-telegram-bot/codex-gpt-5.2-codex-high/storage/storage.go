package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"hn-telegram-bot/model"
)

// Storage provides persistence operations.
type Storage struct {
	db *sql.DB
}

// New returns a new Storage instance.
func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// Init creates database tables if they do not exist.
func (s *Storage) Init(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT NOT NULL,
			tags TEXT NOT NULL,
			hn_score INTEGER NOT NULL,
			comments INTEGER NOT NULL,
			fetched_at DATETIME NOT NULL,
			sent_at DATETIME NULL,
			telegram_msg_id INTEGER NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_msg_id ON articles(telegram_msg_id);`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at DATETIME NOT NULL,
			FOREIGN KEY(article_id) REFERENCES articles(id)
		);`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL NOT NULL,
			count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}
	return nil
}

// UpsertArticle inserts or updates an article.
func (s *Storage) UpsertArticle(ctx context.Context, article model.Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	var sentAt sql.NullTime
	if article.SentAt != nil {
		sentAt = sql.NullTime{Time: *article.SentAt, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO articles (id, title, url, summary, tags, hn_score, comments, fetched_at, sent_at, telegram_msg_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			url = excluded.url,
			summary = excluded.summary,
			tags = excluded.tags,
			hn_score = excluded.hn_score,
			comments = excluded.comments,
			fetched_at = excluded.fetched_at,
			sent_at = excluded.sent_at,
			telegram_msg_id = excluded.telegram_msg_id
	`, article.ID, article.Title, article.URL, article.Summary, string(tagsJSON), article.HNScore, article.Comments, article.FetchedAt, sentAt, article.TelegramMsgID)
	if err != nil {
		return fmt.Errorf("upsert article: %w", err)
	}
	return nil
}

// GetArticleByMessageID fetches an article by telegram message ID.
func (s *Storage) GetArticleByMessageID(ctx context.Context, msgID int) (model.Article, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, url, summary, tags, hn_score, comments, fetched_at, sent_at, telegram_msg_id
		FROM articles WHERE telegram_msg_id = ?
	`, msgID)

	var article model.Article
	var tagsJSON string
	var sentAt sql.NullTime
	if err := row.Scan(&article.ID, &article.Title, &article.URL, &article.Summary, &tagsJSON, &article.HNScore, &article.Comments, &article.FetchedAt, &sentAt, &article.TelegramMsgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Article{}, false, nil
		}
		return model.Article{}, false, fmt.Errorf("get article by message: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return model.Article{}, false, fmt.Errorf("unmarshal tags: %w", err)
	}
	if sentAt.Valid {
		article.SentAt = &sentAt.Time
	}
	return article, true, nil
}

// ListSentArticleIDsSince returns article IDs sent since the given time.
func (s *Storage) ListSentArticleIDsSince(ctx context.Context, since time.Time) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM articles WHERE sent_at IS NOT NULL AND sent_at >= ?`, since)
	if err != nil {
		return nil, fmt.Errorf("list sent articles: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan sent article: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows sent articles: %w", err)
	}
	return ids, nil
}

// IsLiked reports whether an article is already liked.
func (s *Storage) IsLiked(ctx context.Context, articleID int64) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT 1 FROM likes WHERE article_id = ?`, articleID)
	var val int
	if err := row.Scan(&val); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check like: %w", err)
	}
	return true, nil
}

// AddLike records a like for an article.
func (s *Storage) AddLike(ctx context.Context, articleID int64, likedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO likes (article_id, liked_at) VALUES (?, ?) ON CONFLICT(article_id) DO NOTHING`, articleID, likedAt)
	if err != nil {
		return fmt.Errorf("insert like: %w", err)
	}
	return nil
}

// BoostTags increases tag weights and counts.
func (s *Storage) BoostTags(ctx context.Context, tags []string, boost float64) error {
	if len(tags) == 0 {
		return nil
	}
	defer func() {
		_ = recover()
	}()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin boost tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, tag := range tags {
		res, err := tx.ExecContext(ctx, `UPDATE tag_weights SET weight = weight + ?, count = count + 1 WHERE tag = ?`, boost, tag)
		if err != nil {
			return fmt.Errorf("update tag weight: %w", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected: %w", err)
		}
		if rows == 0 {
			_, err := tx.ExecContext(ctx, `INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)`, tag, 1.0+boost, 1)
			if err != nil {
				return fmt.Errorf("insert tag weight: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit boost tx: %w", err)
	}
	return nil
}

// ApplyDecay reduces tag weights by a decay rate with a minimum floor.
func (s *Storage) ApplyDecay(ctx context.Context, decayRate, minWeight float64) error {
	if decayRate <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE tag_weights
		SET weight = CASE
			WHEN weight * (1 - ?) < ? THEN ?
			ELSE weight * (1 - ?)
		END
	`, decayRate, minWeight, minWeight, decayRate)
	if err != nil {
		return fmt.Errorf("apply decay: %w", err)
	}
	return nil
}

// ListTopTags returns tags ordered by weight descending.
func (s *Storage) ListTopTags(ctx context.Context, limit int) ([]model.TagWeight, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight, count FROM tag_weights ORDER BY weight DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list top tags: %w", err)
	}
	defer rows.Close()

	var tags []model.TagWeight
	for rows.Next() {
		var tw model.TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("scan top tag: %w", err)
		}
		tags = append(tags, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows top tags: %w", err)
	}
	return tags, nil
}

// GetTagWeights returns all tag weights as a map.
func (s *Storage) GetTagWeights(ctx context.Context) (map[string]model.TagWeight, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight, count FROM tag_weights`)
	if err != nil {
		return nil, fmt.Errorf("get tag weights: %w", err)
	}
	defer rows.Close()

	weights := make(map[string]model.TagWeight)
	for rows.Next() {
		var tw model.TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("scan tag weight: %w", err)
		}
		weights[tw.Tag] = tw
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows tag weights: %w", err)
	}
	return weights, nil
}

// SetSetting sets a key-value pair in settings.
func (s *Storage) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

// GetSetting retrieves a setting value by key.
func (s *Storage) GetSetting(ctx context.Context, key string) (string, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get setting: %w", err)
	}
	return value, true, nil
}

// CountLikes returns the total number of liked articles.
func (s *Storage) CountLikes(ctx context.Context) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM likes`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count likes: %w", err)
	}
	return count, nil
}

// DB exposes the underlying sql.DB for advanced use.
func (s *Storage) DB() *sql.DB {
	return s.db
}
