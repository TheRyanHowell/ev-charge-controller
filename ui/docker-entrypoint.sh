#!/bin/sh
set -e

# Fix ownership of anonymous volumes (created as root, but we run as appuser)
chown -R appuser:appuser /app/node_modules /app/.next 2>/dev/null || true

if [ ! -d "node_modules" ] || [ -z "$(ls -A node_modules 2>/dev/null)" ]; then
  echo "node_modules not found, running npm ci..."
  npm ci
  chown -R appuser:appuser /app/node_modules
fi

mkdir -p .next
chown -R appuser:appuser /app/.next 2>/dev/null || true

# Switch to appuser for the actual command
exec gosu appuser "$@"
