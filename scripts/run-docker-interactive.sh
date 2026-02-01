#!/usr/bin/env bash
# Testing script: start HattieBot with a clean slate so first-boot always runs.
# Wipes DB and config so you get: OpenRouter key, model, bot name, who it's talking to, purpose.
# Then chat. Enter to send, Ctrl+C to exit.
#
# Usage:
#   ./scripts/run-docker-interactive.sh
#   OPENROUTER_API_KEY=sk-... HATTIEBOT_MODEL=moonshotai/kimi-k2.5 ./scripts/run-docker-interactive.sh

set -e
cd "$(dirname "$0")/.."
SCRIPT_DIR="$(pwd)"

DATA_DIR="${DATA_DIR:-$SCRIPT_DIR/data}"
mkdir -p "$DATA_DIR"

# Clean start: remove config and DB so first-boot prompts run every time (testing script).
rm -f "$DATA_DIR"/config.json "$DATA_DIR"/system_purpose.txt "$DATA_DIR"/hattiebot.db

# Always rebuild so you get the latest code
echo "Building HattieBot image..."
docker build -t hattiebot .

API_PORT="${HATTIEBOT_API_PORT:-}"
if [ -n "$API_PORT" ]; then
  echo "  API: http://localhost:$API_PORT (POST /chat, POST /v1/chat, GET /health)"
  DOCKER_PORTS=(-p "$API_PORT:$API_PORT" -e "HATTIEBOT_API_PORT=$API_PORT")
else
  DOCKER_PORTS=()
fi
echo "Starting HattieBot..."
echo "  Data/config: $DATA_DIR"
echo "  Workspace:   $SCRIPT_DIR"
echo "  Enter to send  |  Ctrl+C to exit"
echo ""

docker run --rm -it \
  "${DOCKER_PORTS[@]}" \
  -v "$DATA_DIR:/data" \
  -v "$SCRIPT_DIR:/workspace" \
  -e HATTIEBOT_CONFIG_DIR=/data \
  -e OPENROUTER_API_KEY="${OPENROUTER_API_KEY:-}" \
  -e HATTIEBOT_MODEL="${HATTIEBOT_MODEL:-}" \
  -e HATTIEBOT_API_ONLY="${HATTIEBOT_API_ONLY:-}" \
  -e HATTIEBOT_SEED_CONFIG="${HATTIEBOT_SEED_CONFIG:-}" \
  -e HATTIEBOT_BOT_NAME="${HATTIEBOT_BOT_NAME:-}" \
  -e HATTIEBOT_AUDIENCE="${HATTIEBOT_AUDIENCE:-}" \
  -e HATTIEBOT_PURPOSE="${HATTIEBOT_PURPOSE:-}" \
  -w /workspace \
  hattiebot
