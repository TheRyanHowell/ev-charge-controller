#!/bin/sh
set -e

APP_USER=$(cat /tmp/appuser 2>/dev/null || echo "root")

if [ "$APP_USER" = "root" ]; then
  exec "$@"
else
  exec gosu "$APP_USER" "$@"
fi
