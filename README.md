# sbrain

## SQLite migrations (golang-migrate)

Install `migrate` with SQLite support:

```bash
CGO_ENABLED=1 go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Verify SQLite driver is available:

```bash
$(go env GOPATH)/bin/migrate -help | rg -i sqlite
```

Run migrations:

```bash
$(go env GOPATH)/bin/migrate -path ./migrations -database "sqlite3://./sbrain.db" up
```

If `migrate` shows `unknown driver sqlite3`:

1. Your shell is likely using Homebrew's binary first (`/opt/homebrew/bin/migrate`).
2. Use the Go-installed binary directly:

```bash
$(go env GOPATH)/bin/migrate -path ./migrations -database "sqlite3://./sbrain.db" up
```

Optional: put `$HOME/go/bin` before Homebrew in your `PATH` so `migrate` resolves to the SQLite-enabled binary.

## Docker / Railway

Build and run locally:

```bash
docker build -t sbrain .
docker run -p 8080:8080 -v sbrain-data:/data -e PORT=8080 sbrain
```

Railway-friendly behavior:
- Exposes port `8080`
- Supports Railway’s `PORT` env var
- Stores the SQLite DB at `/data/sbrain.db`
- Uses a startup migration step before serving

If Railway provides a persistent volume, mount it at `/data` and keep `SBRAIN_DB=/data/sbrain.db`.

## API examples with `curl`

The application exposes a small HTTP API on port `8080` by default.

Set a base URL for the running server:

```bash
BASE_URL="http://localhost:8080"
```

Health check:

```bash
curl -i "$BASE_URL/"
```

Brains collection:

```bash
# List all brains
curl -sS "$BASE_URL/brain"

# Create a brain
curl -sS -X POST "$BASE_URL/brain" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Example title",
    "context": "Important context",
    "project": "sbrain",
    "commits": "abc123",
    "tags": "ops,notes"
  }'
```

Single brain:

```bash
curl -sS "$BASE_URL/brain/1"
```

Logs collection:

```bash
# List all logs
curl -sS "$BASE_URL/logs"

# Create a log entry
curl -sS -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d '{
    "level": "info",
    "message": "Service started",
    "endpoint": "/",
    "method": "GET",
    "ip": "127.0.0.1",
    "request_id": "req-1"
  }'
```

Single log:

```bash
curl -sS "$BASE_URL/logs/1"
```

Notes:

- All mutating requests use `POST`.
- Unhandled methods return `405 Method Not Allowed`.
