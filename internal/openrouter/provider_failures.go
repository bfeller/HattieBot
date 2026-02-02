package openrouter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const providerFailuresFilename = "openrouter_provider_failures.json"

// DefaultProviderCooldown is how long a provider is excluded after a failure before re-entering rotation.
const DefaultProviderCooldown = 10 * time.Minute

// providerFailuresFile is the on-disk shape: key = "model|provider_slug", value = blocked_until RFC3339.
type providerFailuresFile map[string]string

// LoadBlockedProviders returns provider slugs that are still blocked for the given model (blocked_until > now).
// Prunes expired entries when reading. Returns nil slice if configDir is empty or file is missing.
func LoadBlockedProviders(configDir, model string) ([]string, error) {
	if configDir == "" || model == "" {
		return nil, nil
	}
	p := filepath.Join(configDir, providerFailuresFilename)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var file providerFailuresFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	prefix := model + "|"
	var blocked []string
	pruned := make(providerFailuresFile)
	for key, untilStr := range file {
		t, err := time.Parse(time.RFC3339, untilStr)
		if err != nil {
			continue
		}
		if t.Before(now) {
			continue
		}
		pruned[key] = untilStr
		if strings.HasPrefix(key, prefix) {
			slug := strings.TrimPrefix(key, prefix)
			if slug != "" {
				blocked = append(blocked, slug)
			}
		}
	}
	if len(pruned) != len(file) {
		_ = saveProviderFailures(configDir, pruned)
	}
	return blocked, nil
}

// RecordProviderFailure records a provider failure for the model; the provider will be ignored until blockedUntil.
// If the provider is already recorded with a later blocked_until, that is kept.
func RecordProviderFailure(configDir, model, providerSlug string, blockedUntil time.Time) error {
	if configDir == "" || model == "" || providerSlug == "" {
		return nil
	}
	key := model + "|" + providerSlug
	untilStr := blockedUntil.UTC().Format(time.RFC3339)

	p := filepath.Join(configDir, providerFailuresFilename)
	var file providerFailuresFile
	data, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &file)
	}
	if file == nil {
		file = make(providerFailuresFile)
	}
	if existing, ok := file[key]; ok {
		t, _ := time.Parse(time.RFC3339, existing)
		if !blockedUntil.After(t) {
			return nil
		}
	}
	file[key] = untilStr
	return saveProviderFailures(configDir, file)
}

func saveProviderFailures(configDir string, file providerFailuresFile) error {
	if configDir == "" {
		return nil
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	p := filepath.Join(configDir, providerFailuresFilename)
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
