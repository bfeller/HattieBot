package tools

import (
	"testing"
)

func TestValidateToolOutput(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		exitCode int
		want     bool
	}{
		{"valid json object", `{"result":"ok"}`, 0, true},
		{"valid json with output key", `{"output":"x"}`, 0, true},
		{"valid json with error key", `{"error":""}`, 0, true},
		{"invalid json", "not json", 0, false},
		{"empty stdout", "", 0, false},
		{"non-zero exit", `{"result":"ok"}`, 1, false},
		{"truncated json", `{"result":`, 0, false},
		{"whitespace then json", "  \n{\"a\":1}  ", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateToolOutput(tt.stdout, tt.exitCode)
			if got != tt.want {
				t.Errorf("ValidateToolOutput(%q, %d) = %v, want %v", tt.stdout, tt.exitCode, got, tt.want)
			}
		})
	}
}
