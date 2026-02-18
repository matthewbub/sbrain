package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	server := &server{db: db}
	mux := http.NewServeMux()
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

type server struct {
	db *sql.DB
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
