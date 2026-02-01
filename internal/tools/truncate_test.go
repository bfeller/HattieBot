package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateToolOutput_NoTruncation(t *testing.T) {
	s := "short"
	if got := TruncateToolOutput(s, 0); got != s {
		t.Errorf("maxRunes 0: got %q", got)
	}
	if got := TruncateToolOutput(s, -1); got != s {
		t.Errorf("maxRunes -1: got %q", got)
	}
	if got := TruncateToolOutput(s, 100); got != s {
		t.Errorf("short string: got %q", got)
	}
}

func TestTruncateToolOutput_Truncates(t *testing.T) {
	// Build string longer than maxRunes
	long := strings.Repeat("a", 500)
	maxRunes := 200
	got := TruncateToolOutput(long, maxRunes)
	if utf8.RuneCountInString(got) > maxRunes+100 {
		t.Errorf("truncated length too large: %d runes", utf8.RuneCountInString(got))
	}
	if !strings.Contains(got, "...[output truncated, total 500 runes]") {
		t.Errorf("missing truncation suffix: %q", got)
	}
	prefix := strings.Repeat("a", 200-suffixReserve)
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("prefix not preserved: got %q", got[:min(50, len(got))])
	}
}

func TestTruncateToolOutput_Unicode(t *testing.T) {
	// Multi-byte runes: 1 rune = 3 bytes in UTF-8
	s := strings.Repeat("世", 100) // 100 runes
	got := TruncateToolOutput(s, 50)
	if !strings.Contains(got, "...[output truncated, total 100 runes]") {
		t.Errorf("unicode: missing suffix: %q", got)
	}
	r := []rune(got)
	if len(r) > 80 {
		t.Errorf("unicode: result too long: %d runes", len(r))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
