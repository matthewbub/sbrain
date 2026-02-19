package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type brain struct {
	ID       int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Title    string `json:"title"`
	Context  string `json:"context"`
	Project  string `json:"project"`
	Commits  string `json:"commits"`
	Tags     string `json:"tags"`
}

type logEntry struct {
	ID             int64   `json:"id"`
	CreatedAt      string  `json:"created_at"`
	Level          string  `json:"level"`
	Message        string  `json:"message"`
	Endpoint       string  `json:"endpoint"`
	Method         string  `json:"method"`
	IP             string  `json:"ip"`
	UserAgent      string  `json:"user_agent"`
	RequestID      string  `json:"request_id"`
	StatusCode     *int    `json:"status_code,omitempty"`
	ResponseTimeMs *int    `json:"response_time_ms,omitempty"`
	Metadata       string  `json:"metadata"`
}

func main() {
	dbPath := os.Getenv("SBRAIN_DB")
	if dbPath == "" {
		dbPath = "sbrain.db"
	}

	absDBPath, err := filepath.Abs(dbPath)
	if err != nil {
		log.Printf("warning: could not resolve absolute DB path for %q: %v", dbPath, err)
		absDBPath = dbPath
	}

	log.Printf("database path configured: %q (resolved: %q)", dbPath, absDBPath)
	if err := enforcePersistentDBPath(absDBPath); err != nil {
		log.Fatal(err)
	}

	dbWasMissing := false
	info, err := os.Stat(absDBPath)
	switch {
	case err == nil:
		log.Printf("database file exists at startup: %q (%d bytes)", absDBPath, info.Size())
	case os.IsNotExist(err):
		dbWasMissing = true
		log.Printf("warning: database file does not exist at startup: %q (a new database may be created)", absDBPath)
	default:
		log.Printf("warning: unable to inspect database file %q: %v", absDBPath, err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	if dbWasMissing {
		if info, err := os.Stat(absDBPath); err == nil {
			log.Printf("warning: database file was created during startup: %q (%d bytes)", absDBPath, info.Size())
		}
	}

	server := &server{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi", server.openAPISpecHandler)
	mux.HandleFunc("/brain", server.brainCollectionHandler)
	mux.HandleFunc("/brain/", server.brainItemHandler)
	mux.HandleFunc("/logs", server.logCollectionHandler)
	mux.HandleFunc("/logs/", server.logItemHandler)
	mux.HandleFunc("/", server.notFoundHandler)

	addr := os.Getenv("SBRAIN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("server running at %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func enforcePersistentDBPath(dbPath string) error {
	if !isProductionRuntime() {
		return nil
	}
	if dbPath == "/data" || strings.HasPrefix(dbPath, "/data/") {
		return nil
	}
	return fmt.Errorf("refusing to start in production with SBRAIN_DB=%q; use /data/... and mount persistent storage", dbPath)
}

func isProductionRuntime() bool {
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_PROJECT_ID") != "" {
		return true
	}
	return strings.EqualFold(os.Getenv("APP_ENV"), "production") ||
		strings.EqualFold(os.Getenv("GO_ENV"), "production") ||
		strings.EqualFold(os.Getenv("ENV"), "production")
}

type server struct {
	db *sql.DB
}

func (s *server) openAPISpecHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, openAPISpec())
}

func openAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "sbrain API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/openapi": map[string]any{
				"get": map[string]any{
					"summary": "Get OpenAPI schema for the service",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "OpenAPI document",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"type": "object"},
								},
							},
						},
					},
				},
			},
			"/brain": map[string]any{
				"get": map[string]any{
					"summary": "List all brain records",
					"operationId": "listBrains",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "List of brain records",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "array",
										"items": map[string]any{"$ref": "#/components/schemas/Brain"},
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"summary": "Create a brain record",
					"operationId": "createBrain",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/BrainCreate"},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{
							"description": "Created brain record",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/Brain"},
								},
							},
						},
						"400": map[string]any{"description": "Bad request"},
						"500": map[string]any{"description": "Server error"},
					},
				},
			},
			"/brain/{id}": map[string]any{
				"parameters": []map[string]any{
					{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type":   "integer",
							"format": "int64",
						},
					},
				},
				"get": map[string]any{
					"summary": "Get a brain record by ID",
					"operationId": "getBrainById",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Brain record",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/Brain"},
								},
							},
						},
						"400": map[string]any{"description": "Invalid ID"},
						"404": map[string]any{"description": "Not found"},
						"500": map[string]any{"description": "Server error"},
					},
				},
			},
			"/logs": map[string]any{
				"get": map[string]any{
					"summary": "List all logs",
					"operationId": "listLogs",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "List of logs",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "array",
										"items": map[string]any{"$ref": "#/components/schemas/LogEntry"},
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"summary": "Create a log",
					"operationId": "createLog",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/LogCreate"},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{
							"description": "Created log",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/LogEntry"},
								},
							},
						},
						"400": map[string]any{"description": "Bad request"},
						"500": map[string]any{"description": "Server error"},
					},
				},
			},
			"/logs/{id}": map[string]any{
				"parameters": []map[string]any{
					{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type":   "integer",
							"format": "int64",
						},
					},
				},
				"get": map[string]any{
					"summary": "Get a log by ID",
					"operationId": "getLogById",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Log entry",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/LogEntry"},
								},
							},
						},
						"400": map[string]any{"description": "Invalid ID"},
						"404": map[string]any{"description": "Not found"},
						"500": map[string]any{"description": "Server error"},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"Brain": map[string]any{
					"type": "object",
					"required": []string{
						"id",
						"created_at",
						"title",
						"context",
						"project",
						"commits",
						"tags",
					},
					"properties": map[string]any{
						"id":        map[string]any{"type": "integer", "format": "int64"},
						"created_at": map[string]any{"type": "string", "description": "timestamp"},
						"title":     map[string]any{"type": "string"},
						"context":   map[string]any{"type": "string"},
						"project":   map[string]any{"type": "string"},
						"commits":   map[string]any{"type": "string"},
						"tags":      map[string]any{"type": "string"},
					},
				},
				"BrainCreate": map[string]any{
					"type": "object",
					"required": []string{
						"title",
						"context",
						"project",
					},
					"properties": map[string]any{
						"title":   map[string]any{"type": "string"},
						"context": map[string]any{"type": "string"},
						"project": map[string]any{"type": "string"},
						"commits": map[string]any{"type": "string"},
						"tags":    map[string]any{"type": "string"},
					},
				},
				"LogEntry": map[string]any{
					"type": "object",
					"required": []string{
						"id",
						"created_at",
						"level",
						"message",
						"endpoint",
						"method",
						"ip",
						"user_agent",
						"request_id",
						"metadata",
					},
					"properties": map[string]any{
						"id":              map[string]any{"type": "integer", "format": "int64"},
						"created_at":      map[string]any{"type": "string", "description": "timestamp"},
						"level":           map[string]any{"type": "string"},
						"message":         map[string]any{"type": "string"},
						"endpoint":        map[string]any{"type": "string"},
						"method":          map[string]any{"type": "string"},
						"ip":              map[string]any{"type": "string"},
						"user_agent":      map[string]any{"type": "string"},
						"request_id":      map[string]any{"type": "string"},
						"status_code":     map[string]any{"type": "integer", "format": "int32", "nullable": true},
						"response_time_ms": map[string]any{"type": "integer", "format": "int32", "nullable": true},
						"metadata":        map[string]any{"type": "string"},
					},
				},
				"LogCreate": map[string]any{
					"type": "object",
					"required": []string{
						"message",
					},
					"properties": map[string]any{
						"level":           map[string]any{"type": "string", "default": "info"},
						"message":         map[string]any{"type": "string"},
						"endpoint":        map[string]any{"type": "string"},
						"method":          map[string]any{"type": "string"},
						"ip":              map[string]any{"type": "string"},
						"user_agent":      map[string]any{"type": "string"},
						"request_id":      map[string]any{"type": "string"},
						"status_code":     map[string]any{"type": "integer", "format": "int32", "nullable": true},
						"response_time_ms": map[string]any{"type": "integer", "format": "int32", "nullable": true},
						"metadata":        map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func (s *server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
		})
		return
	}

	http.NotFound(w, r)
}

func (s *server) brainCollectionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getBrains(w, r)
	case http.MethodPost:
		s.createBrain(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) brainItemHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.URL.Path, "/brain/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getBrainByID(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) logCollectionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getLogs(w, r)
	case http.MethodPost:
		s.createLog(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) logItemHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.URL.Path, "/logs/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getLogByID(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) getBrains(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, created_at, title, context, project, commits, tags
		FROM second_brain ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, fmt.Sprintf("query brains: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []brain
	for rows.Next() {
		var b brain
		if err := rows.Scan(&b.ID, &b.CreatedAt, &b.Title, &b.Context, &b.Project, &b.Commits, &b.Tags); err != nil {
			http.Error(w, fmt.Sprintf("scan brain: %v", err), http.StatusInternalServerError)
			return
		}
		items = append(items, b)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("iterate brains: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *server) getBrainByID(w http.ResponseWriter, r *http.Request, id int64) {
	var b brain
	row := s.db.QueryRow(`SELECT id, created_at, title, context, project, commits, tags
		FROM second_brain WHERE id = ?`, id)
	if err := row.Scan(&b.ID, &b.CreatedAt, &b.Title, &b.Context, &b.Project, &b.Commits, &b.Tags); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("query brain: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *server) createBrain(w http.ResponseWriter, r *http.Request) {
	var req brain
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode body: %v", err), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Context) == "" || strings.TrimSpace(req.Project) == "" {
		http.Error(w, "title, context, and project are required", http.StatusBadRequest)
		return
	}

	res, err := s.db.Exec(`INSERT INTO second_brain (title, context, project, commits, tags)
		VALUES (?, ?, ?, ?, ?)`, req.Title, req.Context, req.Project, req.Commits, req.Tags)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert brain: %v", err), http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	var b brain
	row := s.db.QueryRow(`SELECT id, created_at, title, context, project, commits, tags
		FROM second_brain WHERE id = ?`, id)
	if err := row.Scan(&b.ID, &b.CreatedAt, &b.Title, &b.Context, &b.Project, &b.Commits, &b.Tags); err != nil {
		http.Error(w, fmt.Sprintf("load brain: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSONStatus(w, http.StatusCreated, b)
}

func (s *server) getLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, created_at, level, message, endpoint, method, ip, user_agent,
		request_id, status_code, response_time_ms, metadata
		FROM logs ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, fmt.Sprintf("query logs: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []logEntry
	for rows.Next() {
		var l logEntry
		var statusCode sql.NullInt64
		var responseMs sql.NullInt64
		if err := rows.Scan(&l.ID, &l.CreatedAt, &l.Level, &l.Message, &l.Endpoint, &l.Method, &l.IP,
			&l.UserAgent, &l.RequestID, &statusCode, &responseMs, &l.Metadata); err != nil {
			http.Error(w, fmt.Sprintf("scan log: %v", err), http.StatusInternalServerError)
			return
		}
		if statusCode.Valid {
			sc := int(statusCode.Int64)
			l.StatusCode = &sc
		}
		if responseMs.Valid {
			rt := int(responseMs.Int64)
			l.ResponseTimeMs = &rt
		}
		items = append(items, l)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("iterate logs: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *server) getLogByID(w http.ResponseWriter, r *http.Request, id int64) {
	var l logEntry
	var statusCode sql.NullInt64
	var responseMs sql.NullInt64
	row := s.db.QueryRow(`SELECT id, created_at, level, message, endpoint, method, ip, user_agent,
		request_id, status_code, response_time_ms, metadata
		FROM logs WHERE id = ?`, id)
	if err := row.Scan(&l.ID, &l.CreatedAt, &l.Level, &l.Message, &l.Endpoint, &l.Method, &l.IP,
		&l.UserAgent, &l.RequestID, &statusCode, &responseMs, &l.Metadata); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("query log: %v", err), http.StatusInternalServerError)
		return
	}
	if statusCode.Valid {
		sc := int(statusCode.Int64)
		l.StatusCode = &sc
	}
	if responseMs.Valid {
		rt := int(responseMs.Int64)
		l.ResponseTimeMs = &rt
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *server) createLog(w http.ResponseWriter, r *http.Request) {
	var req logEntry
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode body: %v", err), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Level) == "" {
		req.Level = "info"
	}
	if strings.TrimSpace(req.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	var statusCode any
	if req.StatusCode != nil {
		statusCode = *req.StatusCode
	}
	var responseMs any
	if req.ResponseTimeMs != nil {
		responseMs = *req.ResponseTimeMs
	}

	res, err := s.db.Exec(`INSERT INTO logs (level, message, endpoint, method, ip, user_agent, request_id, status_code, response_time_ms, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Level, req.Message, req.Endpoint, req.Method, req.IP, req.UserAgent, req.RequestID, statusCode, responseMs, req.Metadata)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert log: %v", err), http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	var l logEntry
	var scanStatusCode sql.NullInt64
	var scanResponseMs sql.NullInt64
	row := s.db.QueryRow(`SELECT id, created_at, level, message, endpoint, method, ip, user_agent,
		request_id, status_code, response_time_ms, metadata
		FROM logs WHERE id = ?`, id)
	if err := row.Scan(&l.ID, &l.CreatedAt, &l.Level, &l.Message, &l.Endpoint, &l.Method, &l.IP,
		&l.UserAgent, &l.RequestID, &scanStatusCode, &scanResponseMs, &l.Metadata); err != nil {
		http.Error(w, fmt.Sprintf("load log: %v", err), http.StatusInternalServerError)
		return
	}
	if scanStatusCode.Valid {
		sc := int(scanStatusCode.Int64)
		l.StatusCode = &sc
	}
	if scanResponseMs.Valid {
		rt := int(scanResponseMs.Int64)
		l.ResponseTimeMs = &rt
	}

	writeJSONStatus(w, http.StatusCreated, l)
}

func parseID(path string, prefix string) (int64, error) {
	idText := strings.TrimPrefix(path, prefix)
	if strings.Contains(idText, "/") || idText == "" {
		return 0, errors.New("invalid id")
	}
	return strconv.ParseInt(idText, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
