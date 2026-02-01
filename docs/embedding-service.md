# Embedding service (vector memory)

HattieBot can use a self-hosted embedding API for vector memory (`memorize` / `recall_memories`) instead of OpenRouter embeddings. The recommended service is [EmbeddingGood](https://github.com/bfeller/EmbeddingGood) (FastAPI + EmbeddingGemma).

---

## Configuration

### Environment variables

| Variable | Description |
|----------|-------------|
| `EMBEDDING_SERVICE_URL` | Base URL of the embedding service (e.g. `http://embeddinggood:8000` or `https://embedding.bfs5.com`) |
| `EMBEDDING_SERVICE_API_KEY` | API key sent as `x-api-key` header; **do not commit** — use `.env` or config file |
| `HATTIEBOT_EMBEDDING_DIMENSION` | Dimension for embeddings: `128`, `256`, `512`, or `768` (default: `768`) |

All embeddings in the DB must use the same dimension for similarity search to be valid.

---

## EmbeddingGood API

- **GET** `/health` → `{ "status": "...", "model": "..." }`
- **POST** `/embed` — requires header `x-api-key`
  - **Body (JSON):**
    - `input`: string or array of strings
    - `type`: `"query"` or `"document"` (use `document` for storing memories, `query` for search)
    - `dimension`: `128` | `256` | `512` | `768`
  - **Response:** `{ "embeddings": number[][], "dimension": number }`

Example:

```json
{"input": "hello world", "type": "document", "dimension": 768}
```

---

## Dynamic embedding routing

The system can switch embedding providers at runtime, similar to LLM routing.

- **Config file:** `$CONFIG_DIR/embedding_routing.json` (e.g. `./data/embedding_routing.json` in Docker).
- **Tool:** `manage_embedding_provider` with actions:
  - `list_providers` — show current config
  - `register_provider` — add or update a provider (name, type e.g. `embeddinggood`, `base_url_env`, `api_key_env`, `dimension`)
  - `set_default` — set which provider is used for the default route

When `embedding_routing.json` exists and has a default provider, that provider is used; otherwise the single URL/key from env (or config file) is used. If no embedding service is configured, HattieBot falls back to the LLM client’s `Embed` (e.g. OpenRouter).

---

## Running EmbeddingGood

See [EmbeddingGood on GitHub](https://github.com/bfeller/EmbeddingGood) for build, run, and auth (e.g. `API_KEYS`, `HF_TOKEN` for the gated model). Use `.env` or environment variables for secrets; never commit API keys.
