#!/bin/bash
# HattieBot Interactive Startup Script
# Starts the Docker container and enters the terminal chat

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Check for --fresh flag
if [[ "$1" == "--fresh" ]]; then
  echo "🧹 Cleaning data directory for fresh start..."
  rm -rf data
fi

# Ensure data directory exists
mkdir -p data

# Build the image if needed
echo "🔨 Building HattieBot Docker image..."
docker build -t hattiebot . || { echo "❌ Build failed"; exit 1; }

# Stop any existing container
docker rm -f hattiebot-interactive 2>/dev/null || true

echo ""
echo "🚀 Starting HattieBot interactive session..."
if [[ ! -f data/hattiebot.db ]]; then
  echo "   First run detected - you will be prompted for setup."
fi
echo ""

# Run in interactive mode (not API-only)
# -it: interactive terminal
# -v data:/data: persist database and config
# -v .:/workspace: workspace for tools
docker run -it --rm \
  -v "$(pwd)/data:/data" \
  -v "$(pwd):/workspace" \
  -e HATTIEBOT_CONFIG_DIR=/data \
  -w /workspace \
  --name hattiebot-interactive \
  hattiebot
