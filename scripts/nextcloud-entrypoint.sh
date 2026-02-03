#!/bin/sh
# Wrapper for Nextcloud container: pass through to default entrypoint.
# HattieBridge is copied in post-install only (as www-data). Copying here as root caused
# "Cannot write into apps" - root-owned files in custom_apps break Nextcloud's app manager.
set -e

# Nextcloud entrypoint expects $1 (e.g. apache2-foreground); use default if none passed
args="$@"
if [ -z "$args" ]; then
  set -- apache2-foreground
fi
if [ -x /entrypoint.sh ]; then
  exec /entrypoint.sh "$@"
fi
exec /docker-entrypoint.sh "$@"
