#!/bin/bash
# HattieBot Nextcloud Deployment & Test Script
# Wrapper for docker-compose.nextcloud.yml with volume cleanup utilities.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_FILE="docker-compose.nextcloud.yml"
PROJECT_NAME="hattiebot" # As defined in docker-compose or based on dir name (docker default)

usage() {
  echo "Usage: $0 [options]"
  echo ""
  echo "Options:"
  echo "  -c, --clean            Clean HattieBot data only (keeps Nextcloud/DB)"
  echo "  -n, --clean-nextcloud  Clean Nextcloud data/DB only (keeps HattieBot)"
  echo "  -a, --clean-all        Clean EVERYTHING (HattieBot + Nextcloud + DB)"
  echo "  -b, --build            Force rebuild of images (included by default)"
  echo "  -l, --logs             Tail logs after starting"
  echo "  -h, --help             Show this help message"
  echo ""
  exit 1
}

CLEAN_HATTIEBOT=0
CLEAN_NEXTCLOUD=0
TAIL_LOGS=0

# Parse arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    -c|--clean) CLEAN_HATTIEBOT=1 ;;
    -n|--clean-nextcloud) CLEAN_NEXTCLOUD=1 ;;
    -a|--clean-all) CLEAN_HATTIEBOT=1; CLEAN_NEXTCLOUD=1 ;;
    -l|--logs) TAIL_LOGS=1 ;;
    -h|--help) usage ;;
    *) echo "Unknown parameter passed: $1"; usage ;;
  esac
  shift
done

echo "⬇️  Stopping containers..."
docker compose -f "$COMPOSE_FILE" down

# Volume Cleanup Logic
# Note: Docker Compose project name defaults to directory name. 
# We try to find volumes prefixed with directory name or explicitly.
# docker-compose.nextcloud.yml uses named volumes: hattiebot_data, nextcloud_data, nextcloud_db

# Helper to remove a named volume if it exists
remove_volume() {
  # grep logic handles optional prefix (e.g., hattiebot_hattiebot_data)
  VOL_NAME=$(docker volume ls -q | grep -iE "${PWD##*/}_$1|$1" | head -n 1)
  if [ -n "$VOL_NAME" ]; then
    echo "🧹 Removing volume: $VOL_NAME"
    docker volume rm "$VOL_NAME"
  else
    echo "⚠️  Volume for '$1' not found, skipping."
  fi
}

if [ "$CLEAN_HATTIEBOT" -eq 1 ]; then
  echo "🗑️  Cleaning HattieBot data..."
  remove_volume "hattiebot_data"
fi

if [ "$CLEAN_NEXTCLOUD" -eq 1 ]; then
  echo "🗑️  Cleaning Nextcloud data and database..."
  remove_volume "nextcloud_data"
  remove_volume "nextcloud_db"
  remove_volume "caddy_data"
  remove_volume "caddy_config"
fi

echo "🚀 Starting stack (Nextcloud + HattieBot)..."
docker compose -f "$COMPOSE_FILE" up -d --build

echo "✅ Deployment complete."
echo "   Nextcloud URL: http://localhost (or via configured domain)"
echo "   Webhook URL:   http://localhost:8080/webhook/talk"
# backgroundjobs_mode is set by post-install hook; no need to exec here (would run before Nextcloud is ready)

if [ "$TAIL_LOGS" -eq 1 ]; then
  echo "📋 Tailing logs (Ctrl+C to stop)..."
  docker compose -f "$COMPOSE_FILE" logs -f
fi
