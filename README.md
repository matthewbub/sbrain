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
