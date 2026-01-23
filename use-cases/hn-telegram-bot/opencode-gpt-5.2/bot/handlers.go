package bot

import (
	"context"
	"fmt"
	"time"
)

type Sender interface {
	SendText(ctx context.Context, chatID int64, text string) error
	SendHTML(ctx context.Context, chatID int64, html string) (messageID int, err error)
}

type DigestRunner interface {
	Run(ctx context.Context) error
}

type StatsStore interface {
	TopTags(ctx context.Context, limit int) ([]TagWeight, error)
	LikeCount(ctx context.Context) (int, error)
}

type TagWeight struct {
	Tag    string
	Weight float64
}

func HandleFetch(ctx context.Context, sender Sender, chatID int64, runner DigestRunner) {
	_ = runner.Run(ctx)
}

func HandleStats(ctx context.Context, sender Sender, chatID int64, st StatsStore) error {
	likes, err := st.LikeCount(ctx)
	if err != nil {
		return err
	}
	if likes == 0 {
		return sender.SendText(ctx, chatID, "No likes yet. React with \U0001F44D on article messages to train preferences.")
	}
	tags, err := st.TopTags(ctx, 10)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Total likes: %d\nTop tags:\n", likes)
	for _, tw := range tags {
		msg += fmt.Sprintf("- %s: %.2f\n", tw.Tag, tw.Weight)
	}
	return sender.SendText(ctx, chatID, msg)
}

type ReactionStore interface {
	ArticleByTelegramMessageID(ctx context.Context, messageID int) (ReactionArticle, error)
	IsLiked(ctx context.Context, articleID int) (bool, error)
	BoostTagsOnLike(ctx context.Context, tags []string, boost float64) error
	RecordLike(ctx context.Context, articleID int, likedAt time.Time) error
}

type ReactionArticle struct {
	ID   int
	Tags []string
}

type ReactionConfig struct {
	Boost float64
}

func HandleThumbsUpReaction(ctx context.Context, st ReactionStore, messageID int, cfg ReactionConfig) error {
	art, err := st.ArticleByTelegramMessageID(ctx, messageID)
	if err != nil {
		return nil
	}
	liked, err := st.IsLiked(ctx, art.ID)
	if err != nil {
		return err
	}
	if liked {
		return nil
	}
	if err := st.BoostTagsOnLike(ctx, art.Tags, cfg.Boost); err != nil {
		return err
	}
	if err := st.RecordLike(ctx, art.ID, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}
