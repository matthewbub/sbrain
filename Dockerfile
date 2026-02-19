FROM golang:1.24-alpine AS builder

RUN apk add --no-cache --virtual .build-deps \
    ca-certificates \
    git \
    gcc \
    musl-dev \
    sqlite-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o /out/sbrain .
RUN CGO_ENABLED=1 go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM alpine:3.20 AS runtime

RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app

COPY --from=builder /out/sbrain /usr/local/bin/sbrain
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /src/migrations ./migrations
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
    && mkdir -p /data

EXPOSE 8080
ENV SBRAIN_DB=/data/sbrain.db
ENV SBRAIN_ADDR=:8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
