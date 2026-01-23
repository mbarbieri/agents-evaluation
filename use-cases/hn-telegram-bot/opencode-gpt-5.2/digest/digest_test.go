package digest

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type fakeHN struct {
	top  []int
	item map[int]HNItem
	terr error
	ierr error
}

func (f fakeHN) TopStories(ctx context.Context) ([]int, error) {
	if f.terr != nil {
		return nil, f.terr
	}
	return append([]int(nil), f.top...), nil
}

func (f fakeHN) Item(ctx context.Context, id int) (HNItem, error) {
	if f.ierr != nil {
		return HNItem{}, f.ierr
	}
	it, ok := f.item[id]
	if !ok {
		return HNItem{}, errors.New("missing")
	}
	return it, nil
}

type fakeScraper struct{ err error }

func (f fakeScraper) Extract(ctx context.Context, url string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "content", nil
}

type fakeSummarizer struct {
	err error
}

func (f fakeSummarizer) Summarize(ctx context.Context, content string) (SummaryResult, error) {
	if f.err != nil {
		return SummaryResult{}, f.err
	}
	return SummaryResult{Summary: "sum", Tags: []string{"go"}}, nil
}

type fakeRanker struct {
	out []RankArticle
	err error
}

func (f fakeRanker) Rank(ctx context.Context, articles []RankArticle) ([]RankArticle, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.out != nil {
		return f.out, nil
	}
	return articles, nil
}

type fakeStore struct {
	recent  map[int]struct{}
	marked  []int
	upserts []int
	decayed bool
}

func (f *fakeStore) ApplyDecay(ctx context.Context, decayRate float64, minWeight float64) error {
	f.decayed = true
	return nil
}

func (f *fakeStore) SentArticleIDsSince(ctx context.Context, since time.Time) (map[int]struct{}, error) {
	if f.recent == nil {
		return map[int]struct{}{}, nil
	}
	return f.recent, nil
}

func (f *fakeStore) UpsertArticle(ctx context.Context, a StoredArticle) error {
	f.upserts = append(f.upserts, a.ID)
	return nil
}

func (f *fakeStore) MarkArticleSent(ctx context.Context, articleID int, sentAt time.Time, telegramMessageID int) error {
	f.marked = append(f.marked, articleID)
	return nil
}

type fakeSender struct {
	sent []int
	fail bool
}

func (f *fakeSender) SendArticle(ctx context.Context, a RankArticle) (int, error) {
	if f.fail {
		return 0, errors.New("send fail")
	}
	f.sent = append(f.sent, a.ID)
	return 1000 + a.ID, nil
}

func TestService_Run_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := &fakeStore{}
	snd := &fakeSender{}

	svc := Service{
		Log:        slog.Default(),
		HN:         fakeHN{top: []int{1, 2}, item: map[int]HNItem{1: {ID: 1, Title: "t1", URL: "u1", Score: 10, Descendants: 2}, 2: {ID: 2, Title: "t2", URL: "u2", Score: 5, Descendants: 1}}},
		Scraper:    fakeScraper{},
		Summarizer: fakeSummarizer{},
		Ranker:     fakeRanker{},
		Store:      st,
		Sender:     snd,
		Cfg:        Config{ArticleCount: 1, DecayRate: 0.02, MinTagWeight: 0.1, RecentWindow: 7 * 24 * time.Hour},
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !st.decayed {
		t.Fatalf("expected decay")
	}
	if len(snd.sent) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(snd.sent))
	}
	if len(st.marked) != 1 {
		t.Fatalf("expected marked")
	}
}

func TestService_Run_FiltersRecent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := &fakeStore{recent: map[int]struct{}{1: {}}}
	snd := &fakeSender{}

	svc := Service{
		HN:         fakeHN{top: []int{1, 2}, item: map[int]HNItem{1: {ID: 1, Title: "t1", URL: "u1"}, 2: {ID: 2, Title: "t2", URL: "u2"}}},
		Scraper:    fakeScraper{},
		Summarizer: fakeSummarizer{},
		Ranker:     fakeRanker{},
		Store:      st,
		Sender:     snd,
		Cfg:        Config{ArticleCount: 10, DecayRate: 0.02, MinTagWeight: 0.1, RecentWindow: 7 * 24 * time.Hour},
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(snd.sent) != 1 || snd.sent[0] != 2 {
		t.Fatalf("expected only 2 sent, got %+v", snd.sent)
	}
}

func TestService_Run_ScrapeFailureFallsBackToTitle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := &fakeStore{}
	snd := &fakeSender{}

	called := 0
	sum := fakeSummarizer{}
	// Wrap summarizer to capture content.
	wrap := summarizerFunc(func(ctx context.Context, content string) (SummaryResult, error) {
		called++
		if content != "t1" {
			t.Fatalf("expected fallback to title, got %q", content)
		}
		return sum.Summarize(ctx, content)
	})

	svc := Service{
		HN:         fakeHN{top: []int{1}, item: map[int]HNItem{1: {ID: 1, Title: "t1", URL: "u1"}}},
		Scraper:    fakeScraper{err: errors.New("boom")},
		Summarizer: wrap,
		Ranker:     fakeRanker{},
		Store:      st,
		Sender:     snd,
		Cfg:        Config{ArticleCount: 1, DecayRate: 0.02, MinTagWeight: 0.1, RecentWindow: 7 * 24 * time.Hour},
	}
	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected summarizer called")
	}
}

type summarizerFunc func(context.Context, string) (SummaryResult, error)

func (f summarizerFunc) Summarize(ctx context.Context, content string) (SummaryResult, error) {
	return f(ctx, content)
}
