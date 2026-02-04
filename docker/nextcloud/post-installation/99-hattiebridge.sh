#!/bin/sh
# Nextcloud Docker post-installation hook: enable Talk (spreed), Passwords, and HattieBridge.
# HattieBridge forwards Talk chat messages to HattieBot when the Hattie user is in the room.
# Runs as www-data. Requires HATTIEBOT_WEBHOOK_SECRET, HATTIEBOT_WEBHOOK_URL, HATTIEBOT_BOT_NAME.
# We exit 0 even if HattieBridge enable fails so the container still starts.

echo "=> Configuring background jobs (cron) for Nextcloud maintenance..."
php /var/www/html/occ config:system:set backgroundjobs_mode --value cron 2>/dev/null || true

echo "=> Enabling Talk app (spreed)..."
if ! php /var/www/html/occ app:enable spreed 2>/dev/null; then
  echo "=> WARNING: Could not enable spreed (Talk)"
  exit 0
fi

echo "=> Installing/Enabling Passwords app..."
php /var/www/html/occ app:install passwords || echo "=> Key: passwords app install check (might be installed)"
php /var/www/html/occ app:enable passwords || echo "=> WARNING: Could not enable passwords app"

echo "=> Installing HattieBridge app (copy from /tmp/hattiebridge-src)..."
mkdir -p /var/www/html/custom_apps
cp -r /tmp/hattiebridge-src /var/www/html/custom_apps/hattiebridge
# Generate autoloader so ChatMessageSentListener is findable (avoids "Class not found" 500)
if command -v composer >/dev/null 2>&1; then
  (cd /var/www/html/custom_apps/hattiebridge && composer dump-autoload --no-dev 2>/dev/null) || true
fi
echo "=> Enabling HattieBridge app..."
if ! php /var/www/html/occ app:enable hattiebridge 2>/dev/null; then
  echo "=> WARNING: Could not enable HattieBridge"
  exit 0
fi
# PHP getenv() often doesn't see Docker env in Nextcloud - write config file for HattieBridge
# Use HATTIEBOT_BOT_NAME as single source; sanitize to lowercase (matches Nextcloud user ID)
if [ -n "${HATTIEBOT_WEBHOOK_URL}" ] && [ -n "${HATTIEBOT_WEBHOOK_SECRET}" ] && [ -n "${HATTIEBOT_BOT_NAME}" ]; then
  HATTIE_USER=$(echo "${HATTIEBOT_BOT_NAME}" | tr '[:upper:]' '[:lower:]' | tr -d ' ')
  php -r '
    $u = $argv[1] ?? ""; $s = $argv[2] ?? ""; $h = $argv[3] ?? "";
    if ($u !== "" && $s !== "" && $h !== "") {
      $j = json_encode(["webhook_url"=>$u,"webhook_secret"=>$s,"hattie_user"=>$h]);
      if ($j !== false && file_put_contents("/var/www/html/data/hattiebridge-config.json", $j) !== false) {
        echo "=> HattieBridge config written (hattie_user=" . $h . ")\n";
      }
    }
  ' "$HATTIEBOT_WEBHOOK_URL" "$HATTIEBOT_WEBHOOK_SECRET" "$HATTIE_USER"
else
  echo "=> WARNING: HATTIEBOT_* env not set, HattieBridge will not forward messages"
fi
echo "=> HattieBridge enabled successfully"
exit 0
