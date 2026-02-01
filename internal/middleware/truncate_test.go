package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockExecutor struct {
	result string
	err    error
}

func (m *mockExecutor) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

func TestTruncatingExecutor_NoTruncationWhenMaxZero(t *testing.T) {
	long := strings.Repeat("x", 1000)
	inner := &mockExecutor{result: long}
	wrap := NewTruncatingExecutor(inner, 0)
	got, err := wrap.Execute(context.Background(), "read_file", `{"path":"big.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != long {
		t.Errorf("maxRunes 0: expected full output, got len %d", len(got))
	}
}

func TestTruncatingExecutor_TruncatesWhenMaxSet(t *testing.T) {
	long := strings.Repeat("x", 500)
	inner := &mockExecutor{result: long}
	wrap := NewTruncatingExecutor(inner, 200)
	got, err := wrap.Execute(context.Background(), "read_file", `{"path":"big.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "...[output truncated, total 500 runes]") {
		t.Errorf("expected truncation suffix: %q", got[len(got)-80:])
	}
	if len(got) >= len(long) {
		t.Errorf("expected truncated result, got len %d", len(got))
	}
}

var errToolFailed = errors.New("tool failed")

func TestTruncatingExecutor_PassesErrorThrough(t *testing.T) {
	inner := &mockExecutor{err: errToolFailed}
	wrap := NewTruncatingExecutor(inner, 100)
	_, err := wrap.Execute(context.Background(), "unknown", "{}")
	if err != errToolFailed {
		t.Errorf("expected error passthrough, got %v", err)
	}
}
