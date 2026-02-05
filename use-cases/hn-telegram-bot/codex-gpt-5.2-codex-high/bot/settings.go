package bot

import "sync"

// Settings holds runtime settings with thread-safe access.
type Settings struct {
	mu           sync.RWMutex
	chatID       int64
	digestTime   string
	articleCount int
}

// NewSettings initializes settings.
func NewSettings(chatID int64, digestTime string, articleCount int) *Settings {
	return &Settings{chatID: chatID, digestTime: digestTime, articleCount: articleCount}
}

func (s *Settings) ChatID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatID
}

func (s *Settings) SetChatID(id int64) {
	s.mu.Lock()
	s.chatID = id
	s.mu.Unlock()
}

func (s *Settings) DigestTime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.digestTime
}

func (s *Settings) SetDigestTime(value string) {
	s.mu.Lock()
	s.digestTime = value
	s.mu.Unlock()
}

func (s *Settings) ArticleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.articleCount
}

func (s *Settings) SetArticleCount(value int) {
	s.mu.Lock()
	s.articleCount = value
	s.mu.Unlock()
}
