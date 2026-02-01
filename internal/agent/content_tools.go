package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hattiebot/hattiebot/internal/openrouter"
)

// Invoke/arg format some models use in content when they don't use API tool_calls:
// <function_calls>...</function_calls> or <invoke name="...">...</invoke>
// <arg name="...">value</arg>
var (
	invokeRx = regexp.MustCompile(`(?s)<invoke\s+name="([^"]+)"\s*>(.*?)</invoke>`)
	argRx    = regexp.MustCompile(`(?s)<arg\s+name="([^"]+)"\s*>(.*?)</arg>`)
)

// ParseContentToolCalls extracts XML-like tool calls from model content.
// Returns synthetic ToolCalls and cleaned content (with markup removed so we don't re-send it).
// If no tool calls are found, returns nil, "" for cleaned (caller should keep original content).
func ParseContentToolCalls(content string) ([]openrouter.ToolCall, string) {
	raw := content
	// Restrict to content inside <function_calls> if present, else whole content
	if start := strings.Index(raw, "<function_calls>"); start != -1 {
		if end := strings.Index(raw, "</function_calls>"); end != -1 && end > start {
			raw = raw[start+len("<function_calls>") : end]
		}
	}
	invokes := invokeRx.FindAllStringSubmatch(raw, -1)
	if len(invokes) == 0 {
		return nil, ""
	}
	var calls []openrouter.ToolCall
	cleaned := content
	for i, m := range invokes {
		name := strings.TrimSpace(m[1])
		inner := m[2]
		argsMap := make(map[string]string)
		for _, am := range argRx.FindAllStringSubmatch(inner, -1) {
			argsMap[strings.TrimSpace(am[1])] = strings.TrimSpace(am[2])
		}
		argsJSON := buildArgsJSON(name, argsMap)
		if argsJSON == "" {
			continue
		}
		calls = append(calls, openrouter.ToolCall{
			ID:   fmt.Sprintf("content-%d", i),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: name, Arguments: argsJSON},
		})
		// Remove this invoke block from cleaned so final reply has no markup
		cleaned = strings.Replace(cleaned, m[0], "", 1)
	}
	if len(calls) == 0 {
		return nil, ""
	}
	// Remove outer <function_calls>...</function_calls> from cleaned
	cleaned = regexp.MustCompile(`(?s)\s*<function_calls>\s*</function_calls>\s*`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`(?s)\s*<function_calls>.*?</function_calls>\s*`).ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(cleaned)
	return calls, cleaned
}

func buildArgsJSON(toolName string, args map[string]string) string {
	// Map model arg names to tool param names and normalize paths
	normalized := make(map[string]interface{})
	for k, v := range args {
		key := k
		if toolName == "read_file" && k == "file_path" {
			key = "path"
		}
		if (toolName == "read_file" || toolName == "list_dir") && (key == "path" || key == "file_path") {
			v = normalizeWorkspacePath(v)
		}
		normalized[key] = v
	}
	b, _ := json.Marshal(normalized)
	return string(b)
}

func normalizeWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	const prefix = "/workspace"
	if p == prefix || p == prefix+"/" {
		return "."
	}
	if strings.HasPrefix(p, prefix+"/") {
		return strings.TrimPrefix(p, prefix+"/")
	}
	return p
}
