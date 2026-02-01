# HattieBot

Self-improving autonomous agent: OpenRouter LLMs, SQLite + sqlite-vec for memory, modular communication channels, skill installation, and context compaction. The CLI is a direct-access console; the agent core is independent so you can connect via Zulip, webhooks, or other channels.

## Features

- **SOUL.md Identity**: Moltbot-inspired persona file with Core Truths, Boundaries, Vibe, and Continuity sections. Fully editable.
- **Onboarding Wizard**: First-run setup generates `SOUL.md` from bot name, audience, and purpose. Includes risk acceptance.
- **Skill System**: Agent can autonomously install tools via `go`, `brew`, or `npm` package managers.
- **Multi-Channel Communication**: Modular `Channel` interface supports terminal, Zulip, webhooks, etc.
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
3. **Zulip integration** (optional)
4. **Bot name, audience, and purpose** → generates `SOUL.md`
5. **Workspace directory** (default: `~/.hattiebot`)
6. **Risk acceptance** (required to proceed)

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
| `ZULIP_URL` | Zulip site URL (e.g. `https://chat.zulip.org`) |
| `ZULIP_EMAIL` | Zulip bot email |
| `ZULIP_KEY` | Zulip bot API key |
| `EMBEDDING_SERVICE_URL` | Base URL of embedding service (e.g. `http://embeddinggood:8000` or `https://embedding.bfs5.com`) |
| `EMBEDDING_SERVICE_API_KEY` | API key for embedding service (`x-api-key` header) |
| `HATTIEBOT_EMBEDDING_DIMENSION` | Embedding dimension: `128`, `256`, `512`, or `768` (default: `768`) |

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
  channels/               # Communication (terminal, zulip, webhook)
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

## License

Add a `LICENSE` file to the repository root for your chosen license (e.g. MIT, Apache-2.0).
