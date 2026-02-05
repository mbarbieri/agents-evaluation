package bot

// MessageSender sends messages to Telegram.
type MessageSender interface {
	SendHTML(chatID int64, text string) (int, error)
}

// ArticleLookup retrieves articles by Telegram message ID.
type ArticleLookup interface {
	GetArticleBySentMsgID(msgID int) (*StoredArticle, error)
}

// LikeTracker records and checks article likes.
type LikeTracker interface {
	IsLiked(articleID int) (bool, error)
	RecordLike(articleID int) error
}

// TagBooster boosts tag weights on like events.
type TagBooster interface {
	GetTagWeight(tag string) (*TagWeightInfo, error)
	UpsertTagWeight(tag string, weight float64, count int) error
}

// SettingsStore reads and writes user settings.
type SettingsStore interface {
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

// StatsProvider provides statistics data.
type StatsProvider interface {
	GetTopTagWeights(limit int) ([]TagWeightInfo, error)
	GetLikeCount() (int, error)
}

// ScheduleUpdater allows updating the cron schedule.
type ScheduleUpdater interface {
	Schedule(digestTime string, task func()) error
}

// StoredArticle is the article data needed by the bot.
type StoredArticle struct {
	ID   int
	Tags string // JSON array
}

// TagWeightInfo holds tag weight data.
type TagWeightInfo struct {
	Tag    string
	Weight float64
	Count  int
}
