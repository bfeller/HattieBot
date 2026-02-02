#!/bin/sh
# Wrapper for Nextcloud container: run default entrypoint, and once installed
# register the HattieBot Talk webhook bot via occ talk:bot:install.
# Requires: NEXTCLOUD_TALK_BOT_SECRET (and optionally NEXTCLOUD_TALK_BOT_NAME, NEXTCLOUD_TALK_BOT_URL).
# Nextcloud 32 (Talk webhook bots supported).

register_bot() {
	# Wait for Nextcloud to be installed and occ to work
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do
		if sudo -u www-data php /var/www/html/occ status 2>/dev/null; then
			break
		fi
		sleep 10
	done
	if [ -z "$NEXTCLOUD_TALK_BOT_SECRET" ]; then
		return 0
	fi
	sudo -u www-data php /var/www/html/occ app:enable spreed 2>/dev/null || true
	name="${NEXTCLOUD_TALK_BOT_NAME:-HattieBot}"
	url="${NEXTCLOUD_TALK_BOT_URL:-http://hattiebot:8080/webhook/talk}"
	desc="${NEXTCLOUD_TALK_BOT_DESCRIPTION:-HattieBot agent}"
	sudo -u www-data php /var/www/html/occ talk:bot:install --name "$name" --secret "$NEXTCLOUD_TALK_BOT_SECRET" --url "$url" --description "$desc" 2>/dev/null || true
}

register_bot &
# Original Nextcloud entrypoint (path may be /entrypoint.sh or /docker-entrypoint.sh in the image)
if [ -x /entrypoint.sh ]; then
  exec /entrypoint.sh "$@"
fi
exec /docker-entrypoint.sh "$@"
