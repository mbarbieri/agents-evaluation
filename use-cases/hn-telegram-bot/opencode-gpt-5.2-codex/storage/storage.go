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

type Store struct {
	db *sql.DB
}

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      []string
	Score     int
	FetchedAt time.Time
	SentAt    *time.Time
	MessageID *int
}

type TagWeight struct {
	Tag    string
	Weight float64
	Count  int
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func applySchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT NOT NULL,
			tags_json TEXT NOT NULL,
			score INTEGER NOT NULL,
			fetched_at TEXT NOT NULL,
			sent_at TEXT,
			message_id INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL NOT NULL,
			occurrence_count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_message_id ON articles(message_id);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	return nil
}

func (s *Store) UpsertArticle(ctx context.Context, article Article) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if article.ID == 0 {
		return errors.New("article id required")
	}
	if article.FetchedAt.IsZero() {
		return errors.New("fetched_at required")
	}
	if article.Title == "" || article.URL == "" || article.Summary == "" {
		return errors.New("title, url, summary required")
	}

	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO articles (id, title, url, summary, tags_json, score, fetched_at, sent_at, message_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			url=excluded.url,
			summary=excluded.summary,
			tags_json=excluded.tags_json,
			score=excluded.score,
			fetched_at=excluded.fetched_at,
			sent_at=excluded.sent_at,
			message_id=excluded.message_id
	`,
		article.ID,
		article.Title,
		article.URL,
		article.Summary,
		string(tagsJSON),
		article.Score,
		article.FetchedAt.Format(time.RFC3339Nano),
		timeToString(article.SentAt),
		article.MessageID,
	)
	if err != nil {
		return fmt.Errorf("upsert article: %w", err)
	}
	return nil
}

func (s *Store) MarkArticleSent(ctx context.Context, id int64, sentAt time.Time, messageID int) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if id == 0 {
		return errors.New("article id required")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE articles SET sent_at = ?, message_id = ? WHERE id = ?
	`, sentAt.Format(time.RFC3339Nano), messageID, id)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}

func (s *Store) GetArticleByMessageID(ctx context.Context, messageID int) (Article, bool, error) {
	if s == nil || s.db == nil {
		return Article{}, false, errors.New("store not initialized")
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, url, summary, tags_json, score, fetched_at, sent_at, message_id
		FROM articles WHERE message_id = ?
	`, messageID)

	article, err := scanArticle(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Article{}, false, nil
		}
		return Article{}, false, fmt.Errorf("get article by message: %w", err)
	}
	return article, true, nil
}

func (s *Store) GetRecentlySentArticleIDs(ctx context.Context, since time.Time) ([]int64, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM articles WHERE sent_at IS NOT NULL AND sent_at >= ?
	`, since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("query recent ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan recent id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter recent ids: %w", err)
	}
	return ids, nil
}

func (s *Store) IsLiked(ctx context.Context, articleID int64) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("store not initialized")
	}
	row := s.db.QueryRowContext(ctx, `SELECT article_id FROM likes WHERE article_id = ?`, articleID)
	var id int64
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check like: %w", err)
	}
	return true, nil
}

func (s *Store) AddLike(ctx context.Context, articleID int64, likedAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if articleID == 0 {
		return errors.New("article id required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO likes (article_id, liked_at) VALUES (?, ?)
	`, articleID, likedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("add like: %w", err)
	}
	return nil
}

func (s *Store) CountLikes(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store not initialized")
	}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM likes`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count likes: %w", err)
	}
	return count, nil
}

func (s *Store) BoostTags(ctx context.Context, tags []string, boost float64) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if len(tags) == 0 {
		return nil
	}
	if boost < 0 {
		return errors.New("boost must be non-negative")
	}

	for _, tag := range tags {
		if tag == "" {
			continue
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tag_weights (tag, weight, occurrence_count)
			VALUES (?, ?, 1)
			ON CONFLICT(tag) DO UPDATE SET
				weight = tag_weights.weight + ?,
				occurrence_count = tag_weights.occurrence_count + 1
		`, tag, 1.0+boost, boost)
		if err != nil {
			return fmt.Errorf("boost tag %s: %w", tag, err)
		}
	}
	return nil
}

func (s *Store) DecayTags(ctx context.Context, decayRate, minWeight float64) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if decayRate < 0 || decayRate > 1 {
		return errors.New("decayRate must be between 0 and 1")
	}
	if minWeight <= 0 {
		return errors.New("minWeight must be positive")
	}

	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight, occurrence_count FROM tag_weights`)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}
	defer rows.Close()

	type update struct {
		tag    string
		weight float64
		count  int
	}
	var updates []update
	for rows.Next() {
		var u update
		if err := rows.Scan(&u.tag, &u.weight, &u.count); err != nil {
			return fmt.Errorf("scan tag: %w", err)
		}
		newWeight := u.weight * (1 - decayRate)
		if newWeight < minWeight {
			newWeight = minWeight
		}
		u.weight = newWeight
		updates = append(updates, u)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iter tag weights: %w", err)
	}

	for _, u := range updates {
		_, err := s.db.ExecContext(ctx, `
			UPDATE tag_weights SET weight = ? WHERE tag = ?
		`, u.weight, u.tag)
		if err != nil {
			return fmt.Errorf("update tag %s: %w", u.tag, err)
		}
	}
	return nil
}

func (s *Store) GetTopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	if limit <= 0 {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tag, weight, occurrence_count
		FROM tag_weights
		ORDER BY weight DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query top tags: %w", err)
	}
	defer rows.Close()

	var tags []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Count); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter top tags: %w", err)
	}
	return tags, nil
}

func (s *Store) GetTagWeights(ctx context.Context) (map[string]float64, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight FROM tag_weights`)
	if err != nil {
		return nil, fmt.Errorf("query tag weights: %w", err)
	}
	defer rows.Close()

	weights := map[string]float64{}
	for rows.Next() {
		var tag string
		var weight float64
		if err := rows.Scan(&tag, &weight); err != nil {
			return nil, fmt.Errorf("scan tag weight: %w", err)
		}
		weights[tag] = weight
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter tag weights: %w", err)
	}
	return weights, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if key == "" {
		return errors.New("setting key required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("store not initialized")
	}
	if key == "" {
		return "", false, errors.New("setting key required")
	}
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

func scanArticle(row *sql.Row) (Article, error) {
	var (
		article   Article
		tagsJSON  string
		fetchedAt string
		sentAt    sql.NullString
		messageID sql.NullInt64
	)
	if err := row.Scan(
		&article.ID,
		&article.Title,
		&article.URL,
		&article.Summary,
		&tagsJSON,
		&article.Score,
		&fetchedAt,
		&sentAt,
		&messageID,
	); err != nil {
		return Article{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
		return Article{}, fmt.Errorf("unmarshal tags: %w", err)
	}
	parsedFetched, err := time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return Article{}, fmt.Errorf("parse fetched_at: %w", err)
	}
	article.FetchedAt = parsedFetched
	if sentAt.Valid {
		parsedSent, err := time.Parse(time.RFC3339Nano, sentAt.String)
		if err != nil {
			return Article{}, fmt.Errorf("parse sent_at: %w", err)
		}
		article.SentAt = &parsedSent
	}
	if messageID.Valid {
		id := int(messageID.Int64)
		article.MessageID = &id
	}
	return article, nil
}

func timeToString(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339Nano)
}
