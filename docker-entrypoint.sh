#!/bin/sh
set -eu

if [ -n "${PORT:-}" ]; then
  case "$PORT" in
    ''|*[!0-9]*)
      echo "PORT must be a numeric value, got: $PORT" >&2
      exit 1
      ;;
    *)
      export SBRAIN_ADDR=":$PORT"
      ;;
  esac
fi

export SBRAIN_DB="${SBRAIN_DB:-/data/sbrain.db}"
mkdir -p /data
mkdir -p "$(dirname "$SBRAIN_DB")"

is_production=0
if [ -n "${RAILWAY_ENVIRONMENT:-}" ] || [ -n "${RAILWAY_PROJECT_ID:-}" ]; then
  is_production=1
fi
if [ "${APP_ENV:-}" = "production" ] || [ "${GO_ENV:-}" = "production" ] || [ "${ENV:-}" = "production" ]; then
  is_production=1
fi

if [ "$is_production" -eq 1 ]; then
  case "$SBRAIN_DB" in
    /data/*) ;;
    *)
      echo "Refusing to start in production with SBRAIN_DB=$SBRAIN_DB. Use /data/... and mount a persistent volume at /data." >&2
      exit 1
      ;;
  esac
fi

echo "Starting with SBRAIN_DB=$SBRAIN_DB"
if [ -f "$SBRAIN_DB" ]; then
  echo "Database file exists at startup: $SBRAIN_DB"
else
  echo "WARNING: Database file missing at startup: $SBRAIN_DB (a new database may be created)"
fi

# Ensure sqlite file exists before running migrations; first boot on a fresh
# volume can otherwise fail depending on sqlite open mode used by migrate.
touch "$SBRAIN_DB"

/usr/local/bin/migrate -path /app/migrations -database "sqlite3://$SBRAIN_DB" up
exec /usr/local/bin/sbrain
