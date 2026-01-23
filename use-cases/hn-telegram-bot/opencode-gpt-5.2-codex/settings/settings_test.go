package settings

import (
	"context"
	"testing"
)

type mockStore struct {
	values map[string]string
	set    map[string]string
}

func (m *mockStore) GetSetting(ctx context.Context, key string) (string, bool, error) {
	value, ok := m.values[key]
	return value, ok, nil
}

func (m *mockStore) SetSetting(ctx context.Context, key, value string) error {
	if m.set == nil {
		m.set = map[string]string{}
	}
	m.set[key] = value
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
	return nil
}

func TestManagerLoadsDefaults(t *testing.T) {
	t.Parallel()
	store := &mockStore{values: map[string]string{"digest_time": "10:00"}}
	manager, err := NewManager(context.Background(), store, map[string]string{
		"digest_time":   "09:00",
		"article_count": "30",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	value, ok := manager.Get("digest_time")
	if !ok || value != "10:00" {
		t.Fatalf("expected stored digest_time")
	}
	value, ok = manager.Get("article_count")
	if !ok || value != "30" {
		t.Fatalf("expected default article_count")
	}
	if store.set["article_count"] != "30" {
		t.Fatalf("expected default persisted")
	}
}

func TestManagerSetUpdatesStore(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	manager, err := NewManager(context.Background(), store, map[string]string{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := manager.Set(context.Background(), "digest_time", "11:00"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, ok := manager.Get("digest_time")
	if !ok || value != "11:00" {
		t.Fatalf("expected updated value")
	}
}
