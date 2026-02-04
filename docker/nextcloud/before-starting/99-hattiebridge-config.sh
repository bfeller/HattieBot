#!/bin/sh
# HattieBridge config (post-install writes on first install; before-starting only runs on install/upgrade).
cfg="/var/www/html/data/hattiebridge-config.json"
if [ ! -f "$cfg" ] && [ -n "${HATTIEBOT_WEBHOOK_URL}" ] && [ -n "${HATTIEBOT_WEBHOOK_SECRET}" ] && [ -n "${HATTIEBOT_BOT_NAME}" ]; then
  HATTIE_USER=$(echo "${HATTIEBOT_BOT_NAME}" | tr '[:upper:]' '[:lower:]' | tr -d ' ')
  php -r '
    $u = $argv[1] ?? ""; $s = $argv[2] ?? ""; $h = $argv[3] ?? "";
    if ($u && $s && $h) {
      file_put_contents("/var/www/html/data/hattiebridge-config.json", json_encode(["webhook_url"=>$u,"webhook_secret"=>$s,"hattie_user"=>$h]));
      echo "=> HattieBridge config written (before-starting)\n";
    }
  ' "$HATTIEBOT_WEBHOOK_URL" "$HATTIEBOT_WEBHOOK_SECRET" "$HATTIE_USER"
fi
