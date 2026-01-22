package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel errors for dependency interfaces
var (
	ErrSettingNotFound  = errors.New("setting not found")
	ErrArticleNotFound  = errors.New("article not found")
)

// MessageSender sends messages to Telegram.
type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, html bool) (int64, error)
}

// SettingsStore manages persistent settings.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// ScheduleUpdater updates the digest schedule.
type ScheduleUpdater interface {
	Schedule(timeStr string, fn func()) error
}

// LikeTracker tracks article likes.
type LikeTracker interface {
	IsArticleLiked(ctx context.Context, articleID int64) (bool, error)
	LikeArticle(ctx context.Context, articleID int64) error
	GetLikeCount(ctx context.Context) (int, error)
}

// TagBooster boosts tag weights.
type TagBooster interface {
	BoostTagWeight(ctx context.Context, tag string, boost float64) error
}

// TagStatsProvider provides tag statistics.
type TagStatsProvider interface {
	GetTopTags(ctx context.Context, limit int) ([]TagStat, error)
}

// ArticleLookup finds articles by message ID.
type ArticleLookup interface {
	GetArticleByMessageID(ctx context.Context, msgID int64) (*ArticleInfo, error)
}

// DigestTrigger triggers a digest manually.
type DigestTrigger interface {
	TriggerDigest(ctx context.Context) error
}

// TagStat holds tag statistics.
type TagStat struct {
	Tag    string
	Weight float64
}

// ArticleInfo holds article data needed for reaction handling.
type ArticleInfo struct {
	ID   int64
	Tags []string
}

// ArticleForDisplay holds article data for message formatting.
type ArticleForDisplay struct {
	ID       int64
	Title    string
	Summary  string
	HNScore  int
	Comments int
	URL      string
}

var timeRegex = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// CommandHandler handles bot commands.
type CommandHandler struct {
	sender         MessageSender
	settings       SettingsStore
	schedUpdater   ScheduleUpdater
	likeTracker    LikeTracker
	tagStats       TagStatsProvider
	digestTrigger  DigestTrigger
}

// NewCommandHandler creates a new command handler.
func NewCommandHandler(
	sender MessageSender,
	settings SettingsStore,
	schedUpdater ScheduleUpdater,
	likeTracker LikeTracker,
	tagStats TagStatsProvider,
) *CommandHandler {
	return &CommandHandler{
		sender:       sender,
		settings:     settings,
		schedUpdater: schedUpdater,
		likeTracker:  likeTracker,
		tagStats:     tagStats,
	}
}

// HandleStart handles the /start command.
func (h *CommandHandler) HandleStart(ctx context.Context, chatID int64) error {
	// Save chat ID
	if err := h.settings.SetSetting(ctx, "chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("save chat_id: %w", err)
	}

	msg := "Welcome to the HN Digest Bot! 🗞️\n\n" +
		"Commands:\n" +
		"/fetch - Get your personalized digest now\n" +
		"/settings - View or update digest settings\n" +
		"/stats - View your interests and stats\n\n" +
		"React with 👍 to articles you like to train your preferences!"

	_, err := h.sender.SendMessage(ctx, chatID, msg, false)
	return err
}

// HandleSettings handles the /settings command.
func (h *CommandHandler) HandleSettings(ctx context.Context, chatID int64, args string) error {
	args = strings.TrimSpace(args)

	// No args: display current settings
	if args == "" {
		return h.displaySettings(ctx, chatID)
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return h.sendSettingsUsage(ctx, chatID)
	}

	subCmd := strings.ToLower(parts[0])
	value := strings.TrimSpace(parts[1])

	switch subCmd {
	case "time":
		return h.updateDigestTime(ctx, chatID, value)
	case "count":
		return h.updateArticleCount(ctx, chatID, value)
	default:
		return h.sendSettingsUsage(ctx, chatID)
	}
}

func (h *CommandHandler) displaySettings(ctx context.Context, chatID int64) error {
	digestTime := "09:00"
	if t, err := h.settings.GetSetting(ctx, "digest_time"); err == nil {
		digestTime = t
	}

	articleCount := "30"
	if c, err := h.settings.GetSetting(ctx, "article_count"); err == nil {
		articleCount = c
	}

	msg := fmt.Sprintf("Current Settings:\n\n"+
		"📅 Digest Time: %s\n"+
		"📰 Articles per Digest: %s\n\n"+
		"Update with:\n"+
		"/settings time HH:MM\n"+
		"/settings count N", digestTime, articleCount)

	_, err := h.sender.SendMessage(ctx, chatID, msg, false)
	return err
}

func (h *CommandHandler) updateDigestTime(ctx context.Context, chatID int64, timeStr string) error {
	if !timeRegex.MatchString(timeStr) {
		_, err := h.sender.SendMessage(ctx, chatID, "Invalid time format. Use HH:MM (e.g., 09:00, 18:30)", false)
		return err
	}

	if err := h.settings.SetSetting(ctx, "digest_time", timeStr); err != nil {
		return fmt.Errorf("save digest_time: %w", err)
	}

	// Update scheduler if available
	if h.schedUpdater != nil {
		// Note: The actual digest function is set up in main
		h.schedUpdater.Schedule(timeStr, func() {})
	}

	msg := fmt.Sprintf("✅ Digest time updated to %s", timeStr)
	_, err := h.sender.SendMessage(ctx, chatID, msg, false)
	return err
}

func (h *CommandHandler) updateArticleCount(ctx context.Context, chatID int64, countStr string) error {
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 100 {
		_, err := h.sender.SendMessage(ctx, chatID, "Invalid count. Must be a number between 1 and 100.", false)
		return err
	}

	if err := h.settings.SetSetting(ctx, "article_count", countStr); err != nil {
		return fmt.Errorf("save article_count: %w", err)
	}

	msg := fmt.Sprintf("✅ Article count updated to %d", count)
	_, err = h.sender.SendMessage(ctx, chatID, msg, false)
	return err
}

func (h *CommandHandler) sendSettingsUsage(ctx context.Context, chatID int64) error {
	msg := "Usage:\n" +
		"/settings - Show current settings\n" +
		"/settings time HH:MM - Update digest time\n" +
		"/settings count N - Update article count (1-100)"
	_, err := h.sender.SendMessage(ctx, chatID, msg, false)
	return err
}

// HandleStats handles the /stats command.
func (h *CommandHandler) HandleStats(ctx context.Context, chatID int64) error {
	likeCount, err := h.likeTracker.GetLikeCount(ctx)
	if err != nil {
		return fmt.Errorf("get like count: %w", err)
	}

	if likeCount == 0 {
		msg := "No likes yet! React with 👍 to articles to train your preferences."
		_, err := h.sender.SendMessage(ctx, chatID, msg, false)
		return err
	}

	topTags, err := h.tagStats.GetTopTags(ctx, 10)
	if err != nil {
		return fmt.Errorf("get top tags: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("📊 Your Interests:\n\n")

	for i, tag := range topTags {
		sb.WriteString(fmt.Sprintf("%d. %s (%.2f)\n", i+1, tag.Tag, tag.Weight))
	}

	sb.WriteString(fmt.Sprintf("\nTotal articles liked: %d", likeCount))

	_, err = h.sender.SendMessage(ctx, chatID, sb.String(), false)
	return err
}

// HandleFetch handles the /fetch command.
func (h *CommandHandler) HandleFetch(ctx context.Context, chatID int64) error {
	if h.digestTrigger != nil {
		return h.digestTrigger.TriggerDigest(ctx)
	}
	return nil
}

// ReactionHandler handles message reactions.
type ReactionHandler struct {
	articleLookup ArticleLookup
	likeTracker   LikeTracker
	tagBooster    TagBooster
	boostAmount   float64
}

// NewReactionHandler creates a new reaction handler.
func NewReactionHandler(
	articleLookup ArticleLookup,
	likeTracker LikeTracker,
	tagBooster TagBooster,
	boostAmount float64,
) *ReactionHandler {
	return &ReactionHandler{
		articleLookup: articleLookup,
		likeTracker:   likeTracker,
		tagBooster:    tagBooster,
		boostAmount:   boostAmount,
	}
}

// HandleReaction processes a reaction event.
func (h *ReactionHandler) HandleReaction(ctx context.Context, messageID int64, emoji string) error {
	// Only process thumbs-up
	if emoji != "👍" {
		return nil
	}

	// Look up article
	article, err := h.articleLookup.GetArticleByMessageID(ctx, messageID)
	if err != nil {
		if errors.Is(err, ErrArticleNotFound) {
			return nil // Silently ignore reactions to non-article messages
		}
		return fmt.Errorf("lookup article: %w", err)
	}

	// Check if already liked (idempotent)
	liked, err := h.likeTracker.IsArticleLiked(ctx, article.ID)
	if err != nil {
		return fmt.Errorf("check if liked: %w", err)
	}
	if liked {
		return nil // Already liked, no-op
	}

	// Record the like
	if err := h.likeTracker.LikeArticle(ctx, article.ID); err != nil {
		return fmt.Errorf("record like: %w", err)
	}

	// Boost tags
	for _, tag := range article.Tags {
		if err := h.tagBooster.BoostTagWeight(ctx, tag, h.boostAmount); err != nil {
			return fmt.Errorf("boost tag %s: %w", tag, err)
		}
	}

	return nil
}

// FormatArticleMessage formats an article for display in Telegram.
func FormatArticleMessage(article *ArticleForDisplay) string {
	title := html.EscapeString(article.Title)
	summary := html.EscapeString(article.Summary)
	hnURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", article.ID)

	return fmt.Sprintf(
		"📰 <b>%s</b>\n\n"+
			"<i>%s</i>\n\n"+
			"⬆️ %d points | 💬 %d comments\n"+
			"<a href=\"%s\">Article</a> | <a href=\"%s\">HN Discussion</a>",
		title, summary, article.HNScore, article.Comments, article.URL, hnURL,
	)
}
