package secrets

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/tools/nextcloud"
)

// SecretStore defines how to retrieve secrets.
type SecretStore interface {
	GetSecret(key string) (string, error)
}

// EnvSecretStore reads from environment variables.
type EnvSecretStore struct{}

func (s *EnvSecretStore) GetSecret(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("env var %s not set", key)
	}
	return val, nil
}

// NextcloudSecretStore reads from Nextcloud Passwords app with caching.
type NextcloudSecretStore struct {
	Config *config.Config
	Cache  map[string]cachedSecret
	Mu     sync.RWMutex
	TTL    time.Duration
}

type cachedSecret struct {
	Value     string
	ExpiresAt time.Time
}

func NewNextcloudSecretStore(cfg *config.Config) *NextcloudSecretStore {
	return &NextcloudSecretStore{
		Config: cfg,
		Cache:  make(map[string]cachedSecret),
		TTL:    5 * time.Minute,
	}
}

func (s *NextcloudSecretStore) GetSecret(key string) (string, error) {
	s.Mu.RLock()
	cached, ok := s.Cache[key]
	s.Mu.RUnlock()

	if ok && time.Now().Before(cached.ExpiresAt) {
		return cached.Value, nil
	}

	// Fetch from Nextcloud
	val, err := nextcloud.GetSecretValue(s.Config, key)
	if err != nil {
		return "", err // Fail closed
	}

	s.Mu.Lock()
	s.Cache[key] = cachedSecret{
		Value:     val,
		ExpiresAt: time.Now().Add(s.TTL),
	}
	s.Mu.Unlock()

	return val, nil
}

// MultiStore combines stores.
type MultiStore struct {
	stores map[string]SecretStore
}

func NewMultiStore() *MultiStore {
	return &MultiStore{
		stores: make(map[string]SecretStore),
	}
}

func (m *MultiStore) Register(source string, store SecretStore) {
	m.stores[source] = store
}

func (m *MultiStore) GetSecret(source, key string) (string, error) {
	s, ok := m.stores[source]
	if !ok {
		return "", fmt.Errorf("unknown secret source: %s", source)
	}
	return s.GetSecret(key)
}
