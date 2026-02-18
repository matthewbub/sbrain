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
