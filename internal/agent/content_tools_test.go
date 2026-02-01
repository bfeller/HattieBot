package agent

import (
	"encoding/json"
	"testing"
)

func TestParseContentToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantCalls int
		wantName string
		wantPath string
	}{
		{
			name: "list_dir with function_calls wrapper",
			content: `<function_calls>
<invoke name="list_dir">
<arg name="path">/workspace</arg>
</invoke>
</function_calls>`,
			wantCalls: 1,
			wantName:  "list_dir",
			wantPath:  ".",
		},
		{
			name: "read_file with file_path",
			content: `<invoke name="read_file">
<arg name="file_path">/workspace/docs/embedding-service.md</arg>
</invoke>`,
			wantCalls: 1,
			wantName:  "read_file",
			wantPath:  "docs/embedding-service.md",
		},
		{
			name:     "no tool calls",
			content:  "Hello, here is the list.",
			wantCalls: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, cleaned := ParseContentToolCalls(tt.content)
			if len(calls) != tt.wantCalls {
				t.Errorf("ParseContentToolCalls() got %d calls, want %d", len(calls), tt.wantCalls)
				return
			}
			if tt.wantCalls == 0 {
				return
			}
			if calls[0].Function.Name != tt.wantName {
				t.Errorf("first call name = %q, want %q", calls[0].Function.Name, tt.wantName)
			}
			if tt.wantPath != "" {
				var args map[string]string
				if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
					t.Fatal(err)
				}
				if args["path"] != tt.wantPath {
					t.Errorf("path = %q, want %q", args["path"], tt.wantPath)
				}
			}
			if tt.wantCalls > 0 && cleaned != "" && len(cleaned) >= len(tt.content) && cleaned == tt.content {
				t.Errorf("cleaned should remove or shorten markup, got len %d", len(cleaned))
			}
		})
	}
}
