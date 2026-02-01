# HattieBot End-to-End Testing Prompts

Use these prompts to manually test all features. Run `./scripts/run-docker-interactive.sh`, then type or paste each prompt and press **Enter** to send.

For API-based testing (no typing), run with `HATTIEBOT_API_PORT=8080` and `HATTIEBOT_API_ONLY=1` plus seed env vars (see README), then `curl -X POST http://localhost:8080/chat -H "Content-Type: application/json" -d '{"message":"..."}'`.

### Curl commands (rebuild + API + extendable systems)

```bash
# Rebuild image
docker compose build

# Start API container (ensure OPENROUTER_API_KEY and optionally HATTIEBOT_MODEL are set)
docker rm -f hattiebot-api 2>/dev/null
docker run -d -p 8080:8080 \
  -v "$(pwd)/data:/data" -v "$(pwd):/workspace" \
  -e HATTIEBOT_CONFIG_DIR=/data -e HATTIEBOT_API_PORT=8080 -e HATTIEBOT_API_ONLY=1 \
  -e HATTIEBOT_SEED_CONFIG=1 -e HATTIEBOT_BOT_NAME=HattieBot -e HATTIEBOT_AUDIENCE=user -e HATTIEBOT_PURPOSE=testing \
  -e OPENROUTER_API_KEY="${OPENROUTER_API_KEY}" -e HATTIEBOT_MODEL="${HATTIEBOT_MODEL:-moonshotai/kimi-k2.5}" \
  -w /workspace --name hattiebot-api hattiebot-hattiebot

# Health
curl -s http://localhost:8080/health

# Extendable systems: agent should create a URL-fetch tool then use it (or use tools it has)
curl -s -X POST http://localhost:8080/chat -H "Content-Type: application/json" \
  -d '{"message":"I need to read https://raw.githubusercontent.com/bfeller/EmbeddingGood/main/README.md but I do not have a tool that fetches URLs. Use your extendable systems: create a tool that fetches URL content, build and register it, then use it to get that page and tell me in one sentence what POST /embed expects."}'

# Simple tool test (list_dir)
curl -s -X POST http://localhost:8080/chat -H "Content-Type: application/json" \
  -d '{"message":"Use list_dir to list the top-level files in the workspace. Reply with a short list."}'
```

**Note:** With `moonshotai/kimi-k2.5`, the model may return tool-call markup in its *content* (e.g. `<function_calls>`) instead of using the API's `tool_calls` field. The core only executes tools when the API returns structured `tool_calls`; for full tool use and extendable-system behavior, use a model that supports OpenRouter tool calling (e.g. `openai/gpt-4o`).

---

**Temporary credentials for testing only — remove or rotate before committing if the repo is public.**

openrouter temp key: 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
openrouter model: moonshotai/kimi-k2.5
moonshotai/kimi-k2-thinking

## 1. Basic chat and identity

**Context:** On first boot, the system asks *you* to describe who the bot is and what it’s for (that text is cleaned and saved to `system_purpose.txt`). This test checks that the bot later reflects what you provided.

- **Prompt:** `What is your name and what are you for?`
- **Expect:** Reply includes the bot name and purpose from `system_purpose.txt` (i.e. whatever you entered during first-boot setup, e.g. HattieBot, self-improving agent).

---

## 2. File tools — list directory

- **Prompt:** `List the files and folders in the workspace root. Use the list_dir tool.`
- **Expect:** Agent uses `list_dir` (if the model supports tools) and replies with entries (e.g. `bin`, `cmd`, `data`, `docs`, `internal`, `tools`, `.gitignore`, `Dockerfile`, `README.md`, etc.).

---

## 3. File tools — read file

- **Prompt:** `Read the first 20 lines of README.md and summarize what the project is.`
- **Expect:** Agent uses `read_file` with path `README.md` and returns a short summary of HattieBot.

---

## 4. Terminal command

- **Prompt:** `Run the command: echo "Hello from terminal" and show me the output.`
- **Expect:** Agent uses `run_terminal_cmd` with `command: echo "Hello from terminal"` and reports stdout (and exit code 0).

---

## 5. Architecture docs

- **Prompt:** `What does the architecture doc say about where the tool registry lives and how to create new tools?`
- **Expect:** Agent uses `read_architecture` and answers using `docs/architecture.md` / `docs/creating-tools.md` (tool registry in SQLite, Go tools, stdin/stdout JSON, `go build`, register in `tools_registry`).

---

## 6. Registered tool (execute_registered_tool)

**Prerequisite:** Register the echo tool once (from project root):

```bash
docker run --rm \
  -v "$(pwd)/data:/data" -v "$(pwd):/workspace" -e HATTIEBOT_CONFIG_DIR=/data \
  --entrypoint /usr/local/bin/register-tool hattiebot \
  echo /workspace/bin/echo "Echoes back the message"
```

Ensure `bin/echo` exists: `go build -o bin/echo ./tools/echo`

- **Prompt:** `Use the execute_registered_tool to run the tool named "echo" with message "testing 123". Then tell me what the tool returned.`
- **Expect:** Agent calls `execute_registered_tool` with `name: "echo"` and `args: {"message":"testing 123"}`, and reports the tool output (e.g. `reply: "testing 123"`).

---

## 7. Conversation history (multi-turn)

- **First prompt:** `Remember this number: 42.`
- **Expect:** Short acknowledgment.
- **Second prompt:** `What number did I ask you to remember?`
- **Expect:** Agent says 42 (full history is loaded each run, so it should have context).

---

## 8. Autohand Code CLI (optional)

- **Prompt:** `Use the autohand_cli to add a comment at the top of tools/echo/main.go saying "Echo tool for testing."`
- **Expect:** Agent uses `autohand_cli` with an instruction; Autohand may prompt for login or run. If OpenRouter is configured for Autohand in the container, the file may be edited. (Can skip if Autohand is not logged in.)

---

## 9. Error handling — unknown file

- **Prompt:** `Read the file this_file_does_not_exist.txt and tell me what happened.`
- **Expect:** Agent uses `read_file`, gets an error (e.g. no such file), and reports the error in natural language.

---

## 10. Error handling — empty or invalid tool use

- **Prompt:** `Run a terminal command with an empty command and tell me the result.`
- **Expect:** Agent either avoids empty command or uses `run_terminal_cmd` with empty command and reports the error (e.g. "command is required").

---

## Quick checklist

| # | Feature              | Prompt focus                    | Pass? |
|---|----------------------|----------------------------------|------|
| 1 | Identity             | Name and purpose                 | [ ]  |
| 2 | list_dir             | List workspace root              | [ ]  |
| 3 | read_file            | Read README.md                   | [ ]  |
| 4 | run_terminal_cmd     | echo "Hello from terminal"       | [ ]  |
| 5 | read_architecture   | Tool registry + creating tools  | [ ]  |
| 6 | execute_registered_tool | echo tool with message        | [ ]  |
| 7 | Conversation history| Remember 42, then ask            | [ ]  |
| 8 | autohand_cli        | (Optional) Edit file             | [ ]  |
| 9 | Error — missing file| Read nonexistent file            | [ ]  |
|10 | Error — empty cmd   | Empty terminal command           | [ ]  |

---

## Notes

- **Tool calling:** Hattie uses `moonshotai/kimi-k2.5`. If the model or provider does not support tools, the agent falls back to text-only; prompts 2–6 and 8–10 may yield text answers instead of tool use. For full tool use, use a model that supports tool calling (e.g. `openai/gpt-4o`).
- **Enter:** Sends your message.
- **Ctrl+C:** Exits the application.
- **First boot (testing script):** The script wipes `./data` (config, system_purpose.txt, DB). You're prompted for: OpenRouter API key, model, then the bot's name, who it's talking to, and its purpose. That is cleaned by the LLM and saved to `system_purpose.txt`. Then chat starts.


I want to talk to you via zulip. i have it hosted at chat.1bf.ca and your api key is kuh9vqy22EjIp4pcwG1xHKELPke3wH9n and your email is hattie-bot@chat.1bf.ca can you set yourself up to automatically reply to all messages in the #Hattie channel I have in there.