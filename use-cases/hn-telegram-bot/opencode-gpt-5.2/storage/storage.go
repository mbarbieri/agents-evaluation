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
	ID                int
	Title             string
	URL               string
	Summary           string
	Tags              []string
	HNScore           int
	HNCommentCount    int
	FetchedAt         time.Time
	SentAt            *time.Time
	TelegramMessageID *int
}

func Open(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	st := &Store{db: db}
	if err := st.InitSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) InitSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT NOT NULL,
			tags_json TEXT NOT NULL,
			hn_score INTEGER NOT NULL,
			hn_comment_count INTEGER NOT NULL,
			fetched_at_unix INTEGER NOT NULL,
			sent_at_unix INTEGER,
			telegram_message_id INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_sent_at ON articles(sent_at_unix);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_message_id ON articles(telegram_message_id) WHERE telegram_message_id IS NOT NULL;`,
		`CREATE TABLE IF NOT EXISTS likes (
			article_id INTEGER PRIMARY KEY,
			liked_at_unix INTEGER NOT NULL,
			FOREIGN KEY(article_id) REFERENCES articles(id)
		);`,
		`CREATE TABLE IF NOT EXISTS tag_weights (
			tag TEXT PRIMARY KEY,
			weight REAL NOT NULL,
			occurrences INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (s *Store) UpsertArticle(ctx context.Context, a Article) error {
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	fetched := a.FetchedAt.Unix()
	var sent any
	var msg any
	if a.SentAt != nil {
		sent = a.SentAt.Unix()
	}
	if a.TelegramMessageID != nil {
		msg = *a.TelegramMessageID
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO articles (id, title, url, summary, tags_json, hn_score, hn_comment_count, fetched_at_unix, sent_at_unix, telegram_message_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			url=excluded.url,
			summary=excluded.summary,
			tags_json=excluded.tags_json,
			hn_score=excluded.hn_score,
			hn_comment_count=excluded.hn_comment_count,
			fetched_at_unix=excluded.fetched_at_unix,
			sent_at_unix=excluded.sent_at_unix,
			telegram_message_id=excluded.telegram_message_id
	`, a.ID, a.Title, a.URL, a.Summary, string(tagsJSON), a.HNScore, a.HNCommentCount, fetched, sent, msg)
	if err != nil {
		return fmt.Errorf("upsert article: %w", err)
	}
	return nil
}

func (s *Store) MarkArticleSent(ctx context.Context, articleID int, sentAt time.Time, telegramMessageID int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE articles SET sent_at_unix = ?, telegram_message_id = ? WHERE id = ?`, sentAt.Unix(), telegramMessageID, articleID)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}

var ErrNotFound = errors.New("not found")

func (s *Store) ArticleByTelegramMessageID(ctx context.Context, messageID int) (Article, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, url, summary, tags_json, hn_score, hn_comment_count, fetched_at_unix, sent_at_unix, telegram_message_id
		FROM articles
		WHERE telegram_message_id = ?
	`, messageID)

	var a Article
	var tagsJSON string
	var fetchedUnix int64
	var sentUnix sql.NullInt64
	var msgID sql.NullInt64
	if err := row.Scan(&a.ID, &a.Title, &a.URL, &a.Summary, &tagsJSON, &a.HNScore, &a.HNCommentCount, &fetchedUnix, &sentUnix, &msgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Article{}, ErrNotFound
		}
		return Article{}, fmt.Errorf("get article by message id: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
		return Article{}, fmt.Errorf("unmarshal tags: %w", err)
	}
	a.FetchedAt = time.Unix(fetchedUnix, 0).UTC()
	if sentUnix.Valid {
		t := time.Unix(sentUnix.Int64, 0).UTC()
		a.SentAt = &t
	}
	if msgID.Valid {
		v := int(msgID.Int64)
		a.TelegramMessageID = &v
	}
	return a, nil
}

func (s *Store) SentArticleIDsSince(ctx context.Context, since time.Time) (map[int]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM articles WHERE sent_at_unix IS NOT NULL AND sent_at_unix >= ?`, since.Unix())
	if err != nil {
		return nil, fmt.Errorf("query sent ids: %w", err)
	}
	defer rows.Close()

	out := map[int]struct{}{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan sent id: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sent ids: %w", err)
	}
	return out, nil
}

func (s *Store) IsLiked(ctx context.Context, articleID int) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT 1 FROM likes WHERE article_id = ?`, articleID)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("is liked: %w", err)
	}
	return true, nil
}

func (s *Store) RecordLike(ctx context.Context, articleID int, likedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO likes (article_id, liked_at_unix) VALUES (?, ?)
		ON CONFLICT(article_id) DO NOTHING
	`, articleID, likedAt.Unix())
	if err != nil {
		return fmt.Errorf("record like: %w", err)
	}
	return nil
}

func (s *Store) LikeCount(ctx context.Context) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM likes`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, fmt.Errorf("like count: %w", err)
	}
	return c, nil
}

type TagWeight struct {
	Tag         string
	Weight      float64
	Occurrences int
}

func (s *Store) BoostTagsOnLike(ctx context.Context, tags []string, boost float64) error {
	if len(tags) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, tag := range tags {
		if tag == "" {
			continue
		}
		// New tag starts at 1.0 + boost.
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tag_weights (tag, weight, occurrences) VALUES (?, ?, 1)
			ON CONFLICT(tag) DO UPDATE SET
				weight = tag_weights.weight + ?,
				occurrences = tag_weights.occurrences + 1
		`, tag, 1.0+boost, boost)
		if err != nil {
			return fmt.Errorf("boost tag %q: %w", tag, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (s *Store) ApplyDecay(ctx context.Context, decayRate float64, minWeight float64) error {
	if decayRate < 0 || decayRate >= 1 {
		return fmt.Errorf("invalid decay rate")
	}
	if minWeight <= 0 {
		return fmt.Errorf("invalid min weight")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight FROM tag_weights`)
	if err != nil {
		return fmt.Errorf("query tag weights: %w", err)
	}
	defer rows.Close()

	type row struct {
		tag    string
		weight float64
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.tag, &r.weight); err != nil {
			return fmt.Errorf("scan tag weights: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tag weights: %w", err)
	}

	for _, r := range all {
		newW := r.weight * (1 - decayRate)
		if newW < minWeight {
			newW = minWeight
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE tag_weights SET weight = ? WHERE tag = ?`, newW, r.tag); err != nil {
			return fmt.Errorf("update tag %q: %w", r.tag, err)
		}
	}
	return nil
}

func (s *Store) GetTagWeights(ctx context.Context, tags []string) (map[string]float64, error) {
	out := map[string]float64{}
	for _, t := range tags {
		if t != "" {
			out[t] = 1.0
		}
	}
	if len(out) == 0 {
		return out, nil
	}

	// Query each tag individually to keep SQL simple and deterministic.
	for tag := range out {
		row := s.db.QueryRowContext(ctx, `SELECT weight FROM tag_weights WHERE tag = ?`, tag)
		var w float64
		switch err := row.Scan(&w); {
		case err == nil:
			out[tag] = w
		case errors.Is(err, sql.ErrNoRows):
			// default 1.0
		default:
			return nil, fmt.Errorf("get weight %q: %w", tag, err)
		}
	}
	return out, nil
}

func (s *Store) TopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT tag, weight, occurrences FROM tag_weights ORDER BY weight DESC, tag ASC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("top tags: %w", err)
	}
	defer rows.Close()

	var out []TagWeight
	for rows.Next() {
		var tw TagWeight
		if err := rows.Scan(&tw.Tag, &tw.Weight, &tw.Occurrences); err != nil {
			return nil, fmt.Errorf("scan top tags: %w", err)
		}
		out = append(out, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top tags: %w", err)
	}
	return out, nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	var v string
	if err := row.Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return v, nil
}

func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}
