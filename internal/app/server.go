package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Server struct {
	store  *Store
	token  string
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewServer(store *Store, token string, logger *slog.Logger) *Server {
	s := &Server{store: store, token: token, logger: logger, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("POST /upload/v1/batches", s.auth(s.upload))
	s.mux.HandleFunc("POST /mcp", s.auth(s.mcp))
	s.mux.HandleFunc("GET /mcp", s.auth(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "SSE stream not required by this server", http.StatusMethodNotAllowed)
	}))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			http.Error(w, "server token is not configured", http.StatusServiceUnavailable)
			return
		}
		want := "Bearer " + s.token
		got := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "null" && !isAllowedOrigin(origin, r.Host) {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
func isAllowedOrigin(origin, requestHost string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.EqualFold(parsed.Host, requestHost)
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var batch UploadBatch
	if err := dec.Decode(&batch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.store.ApplyBatch(r.Context(), batch)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) mcp(w http.ResponseWriter, r *http.Request) {
	if accept := r.Header.Get("Accept"); accept != "" && !strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "Accept must include application/json or text/event-stream", http.StatusNotAcceptable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeRPCError(w, nil, -32700, "Parse error")
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "Invalid Request")
		return
	}
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := "2025-11-25"
		if params.ProtocolVersion == "2025-03-26" || params.ProtocolVersion == "2025-06-18" || params.ProtocolVersion == "2025-11-25" {
			version = params.ProtocolVersion
		}
		result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "ihealth-export", "version": "0.1.0", "description": "Personal Apple Health data server"}}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			writeRPCError(w, req.ID, -32602, "Invalid params")
			return
		}
		result, err = s.callTool(r, params)
		if err != nil {
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": err.Error()}}, "isError": true}
		}
	default:
		writeRPCError(w, req.ID, -32601, "Method not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func (s *Server) callTool(r *http.Request, p callParams) (any, error) {
	argString := func(k string) string { v, _ := p.Arguments[k].(string); return v }
	argInt := func(k string, def int) int {
		v, ok := p.Arguments[k].(float64)
		if !ok {
			return def
		}
		return int(v)
	}
	from, err := normalizeDate(argString("from"), false)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	to, err := normalizeDate(argString("to"), true)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	var data any
	err = nil
	switch p.Name {
	case "health_list_types":
		data, err = s.store.ListTypes(r.Context())
	case "health_query":
		options := QueryOptions{Type: argString("type"), Kind: argString("kind"), From: from, To: to, ActivityName: argString("activity_name"), Cursor: argString("cursor"), Limit: argInt("limit", 100)}
		if v, ok := p.Arguments["activity_type"].(float64); ok {
			n := int64(v)
			options.ActivityType = &n
		}
		var items []map[string]any
		var cursor string
		items, cursor, err = s.store.Query(r.Context(), options)
		data = map[string]any{"items": items, "next_cursor": cursor}
	case "health_list_workouts":
		options := QueryOptions{Kind: "workout", From: from, To: to, ActivityName: argString("activity_name"), Cursor: argString("cursor"), Limit: argInt("limit", 100)}
		if v, ok := p.Arguments["activity_type"].(float64); ok {
			n := int64(v)
			options.ActivityType = &n
		}
		var items []map[string]any
		var cursor string
		items, cursor, err = s.store.Query(r.Context(), options)
		data = map[string]any{"items": items, "next_cursor": cursor}
	case "health_summary":
		data, err = s.store.Summary(r.Context(), argString("type"), from, to, argString("bucket"), argString("aggregation"))
	case "health_profile":
		data, err = s.store.Profiles(r.Context())
	case "health_sync_status":
		data, err = s.store.SyncStatus(r.Context())
	default:
		return nil, fmt.Errorf("unknown tool %q", p.Name)
	}
	if err != nil {
		return nil, err
	}
	encoded, _ := json.MarshalIndent(data, "", "  ")
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(encoded)}}, "structuredContent": map[string]any{"data": data}}, nil
}

func normalizeDate(value string, endOfDay bool) (string, error) {
	if value == "" {
		return "", nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", errors.New("use YYYY-MM-DD or RFC3339")
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func toolDefinitions() []map[string]any {
	object := func(properties map[string]any, required ...string) map[string]any {
		s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	date := map[string]any{"type": "string", "description": "ISO 8601 date/time, inclusive"}
	limit := map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "default": 100}
	cursor := map[string]any{"type": "string"}
	return []map[string]any{
		{"name": "health_list_types", "description": "List stored HealthKit types, units, counts and date ranges.", "inputSchema": object(map[string]any{})},
		{"name": "health_query", "description": "Query raw health samples by HealthKit type, kind and date range.", "inputSchema": object(map[string]any{"type": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"}, "from": date, "to": date, "activity_type": map[string]any{"type": "integer"}, "activity_name": map[string]any{"type": "string"}, "limit": limit, "cursor": cursor})},
		{"name": "health_list_workouts", "description": "List workouts filtered by workout activity and dates.", "inputSchema": object(map[string]any{"from": date, "to": date, "activity_type": map[string]any{"type": "integer"}, "activity_name": map[string]any{"type": "string"}, "limit": limit, "cursor": cursor})},
		{"name": "health_summary", "description": "Aggregate a numeric HealthKit type into time buckets.", "inputSchema": object(map[string]any{"type": map[string]any{"type": "string"}, "from": date, "to": date, "bucket": map[string]any{"type": "string", "enum": []string{"hour", "day", "month"}}, "aggregation": map[string]any{"type": "string", "enum": []string{"sum", "avg", "min", "max", "count"}}}, "type", "bucket", "aggregation")},
		{"name": "health_profile", "description": "Read exported HealthKit characteristics.", "inputSchema": object(map[string]any{})},
		{"name": "health_sync_status", "description": "Show stored sample count and latest synchronization.", "inputSchema": object(map[string]any{})},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}
func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}
