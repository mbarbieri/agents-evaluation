package digest

import (
	"context"
	"errors"

	"hn-telegram-bot/summarizer"
)

type SummarizerAdapter struct {
	Client *summarizer.Client
}

func (a *SummarizerAdapter) Summarize(ctx context.Context, content string) (Summary, error) {
	if a == nil || a.Client == nil {
		return Summary{}, errors.New("summarizer not initialized")
	}
	result, err := a.Client.Summarize(ctx, content)
	if err != nil {
		return Summary{}, err
	}
	return Summary{Summary: result.Summary, Tags: result.Tags}, nil
}
