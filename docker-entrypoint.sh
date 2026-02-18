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

/usr/local/bin/migrate -path /app/migrations -database "sqlite3://$SBRAIN_DB" up
exec /usr/local/bin/sbrain
