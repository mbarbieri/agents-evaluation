package digest

import (
	"context"
	"errors"

	"hn-telegram-bot/hn"
)

type HNAdapter struct {
	Client *hn.Client
}

func (a *HNAdapter) TopStories(ctx context.Context) ([]int64, error) {
	if a == nil || a.Client == nil {
		return nil, errors.New("hn client not initialized")
	}
	return a.Client.TopStories(ctx)
}

func (a *HNAdapter) Item(ctx context.Context, id int64) (Item, error) {
	if a == nil || a.Client == nil {
		return Item{}, errors.New("hn client not initialized")
	}
	item, err := a.Client.Item(ctx, id)
	if err != nil {
		return Item{}, err
	}
	return Item{
		ID:          item.ID,
		Title:       item.Title,
		URL:         item.URL,
		Score:       item.Score,
		Descendants: item.Descendants,
	}, nil
}
