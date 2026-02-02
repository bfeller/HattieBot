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

// Pipe-style tool call markers some models output in content (e.g. <|tool_calls_section_begin|> ... <|tool_call_begin|> ...).
var (
	pipeSectionRx = regexp.MustCompile(`(?s)<\|tool_calls_section_begin\|>.*?<\|tool_calls_section_end\|>\s*`)
	pipeCallRx    = regexp.MustCompile(`(?s)<\|tool_call_begin\|>.*?<\|tool_call_end\|>\s*`)
)

// StripInlineToolCallMarkers removes pipe-style and similar tool-call markup from content so we never send it to the user.
func StripInlineToolCallMarkers(content string) string {
	s := pipeSectionRx.ReplaceAllString(content, "")
	s = pipeCallRx.ReplaceAllString(s, "")
	// Also strip lone pipe markers that might remain
	s = regexp.MustCompile(`<\|tool_calls_section_begin\|>\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<\|tool_calls_section_end\|>\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<\|tool_call_begin\|>.*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<\|tool_call_argument_begin\|>.*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<\|tool_call_end\|>\s*`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func parsePipeStyleToolCalls(content string) ([]openrouter.ToolCall, string) {
	sectionStart := "<|tool_calls_section_begin|>"
	argBegin := "<|tool_call_argument_begin|>"
	callEnd := "<|tool_call_end|>"
	if !strings.Contains(content, sectionStart) && !strings.Contains(content, "<|tool_call_begin|>") {
		return nil, ""
	}
	var calls []openrouter.ToolCall
	cleaned := content
	idx := 0
	for {
		beginIdx := strings.Index(cleaned, "<|tool_call_begin|>")
		if beginIdx == -1 {
			break
		}
		afterBegin := cleaned[beginIdx+len("<|tool_call_begin|>"):]
		// Name is until whitespace or next pipe marker (e.g. "functions.read_file:0")
		nameEnd := 0
		for nameEnd < len(afterBegin) && afterBegin[nameEnd] != ' ' && afterBegin[nameEnd] != '\n' && afterBegin[nameEnd] != '\r' && !strings.HasPrefix(afterBegin[nameEnd:], "<|") {
			nameEnd++
		}
		nameRaw := strings.TrimSpace(afterBegin[:nameEnd])
		// Normalize: "functions.read_file:0" -> "read_file", "read_file:0" -> "read_file"
		name := nameRaw
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if i := strings.Index(name, ":"); i >= 0 {
			name = name[:i]
		}
		argStartIdx := strings.Index(afterBegin, argBegin)
		if argStartIdx == -1 {
			cleaned = cleaned[:beginIdx] + cleaned[beginIdx+len("<|tool_call_begin|>"):]
			continue
		}
		argsStart := argStartIdx + len(argBegin)
		afterArgs := afterBegin[argsStart:]
		endIdx := strings.Index(afterArgs, callEnd)
		if endIdx == -1 {
			break
		}
		argsStr := strings.TrimSpace(afterArgs[:endIdx])
		// Unescape if needed (e.g. \" -> ")
		argsStr = strings.ReplaceAll(argsStr, `\"`, `"`)
		argsJSON := argsStr
		if argsJSON != "" && argsJSON != "{}" {
			// Validate it's JSON
			var m map[string]interface{}
			if json.Unmarshal([]byte(argsJSON), &m) != nil {
				idx++
				cleaned = cleaned[:beginIdx] + cleaned[beginIdx+len("<|tool_call_begin|>")+argStartIdx+len(argBegin)+endIdx+len(callEnd):]
				continue
			}
		}
		calls = append(calls, openrouter.ToolCall{
			ID:   fmt.Sprintf("pipe-%d", idx),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: name, Arguments: argsJSON},
		})
		// Remove this entire call block from cleaned
		blockEnd := beginIdx + len("<|tool_call_begin|>") + argStartIdx + len(argBegin) + endIdx + len(callEnd)
		cleaned = cleaned[:beginIdx] + strings.TrimSpace(cleaned[blockEnd:])
		idx++
	}
	if len(calls) == 0 {
		return nil, ""
	}
	cleaned = strings.TrimSpace(pipeSectionRx.ReplaceAllString(cleaned, ""))
	return calls, cleaned
}

// ParseContentToolCalls extracts XML-like or pipe-style tool calls from model content.
// Returns synthetic ToolCalls and cleaned content (with markup removed so we don't re-send it).
// If no tool calls are found, returns nil, "" for cleaned (caller should keep original content).
func ParseContentToolCalls(content string) ([]openrouter.ToolCall, string) {
	// Try pipe-style first (e.g. <|tool_call_begin|> functions.read_file:0 <|tool_call_argument_begin|> {...})
	if calls, cleaned := parsePipeStyleToolCalls(content); len(calls) > 0 {
		return calls, cleaned
	}
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
