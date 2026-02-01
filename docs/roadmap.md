# HattieBot Roadmap

This document outlines potential future improvements for the system.

## Planned Improvements


### Response and output

- **Summarize long tool output:** When a tool returns large JSON or text (e.g. `list_dir` on a big tree, or a long file), instruct the agent to summarize or paginate instead of dumping the full raw result. Optionally enforce a max-char or “top N” hint in the prompt.
- **Structured tool-result display:** For tools that return JSON, encourage the agent to present key fields in readable form (e.g. “Found 12 files: …” with a short list) rather than raw JSON unless the user asked for raw data.
- **Progress for multi-step tasks:** For “create a tool and use it” flows, the agent could briefly acknowledge steps (“Building…”, “Registering…”, “Running…”) so the user knows the bot is working. This is prompt-only (no code change) or could be extended with a “status” field in the API later.

### Errors and robustness

- **Clearer error messages:** When a tool fails (e.g. file not found, command error), the prompt could ask the agent to: (1) state what it tried, (2) what went wrong in one sentence, and (3) a concrete suggestion (e.g. “Check that the path exists” or “Use a non-empty command”).
- **Graceful degradation:** If the model doesn’t support tools or returns invalid tool calls, ensure the agent explains that it can’t run tools and suggests alternatives (e.g. “I can’t run commands in your environment; here’s what you could run locally…”).

### API and programmatic use

- **Streaming replies:** For the HTTP API, support Server-Sent Events (SSE) or chunked streaming so clients can show text as it’s generated instead of waiting for the full response.
- **Reply metadata:** In API responses, optionally include metadata such as: which tools were called, success/failure, token usage, or a `sources` list (e.g. files read) so UIs can show “Based on README.md and docs/architecture.md”.
- **Idempotency / request IDs:** For retries and debugging, support a client-supplied idempotency key or request ID and return it in the response.

### Conversation and context

- **Turn summary or title:** For long conversations, optionally ask the model to emit a one-line summary or title per turn (or per N turns) that can be used in a sidebar or history list.
- **Explicit “I’m done” for tools:** Encourage the agent to end with a single clear conclusion (e.g. “Done. The echo tool returned: …”) so users know when the chain of tool calls is finished.

### TUI and CLI

- **Typing indicator:** In the TUI, show a simple “thinking” or “…” when waiting for the model, so the user knows the bot hasn’t frozen.
- **Copy / export:** Allow copying the last reply or full conversation to the clipboard or a file from the TUI.
- **Clear conversation:** A command or key to clear in-memory (and optionally persisted) history for a fresh start.

### Security and safety

- **Confirm destructive commands:** The prompt already says not to run destructive commands without confirmation; consider adding a small list of risky patterns (e.g. `rm -rf`, `format`, `DROP TABLE`) and instruct the agent to always ask for explicit “yes” before running them.
- **Sandbox reminder:** Periodically remind the agent in the prompt that it runs in a container and cannot access the host or the internet unless explicitly given a tool for it.

---

## How to use this list

- **Prompt-only:** Many items (summarize output, clearer errors, progress, “I’m done”, destructive confirmation) can be tried by editing `internal/agent/prompt.go` and optionally `system_purpose.txt`.
- **Code changes:** Streaming, reply metadata, request IDs, TUI typing indicator, copy/clear require changes in `internal/agent/`, `cmd/`, or API handlers.
- **Testing:** After prompt changes, run the scenarios in `docs/TESTING_PROMPTS.md` and the API test suite to ensure behavior and that raw XML tags no longer appear in replies.
