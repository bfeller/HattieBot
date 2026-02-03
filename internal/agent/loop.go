package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/memory"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
	"github.com/hattiebot/hattiebot/internal/tools"
)

// isProviderValidationError returns true for OpenRouter "Provider returned error" 400s due to
// provider-specific validation (e.g. reasoning_content, thinking) so we can retry with truncated context.
func isProviderValidationError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	if !strings.Contains(s, "Provider returned error") || !strings.Contains(s, "HTTP 400") {
		return false
	}
	return strings.Contains(s, "reasoning_content") ||
		strings.Contains(s, "thinking") ||
		strings.Contains(s, "invalid_request_error")
}

// isProviderOrAPIError returns true for transient provider/API errors that we should not expose raw to the user.
func isProviderOrAPIError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "provider returned error") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "503") ||
		strings.Contains(s, "502") ||
		strings.Contains(s, "504") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "temporarily unavailable")
}

// userFriendlyProviderError returns a message suitable for the user when a provider/API error occurs.
func userFriendlyProviderError(err error) string {
	_ = err // Could tailor message by error type in future
	return "I'm sorry, the AI provider temporarily returned an error. Please try again in a moment—your message was received and I'll process it when you resend."
}

// maxMessagesBeforeTruncationRetry is the message count above which we truncate and retry on provider validation error.
const maxMessagesBeforeTruncationRetry = 28

// Loop runs the agent: messages (system + full history + new user msg) -> OpenRouter with tools -> execute tool_calls -> repeat until no tool_calls -> save and return.
type Loop struct {
	Config          *config.Config
	DB              *store.DB
	Executor        core.ToolExecutor
	Client          core.LLMClient
	Context         core.ContextSelector
	Gateway         *gateway.Gateway
	Compactor       *memory.Compactor
	SubmindRegistry *SubmindRegistry
	LogStore        *store.LogStore
}

// SpawnSubmind creates and runs a sub-mind with the given mode and task.
// userID scopes the session; sessionID 0 = new session, non-zero = resume.
// Implements the core.SubmindSpawner interface.
func (l *Loop) SpawnSubmind(ctx context.Context, userID, mode, task string, sessionID int64) (core.SubMindResult, error) {
	if l.SubmindRegistry == nil {
		return core.SubMindResult{}, fmt.Errorf("submind registry not initialized")
	}

	cfg, ok := l.SubmindRegistry.Get(mode)
	if !ok {
		return core.SubMindResult{}, fmt.Errorf("unknown submind mode: %s", mode)
	}

	submind := &SubMind{
		Config:   cfg,
		Client:   l.Client,
		Executor: l.Executor,
		LogStore: l.LogStore,
	}

	// No persistence: backward compat when userID empty or no DB
	if userID == "" || l.DB == nil {
		return submind.Run(ctx, task)
	}

	if sessionID == 0 {
		id, err := l.DB.CreateSubmindSession(ctx, userID, mode, task, cfg.SystemPrompt)
		if err != nil {
			return core.SubMindResult{}, err
		}
		sessionID = id
	} else {
		_, err := l.DB.GetSubmindSession(ctx, sessionID, userID)
		if err != nil {
			return core.SubMindResult{}, fmt.Errorf("resume session: %w", err)
		}
	}
	return submind.RunWithSession(ctx, task, sessionID, userID, l.DB)
}

// RunOneTurn adds the user message, calls the model (with tool execution loop), saves messages, and returns the assistant reply.
// RunOneTurn adds the user message, calls the model (with tool execution loop), saves messages, and returns the assistant reply.
func (l *Loop) RunOneTurn(ctx context.Context, msg gateway.Message) (assistantContent string, err error) {
	// 1. Resolve User Identity
	// Gateway message doesn't carry Name yet, so we rely on ID.
	user, err := l.DB.GetOrCreateUser(ctx, msg.SenderID, "", msg.Channel)
	if err != nil {
		log.Printf("[AGENT] Failed to resolve user: %v", err)
		return "", fmt.Errorf("resolving user: %w", err)
	}

	// 1.2. Store room token for nextcloud_talk (needed for proactive reminders)
	if msg.Channel == "nextcloud_talk" && msg.ThreadID != "" {
		roomToken := msg.ThreadID
		if idx := strings.Index(roomToken, ":"); idx > 0 {
			roomToken = roomToken[:idx]
		}
		if roomToken != "" {
			meta := make(map[string]string)
			if user.Metadata != "" {
				_ = json.Unmarshal([]byte(user.Metadata), &meta)
			}
			meta["last_room_token"] = roomToken
			if b, err := json.Marshal(meta); err == nil {
				_ = l.DB.UpdateUserMetadata(ctx, user.ID, string(b))
			}
		}
	}

	// 1.5. Authorization & Trust Level Check
	// Auto-promote configured admin
	if l.Config.AdminUserID != "" && user.ID == l.Config.AdminUserID {
		if user.TrustLevel != "admin" {
			log.Printf("[AGENT] Auto-promoting admin user %s", user.ID)
			if err := l.DB.UpdateUserTrust(ctx, user.ID, "admin"); err == nil {
				user.TrustLevel = "admin"
			}
		}
	}

	// Enforce Trust Levels
	switch user.TrustLevel {
	case "blocked":
		log.Printf("[AGENT] Blocked user %s attempted access", user.ID)
		return "", nil // Silent drop or empty response
	
	case "restricted":
		// Notify Admin (if not self)
		if l.Config.AdminUserID != "" && user.ID != l.Config.AdminUserID {
			// Best effort notification
			go func() {
				// We don't know Admin's channel here easily unless we store "AdminChannel" or "AdminThread".
				// For now, relies on Gap 6 (Proactive Messaging) to route properly.
				// But we can log deeply or try to broadcast if we knew a channel.
				log.Printf("[AUTH] User %s (%s) requested access. Waiting for admin %s approval.", user.ID, user.Platform, l.Config.AdminUserID)
			}()
		}
		return "Access Restricted. Your account is pending approval by the administrator.", nil
	}

	// Inject user_id and trust_level into context for tools
	ctx = context.WithValue(ctx, "user_id", user.ID)
	ctx = context.WithValue(ctx, "user_trust", user.TrustLevel)

	// 2. Select History filtered by thread
	historyMessages, err := l.Context.SelectHistory(ctx, msg.ThreadID)
	if err != nil {
		return "", err
	}

	// Dynamic Compaction (Phase 6)
	if l.Compactor != nil {
		if compacted, changed, cErr := l.Compactor.Compact(ctx, historyMessages); cErr == nil && changed {
			log.Printf("[AGENT] Compacted history from %d to %d messages", len(historyMessages), len(compacted))
			historyMessages = compacted
		} else if cErr != nil {
			log.Printf("[AGENT] Compaction failed: %v", cErr)
		}
	}

	systemPrompt, err := BuildSystemPrompt(ctx, l.DB, l.Config, user.ID)
	if err != nil {
		return "", err
	}

	// 3. Inject User Context into System Prompt
	// Fetch recent facts/memories
	facts, _ := l.DB.SearchFacts(ctx, user.ID, "") // Ignore error, just empty list
	
	userContext := fmt.Sprintf("\n\nUser Context:\n- ID: %s\n- Platform: %s", user.ID, user.Platform)
	if user.Name != "" && user.Name != "User "+user.ID {
		userContext += fmt.Sprintf("\n- Name: %s", user.Name)
	}
	if len(facts) > 0 {
		userContext += "\n- Memories/Facts:"
		for _, f := range facts {
			userContext += fmt.Sprintf("\n  * %s: %s", f.Key, f.Value)
		}
	}
	
	// Inject Pending/Blocked Items (Gap 6)
	// Fetch blocked jobs
	blockedJobs, _ := l.DB.ListJobs(ctx, user.ID, "blocked")
	// Fetch overdue plans (simple active filter for now)
	activePlans, _ := l.DB.ListPlans(ctx, user.ID, "active") // Filter in loop if needed
	
	if len(blockedJobs) > 0 || len(activePlans) > 0 {
		userContext += "\n\n[PENDING ITEMS - ASK USER TO RESOLVE]:"
		for _, j := range blockedJobs {
			userContext += fmt.Sprintf("\n- Job #%d: %s (BLOCKED: %s) [TIP: Use snooze action if user needs time]", j.ID, j.Title, j.BlockedReason)
		}
		now := time.Now()
		for _, p := range activePlans {
			if p.NextRunAt != nil && p.NextRunAt.Before(now) {
				userContext += fmt.Sprintf("\n- Plan #%d: %s (Overdue since %s)", p.ID, p.Description, p.NextRunAt)
			}
		}
	}

	if msg.Autonomous {
		userContext += "\n\n[AUTONOMOUS TASK]: You are running an autonomous scheduled task. Complete it without requiring user input. Only call notify_user if something needs the user's attention (errors, anomalies, important findings). If the task completes successfully with nothing notable, finish without calling notify_user."
	}

	systemPrompt += userContext

	// Build OpenRouter messages: system + history + new user
	messages := []openrouter.Message{{Role: "system", Content: systemPrompt}}
	messages = append(messages, historyMessages...)
	messages = append(messages, openrouter.Message{Role: "user", Content: msg.Content})

	// Save user message
	_, err = l.DB.InsertMessage(ctx, "user", msg.Content, "", msg.SenderID, msg.Channel, msg.ThreadID, "", "", "")
	if err != nil {
		return "", err
	}

	toolDefs := tools.BuiltinToolDefs()
    
    // Empty-response retries: count consecutive empty model replies; reset after any successful tool execution.
    const maxEmptyRetries = 2
    emptyRetries := 0
    // Safety cap for total turns per user message (avoid runaway loops).
    const maxTotalTurns = 50
    totalTurns := 0
    // One retry with truncated context on OpenRouter "Provider returned error" (e.g. reasoning_content/thinking).
    truncationRetryDone := false
    // Track tool rounds for status-update hint (after 2+ rounds with no user feedback).
    toolRounds := 0
    statusUpdateHintSent := false

    var content string
    var toolCalls []openrouter.ToolCall

TurnLoop:
	for {
        totalTurns++
        if totalTurns > maxTotalTurns {
            log.Printf("[AGENT] Max turns (%d) reached for this request.", maxTotalTurns)
            content = "I hit the turn limit for this request. Please try a shorter or simpler ask, or break it into separate messages."
            break TurnLoop
        }
        useTools := true
        // Inner Tool Loop
        for {
            if useTools {
                // After 2+ tool rounds with no user feedback, prompt the model to include a status update.
                if toolRounds >= 2 && !statusUpdateHintSent && l.Gateway != nil {
                    statusUpdateHintSent = true
                    messages = append(messages, openrouter.Message{
                        Role:    "system",
                        Content: "The user has received no feedback yet. Include a brief status update (1-2 sentences) in your next response along with any tool calls, so the user knows you're working.",
                    })
                }
                var err error
                content, toolCalls, err = l.Client.ChatCompletionWithTools(ctx, messages, toolDefs)
                log.Printf("[AGENT] ChatCompletionWithTools returned: content_len=%d, toolCalls=%d, err=%v", len(content), len(toolCalls), err)
                if err != nil {
                    // Only fallback to non-tool mode if the error indicates tools aren't supported.
                    // Do NOT treat "Invalid tool call" / "invalid JSON" (bad request) as unsupported—provider does support tools.
                    errStr := err.Error()
                    isBadRequest := strings.Contains(errStr, "Invalid tool call") || strings.Contains(errStr, "invalid JSON")
                    isToolNotSupported := !isBadRequest && (
                        strings.Contains(errStr, "does not support tools") ||
                            strings.Contains(errStr, "tool_calls") ||
                            strings.Contains(errStr, "function_call"))
                    if isToolNotSupported {
                        log.Printf("[AGENT] Tool fallback triggered (model doesn't support tools): %v", err)
                        useTools = false
                        continue
                    }
                    // Retry once with truncated context on provider validation errors (e.g. reasoning_content/thinking).
                    if isProviderValidationError(err) && !truncationRetryDone && len(messages) > maxMessagesBeforeTruncationRetry {
                        keep := maxMessagesBeforeTruncationRetry - 1 // keep system + last (keep) messages
                        if keep < 2 {
                            keep = 2
                        }
                        newLen := 1 + keep
                        if len(messages) > newLen {
                            log.Printf("[AGENT] Provider validation error (e.g. reasoning_content); truncating to last %d messages and retrying", keep)
                            messages = append(messages[:1], messages[len(messages)-keep:]...)
                            truncationRetryDone = true
                            continue
                        }
                    }
                    // Transient or other error—return user-friendly message for provider/API errors
                    log.Printf("[AGENT] API error (not tool-related): %v", err)
                    if isProviderOrAPIError(err) {
                        return userFriendlyProviderError(err), nil
                    }
                    return "", err
                }
                
                // Content-based tool parsing (e.g. XML)
                if len(toolCalls) == 0 {
                    parsed, cleaned := ParseContentToolCalls(content)
                    log.Printf("[AGENT] ParseContentToolCalls: found %d tool calls in content", len(parsed))
                    if len(parsed) > 0 {
                        toolCalls = parsed
                        content = cleaned
                        if strings.TrimSpace(content) == "" {
                            content = ""
                        }
                    }
                }

                // Send intermediate status update when model returns both content and tool calls
                if strings.TrimSpace(content) != "" && len(toolCalls) > 0 && l.Gateway != nil {
                    statusContent := StripInlineToolCallMarkers(content)
                    if strings.TrimSpace(statusContent) != "" {
                        l.Gateway.RouteReply(msg, statusContent)
                        log.Printf("[AGENT] Sent intermediate status update to user: %q", statusContent)
                    }
                }

                if len(toolCalls) == 0 {
                    log.Printf("[AGENT] No tool calls, breaking inner loop")
                    break
                }
                var toolNames []string
                for _, tc := range toolCalls {
                    toolNames = append(toolNames, tc.Function.Name)
                }
                log.Printf("[AGENT] Executing %d tool calls: %s", len(toolCalls), strings.Join(toolNames, ", "))
                toolRounds++

                // Append assistant message with tool_calls
                assistantMsg := openrouter.Message{
                    Role:      "assistant",
                    Content:   content,
                    ToolCalls: toolCalls,
                }
                messages = append(messages, assistantMsg)

                // Save assistant message to DB
                toolCallsJSON, _ := json.Marshal(toolCalls)
                l.DB.InsertMessage(ctx, "assistant", content, l.Config.Model, "hattiebot", msg.Channel, msg.ThreadID, string(toolCallsJSON), "", "")

                for _, tc := range toolCalls {
                    args := tc.Function.Arguments
                    result, execErr := l.Executor.Execute(ctx, tc.Function.Name, args)
                    if execErr != nil {
                        b, _ := json.Marshal(map[string]string{"error": execErr.Error()})
                        result = string(b)
                    }
                    
                    // Append to memory
                    messages = append(messages, openrouter.Message{
                        Role:       "tool",
                        Content:    result,
                        ToolCallID: tc.ID,
                    })

                    // Save to DB
                    l.DB.InsertMessage(ctx, "tool", result, "", "system", msg.Channel, msg.ThreadID, "", "", tc.ID)
                }
                // Inject any new user messages that arrived while we were working (e.g. "stop").
                // The model will see them on the next LLM call and can respond accordingly.
                if l.Gateway != nil {
                    tk := gateway.ThreadKey(msg)
                    pending := l.Gateway.GetPendingAndClear(tk)
                    if len(pending) > 0 {
                        messages = append(messages, openrouter.Message{
                            Role:    "system",
                            Content: "The user sent a new message while you were working. Read it and respond—if they ask you to stop or change direction, acknowledge and do so.",
                        })
                        for _, p := range pending {
                            messages = append(messages, openrouter.Message{Role: "user", Content: p.Content})
                            _, _ = l.DB.InsertMessage(ctx, "user", p.Content, "", p.SenderID, msg.Channel, msg.ThreadID, "", "", "")
                        }
                    }
                }
                // Reset empty-response counter after successful tool execution so we don't give up mid-request.
                emptyRetries = 0
                continue
            }
            
            // No tools: single chat completion
            var simpleMessages []openrouter.Message
            for _, m := range messages {
                if m.Role == "tool" { continue }
                if m.Role == "assistant" && strings.TrimSpace(m.Content) == "" { continue }
                simpleMessages = append(simpleMessages, openrouter.Message{Role: m.Role, Content: m.Content})
            }
            var err error
            content, err = l.Client.ChatCompletion(ctx, simpleMessages)
            if err != nil {
                log.Printf("[AGENT] ChatCompletion error: %v", err)
                if isProviderOrAPIError(err) {
                    return userFriendlyProviderError(err), nil
                }
                return "", err
            }
            break
        } // End Inner Tool Loop

        // Validate Content & Self-Correct (only count consecutive empty responses; counter was reset after tool execution)
        if strings.TrimSpace(content) == "" || content == "(No text in model response; try rephrasing or a different model.)" {
            if emptyRetries < maxEmptyRetries {
                log.Printf("[AGENT] Empty response detected. Triggering self-correction (consecutive empty %d/%d)...", emptyRetries+1, maxEmptyRetries)
                retryMsg := openrouter.Message{
                    Role: "system", 
                    Content: "You returned an empty response. Please provide a text summary of the tool results or an answer to the user. Do not output empty text. If you need to run more tools, do so.",
                }
                messages = append(messages, retryMsg)
                emptyRetries++
                continue TurnLoop
            }
            // Fallback after consecutive empty retries
            content = "(No text in model response; try rephrasing or a different model.)"
            log.Printf("[AGENT] Self-correction failed after %d consecutive empty responses.", emptyRetries)
        }
        break TurnLoop
    }

	// Strip inline tool-call markers so we never send raw tool syntax to the user
	content = StripInlineToolCallMarkers(content)

	// Save assistant message
	toolCallsJSON := ""
	toolResultsJSON := ""
	_, err = l.DB.InsertMessage(ctx, "assistant", content, l.Config.Model, "hattiebot", msg.Channel, msg.ThreadID, toolCallsJSON, toolResultsJSON, "")
	if err != nil {
		return "", err
	}
	return content, nil
}
