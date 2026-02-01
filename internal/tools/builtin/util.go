package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Helper to get user ID from context
func getUserID(ctx context.Context) (string, error) {
	uid := ctx.Value("user_id")
	if uid == nil {
		return "", fmt.Errorf("user context required")
	}
	return uid.(string), nil
}

func ErrJSON(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}

// parseDuration parses human-readable durations like "1h", "2d", "30m"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	// Check for day suffix (not supported by time.ParseDuration)
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	// Standard Go duration parsing for h, m, s
	return time.ParseDuration(s)
}
