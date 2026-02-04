# HattieBot

Self-improving autonomous agent: OpenRouter LLMs, SQLite + sqlite-vec for memory, modular communication channels, skill installation, and context compaction. The CLI is a direct-access console; the agent core is independent so you can connect via Nextcloud Talk, webhooks, or other channels.

## Features

- **SOUL.md Identity**: Moltbot-inspired persona file with Core Truths, Boundaries, Vibe, and Continuity sections. Fully editable.
- **Onboarding Wizard**: First-run setup generates `SOUL.md` from bot name, audience, and purpose. Includes risk acceptance.
- **Skill System**: Agent can autonomously install tools via `go`, `brew`, or `npm` package managers.
- **Multi-Channel Communication**: Modular `Channel` interface supports terminal, Nextcloud Talk, webhooks, etc.
- **Dynamic System Prompts**: Runtime info (time, OS, workspace, agent name) + SOUL.md injected into every prompt.
- **Context Compaction**: Automatically summarizes old conversation turns to manage token limits.
- **Vector Memory**: Embed and recall memories using sqlite-vec for semantic search.

---

## Quick Start

### Prerequisites

- **Docker** (recommended) or Go 1.22+
- **OpenRouter API key** ([get one here](https://openrouter.ai/))

### Option 1: Docker (Recommended)

```bash
# 1. Clone the repo
git clone https://github.com/bfeller/hattiebot.git
cd hattiebot

# 2. Create data directory for persistence
mkdir -p data

# 3. Build and run (or use deploy image: see Deployment below)
docker compose -f docker-compose.demo.yml build
docker compose -f docker-compose.demo.yml run --rm hattiebot
```

On first run, you'll be prompted to configure:
1. **OpenRouter API Key**
2. **Model** (e.g. `moonshotai/kimi-k2.5`, `openai/gpt-4o`)
3. **Bot name, audience, and purpose** → generates `SOUL.md`
4. **Workspace directory** (default: `~/.hattiebot`)
5. **Risk acceptance** (required to proceed)
6. **Admin User ID** (default: `admin`)

Config, DB, and `SOUL.md` are persisted to `./data/`.

### Option 2: Local Build

```bash
# Build
go build -o hattiebot ./cmd/hattiebot

# Run
./hattiebot
```

Config is stored in `./.hattiebot/` or `~/.config/hattiebot/`.

---

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | Your OpenRouter API key |
| `HATTIEBOT_MODEL` | Model ID (e.g. `moonshotai/kimi-k2.5`) |
| `HATTIEBOT_CONFIG_DIR` | Config directory path (default: `~/.config/hattiebot`) |
| `HATTIEBOT_SEED_CONFIG` | Set to `1` to skip interactive first-boot |
| `HATTIEBOT_BOT_NAME` | Bot name for seeded config |
| `HATTIEBOT_AUDIENCE` | Target audience for seeded config |
| `HATTIEBOT_PURPOSE` | Bot purpose for seeded config |
| `HATTIEBOT_HEADLESS` | Set to `1` for single-turn stdin/stdout mode |
| `HATTIEBOT_API_PORT` | Port for HTTP API (default: none) |
| `HATTIEBOT_API_ONLY` | Set to `1` to run HTTP server only (no console) |
| `EMBEDDING_SERVICE_URL` | Base URL of embedding service (e.g. `http://embeddinggood:8000` or `https://embedding.bfs5.com`) |
| `EMBEDDING_SERVICE_API_KEY` | API key for embedding service (`x-api-key` header) |
| `HATTIEBOT_EMBEDDING_DIMENSION` | Embedding dimension: `128`, `256`, `512`, or `768` (default: `768`) |
| `HATTIEBOT_COMPOSE_MODE` | Set to `1` for env-only setup (no interactive first-boot); used with Nextcloud stack |
| `HATTIEBOT_DEFAULT_CHANNEL` | Default channel for proactive messages: `admin_term` or `nextcloud_talk` |
| `HATTIEBOT_HTTP_PORT` | HTTP port for webhooks (default: `8080`) |
| `NEXTCLOUD_URL` | Nextcloud base URL (e.g. `http://nextcloud` in compose) |
| `HATTIEBOT_WEBHOOK_SECRET` | Shared secret for HattieBridge webhook (must match HattieBridge app config) |
| `NEXTCLOUD_ADMIN_USER` | Nextcloud admin username; used as HattieBot admin (trusted source) in compose mode |
| `HATTIEBOT_ADMIN_USER_ID` | Override admin user ID (default: `NEXTCLOUD_ADMIN_USER` in compose mode) |

### Embedding service (vector memory)

Vector memory (`memorize` / `recall_memories`) can use a self-hosted [EmbeddingGood](https://github.com/bfeller/EmbeddingGood)-compatible API instead of OpenRouter embeddings. Set `EMBEDDING_SERVICE_URL` and `EMBEDDING_SERVICE_API_KEY`; the agent can also switch embedding providers at runtime via the `manage_embedding_provider` tool and `embedding_routing.json` in the config dir.

### Skip Interactive Setup (CI/Automation)

```bash
docker run --rm -it \
  -v $(pwd)/data:/data \
  -v $(pwd):/workspace \
  -e HATTIEBOT_CONFIG_DIR=/data \
  -e HATTIEBOT_SEED_CONFIG=1 \
  -e OPENROUTER_API_KEY=sk-... \
  -e HATTIEBOT_MODEL=moonshotai/kimi-k2.5 \
  -e HATTIEBOT_BOT_NAME=HattieBot \
  -e HATTIEBOT_AUDIENCE=developers \
  -e HATTIEBOT_PURPOSE="coding assistant" \
  -w /workspace \
  hattiebot
```

---

## Running Modes

### Interactive Console (Default)

```bash
# When building from source (demo compose)
docker compose -f docker-compose.demo.yml run --rm hattiebot

# When using pre-built deploy image
docker compose -f docker-compose.deploy.yml run --rm hattiebot
```

Type messages and press **Enter**. Press **Ctrl+C** to exit.

### Headless Mode (CI/Scripts)

```bash
echo "What files are in the current directory?" | docker run --rm -i \
  -v $(pwd)/data:/data \
  -e HATTIEBOT_CONFIG_DIR=/data \
  -e HATTIEBOT_HEADLESS=1 \
  hattiebot
```

### HTTP API Mode

```bash
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e HATTIEBOT_CONFIG_DIR=/data \
  -e HATTIEBOT_API_PORT=8080 \
  -e HATTIEBOT_API_ONLY=1 \
  -e OPENROUTER_API_KEY=sk-... \
  -e HATTIEBOT_SEED_CONFIG=1 \
  -e HATTIEBOT_BOT_NAME=HattieBot \
  -e HATTIEBOT_AUDIENCE=user \
  -e HATTIEBOT_PURPOSE=testing \
  hattiebot
```

**Endpoints:**
- `POST /chat` or `POST /v1/chat`: `{"message":"..."}` → `{"reply":"..."}`
- `GET /health`: returns `ok`

---

## Architecture

```
cmd/hattiebot/main.go    # Entry point, wiring
internal/
  agent/                  # Core loop, prompts, context
  channels/               # Communication (terminal, nextcloud_talk, webhook)
  config/                 # Runtime configuration
  gateway/                # Multi-channel message router
  memory/                 # Context compaction
  skills/                 # Package installation (go/brew/npm)
  store/                  # SQLite + sqlite-vec persistence
  tools/                  # Built-in tool definitions & execution
  tui/                    # First-boot interactive setup
```

---

## Built-in Tools

| Tool | Description |
|------|-------------|
| `run_terminal_cmd` | Execute shell commands |
| `read_file` / `write_file` | File I/O |
| `list_dir` | Directory listing |
| `memorize` / `recall_memories` | Vector memory |
| `manage_job` | Epic/task tracking |
| `manage_facts` | Key-value persistent facts |
| `manage_schedule` | Reminders and recurring tasks |
| `install_skill` | Install packages via go/brew/npm |
| `register_tool` / `execute_registered_tool` | Custom tool management |
| `manage_llm_provider` | Register LLM providers and set routing (e.g. Ollama, OpenRouter) |
| `manage_embedding_provider` | Register embedding providers and set default (e.g. EmbeddingGood) |

---

## Models

Default: `moonshotai/kimi-k2.5`

For full tool support, use a model with function calling (e.g. `openai/gpt-4o`). If the model doesn't support tools, HattieBot falls back to text-only responses.

---

## Development

```bash
# Run tests
go test ./...

# Build locally
go build -o hattiebot ./cmd/hattiebot

# Build Docker image
docker build -t hattiebot .
```

See [docs/TESTING_PROMPTS.md](docs/TESTING_PROMPTS.md) for end-to-end test scenarios.

---

## Deployment

To run from the pre-built image (no local build), use [docker-compose.deploy.yml](docker-compose.deploy.yml). The image is published to GitHub Container Registry as `ghcr.io/bfeller/hattiebot`.

```bash
# Create data dir and pull the image
mkdir -p data
docker compose -f docker-compose.deploy.yml pull
docker compose -f docker-compose.deploy.yml run --rm hattiebot
```

For a persistent private setup, copy the deploy file to `docker-compose.yml` (that file is gitignored) and use your own env file for secrets:

```bash
cp docker-compose.deploy.yml docker-compose.yml
# Edit .env with OPENROUTER_API_KEY, etc., then:
docker compose up -d
```

### Single-stack deploy (with embeddings)

[docker-compose.demo.yml](docker-compose.demo.yml) runs HattieBot and an [EmbeddingGood](https://github.com/bfeller/EmbeddingGood) embedding service in one stack. Build the EmbeddingGood image first, then run:

```bash
# Build EmbeddingGood image (gated model; set HF_TOKEN if needed)
git clone https://github.com/bfeller/EmbeddingGood.git ../EmbeddingGood
cd ../EmbeddingGood && docker build -t embeddinggood:latest . && cd -

# Run full stack (set OPENROUTER_API_KEY, EMBEDDING_SERVICE_API_KEY, optionally HF_TOKEN in .env)
mkdir -p data
docker compose -f docker-compose.demo.yml up
```

To use an existing embedding service (e.g. `https://embedding.bfs5.com`), use [docker-compose.override.example.yml](docker-compose.override.example.yml): copy to `docker-compose.override.yml`, set `EMBEDDING_SERVICE_URL` and `EMBEDDING_SERVICE_API_KEY` in `.env` (do not commit the API key).

### Nextcloud + HattieBot (single compose)

[docker-compose.nextcloud.yml](docker-compose.nextcloud.yml) runs **PostgreSQL**, **Nextcloud**, and **HattieBot** in one stack. Nextcloud auto-installs from env; the **HattieBridge** app (mounted from `apps/hattiebridge`) forwards Talk chat messages to HattieBot when the Hattie user is in the room. Hattie appears as a Nextcloud user (single identity), not a bot.

**Requirements:** Nextcloud 32. PostgreSQL 17.

1. Copy [.env.example](.env.example) to `.env` and set:
   - `POSTGRES_PASSWORD`, `NEXTCLOUD_ADMIN_USER`, `NEXTCLOUD_ADMIN_PASSWORD`, `HATTIEBOT_WEBHOOK_SECRET`
   - `NEXTCLOUD_TRUSTED_DOMAINS=localhost nextcloud` (include `nextcloud` so HattieBot’s bootstrap health check gets 200)
   - `OPENROUTER_API_KEY`, `HATTIEBOT_MODEL`, `HATTIEBOT_AUDIENCE`, `HATTIEBOT_PURPOSE`
2. Run:
   ```bash
   docker compose -f docker-compose.nextcloud.yml up -d
   ```
3. On first boot, Hattie creates a 1:1 Talk conversation with the admin and sends an intro. Open Nextcloud Talk to see it and start chatting.
4. **Trust:** The Nextcloud admin user (`NEXTCLOUD_ADMIN_USER`) is HattieBot’s trusted admin. New Nextcloud users who message the bot start as *restricted* until that admin approves them (e.g. via an approval tool or DB).

**First-time flow:** Postgres and Nextcloud start; Nextcloud auto-installs; the post-install hook enables Talk and HattieBridge; HattieBot (compose mode) waits for Nextcloud, provisions the Hattie user, writes config, then starts. HattieBridge forwards messages to `http://hattiebot:8080/webhook/talk`. HattieBot sends replies via the chat API as the Hattie user. Use `.env` or Docker secrets for all secrets; do not commit them.

**If Nextcloud doesn’t start:** Run `docker logs nextcloud` to see the entrypoint and post-install output (e.g. hook script errors). Port 80 must be free; use `ports: "8081:80"` in compose if 80 is in use.

**403 on webhook or send failures:** Ensure `HATTIEBOT_WEBHOOK_SECRET` in `.env` matches the value passed to the Nextcloud container (HattieBridge uses it to authenticate to HattieBot). For send failures, ensure the Hattie user was auto-provisioned (check logs for "Auto-provisioned Nextcloud user"). Rebuild after code changes: `docker compose -f docker-compose.nextcloud.yml build hattiebot && docker compose -f docker-compose.nextcloud.yml up -d`.

**500 Internal Server Error on intro message:** If HattieBot fails to send the first intro message (`statuscode: 996`), capture the PHP stack trace:

```bash
docker exec nextcloud tail -150 /var/www/html/data/nextcloud.log
docker exec nextcloud tail -50 /var/www/html/data/hattiebridge-debug.log
```

The `nextcloud.log` shows the exact PHP error and stack trace. The `hattiebridge-debug.log` shows whether the listener registered and ran (`[APP] HattieBridge listener registered successfully`, `[LISTENER] handle() ENTRY`). To isolate: set `HATTIEBRIDGE_DISABLED=1` in the Nextcloud container env and redeploy; if the intro succeeds, the issue is in HattieBridge.

---

## Documentation

| Doc | Description |
|-----|-------------|
| [docs/architecture.md](docs/architecture.md) | Runtime environment, agent loop, memory, LLM router |
| [docs/creating-tools.md](docs/creating-tools.md) | How to add and register custom tools |
| [docs/embedding-service.md](docs/embedding-service.md) | Vector memory and embeddings |
| [docs/roadmap.md](docs/roadmap.md) | Planned features |
| [docs/self_improvement_flows.md](docs/self_improvement_flows.md) | Sub-minds and self-improvement |
| [docs/tools.md](docs/tools.md) | Built-in tool reference |
| [docs/TESTING_PROMPTS.md](docs/TESTING_PROMPTS.md) | E2E test scenarios |

---

## Contributing

Contributions are welcome. Open an issue or PR on [GitHub](https://github.com/bfeller/hattiebot). Please keep secrets and local config out of commits (use `.env`, `.hattiebot/`, and the patterns in [.gitignore](.gitignore)).

---

## Updating Portainer Deployment

If you are running HattieBot via Portainer and have updated the source code (e.g., via `git pull` or manual edits), you must rebuild the container to apply changes:

1. **Terminal**:
   ```bash
   docker compose build hattiebot
   docker compose up -d hattiebot
   ```

2. **Portainer UI**:
   - Go to **Stacks** > **[Your Stack]** > **Editor**.
   - If using a Git repository, ensure "Repository Reference" is correct.
   - Click **Update the stack**.
   - **Crucial**: If building from source (`build: context: .`), Portainer might not rebuild the image automatically unless the configuration changed. You may need to manually trigger a build or use the "Re-pull image and redeploy" toggle if using an external image registry. For local source builds, the CLI method above is most reliable.

---

## License

Add a `LICENSE` file to the repository root for your chosen license (e.g. MIT, Apache-2.0).


