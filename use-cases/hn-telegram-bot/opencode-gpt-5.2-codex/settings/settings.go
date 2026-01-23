package settings

import (
	"context"
	"errors"
	"sync"
)

type Store interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

type Manager struct {
	mu     sync.RWMutex
	values map[string]string
	store  Store
}

func NewManager(ctx context.Context, store Store, defaults map[string]string) (*Manager, error) {
	if store == nil {
		return nil, errors.New("store required")
	}
	m := &Manager{store: store, values: map[string]string{}}
	for key, value := range defaults {
		stored, ok, err := store.GetSetting(ctx, key)
		if err != nil {
			return nil, err
		}
		if ok {
			m.values[key] = stored
			continue
		}
		if err := store.SetSetting(ctx, key, value); err != nil {
			return nil, err
		}
		m.values[key] = value
	}
	return m, nil
}

func (m *Manager) Get(key string) (string, bool) {
	if m == nil {
		return "", false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.values[key]
	return value, ok
}

func (m *Manager) Set(ctx context.Context, key, value string) error {
	if m == nil {
		return errors.New("manager not initialized")
	}
	if err := m.store.SetSetting(ctx, key, value); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}
