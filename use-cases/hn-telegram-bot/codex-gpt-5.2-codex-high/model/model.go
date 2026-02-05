package model

import "time"

// Article represents a processed HN article and its metadata.
type Article struct {
	ID           int64
	Title        string
	URL          string
	Summary      string
	Tags         []string
	HNScore      int
	Comments     int
	FetchedAt    time.Time
	SentAt       *time.Time
	TelegramMsgID int
}

// SummaryResult holds summarizer output.
type SummaryResult struct {
	Summary string
	Tags    []string
}

// TagWeight represents a learned tag preference.
type TagWeight struct {
	Tag       string
	Weight    float64
	Count     int
}
