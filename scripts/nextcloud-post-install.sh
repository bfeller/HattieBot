#!/bin/sh
# Nextcloud Docker post-installation hook: enable Talk (spreed) and register
# the HattieBot webhook bot via occ talk:bot:install.
# Runs as www-data. Requires NEXTCLOUD_TALK_BOT_SECRET (and optional NAME, URL).
# Nextcloud 32 (Talk webhook bots supported).
# We exit 0 even if bot registration fails so the container still starts.

if [ -z "${NEXTCLOUD_TALK_BOT_SECRET:-}" ]; then
  echo "=> NEXTCLOUD_TALK_BOT_SECRET not set, skipping Talk bot registration"
  exit 0
fi

echo "=> Enabling Talk app (spreed)..."
if ! php /var/www/html/occ app:enable spreed 2>/dev/null; then
  echo "=> WARNING: Could not enable spreed (Talk); bot registration skipped"
  exit 0
fi

name="${NEXTCLOUD_TALK_BOT_NAME:-HattieBot}"
url="${NEXTCLOUD_TALK_BOT_URL:-http://hattiebot:8080/webhook/talk}"
desc="${NEXTCLOUD_TALK_BOT_DESCRIPTION:-HattieBot agent}"

echo "=> Registering Talk webhook bot: $name"
# Arguments are positional: name secret url [description]
if ! php /var/www/html/occ talk:bot:install "$name" "$NEXTCLOUD_TALK_BOT_SECRET" "$url" "$desc"; then
  echo "=> WARNING: talk:bot:install failed; Nextcloud will still start. Run it manually in the container if needed."
  exit 0
fi

echo "=> Talk bot registered successfully"
exit 0
