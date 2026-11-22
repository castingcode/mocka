package mocka

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxRequestRecords caps the request log so a long-running server does not
// grow without bound. The oldest records are dropped first.
const maxRequestRecords = 1000

// AdminHandlerInterface is the set of admin endpoints registered by
// RegisterAdminRoutes.
type AdminHandlerInterface interface {
	HandleMatch(w http.ResponseWriter, r *http.Request)
	HandleListRequests(w http.ResponseWriter, r *http.Request)
	HandleClearRequests(w http.ResponseWriter, r *http.Request)
}

var _ AdminHandlerInterface = (*MocaRequestHandler)(nil)

// RegisterAdminRoutes registers the admin endpoints under /_mocka/.
func RegisterAdminRoutes(router Router, handler AdminHandlerInterface) {
	router.HandleFunc("POST /_mocka/match", handler.HandleMatch)
	router.HandleFunc("GET /_mocka/requests", handler.HandleListRequests)
	router.HandleFunc("DELETE /_mocka/requests", handler.HandleClearRequests)
}

// MatchInfo describes what handled a query: a registered entry, or one of the
// built-in commands.
type MatchInfo struct {
	Type    string            `json:"type"` // exact, publish_data, prefix, or builtin
	Query   string            `json:"query,omitempty"`
	Inner   string            `json:"inner,omitempty"`
	Context map[string]string `json:"context,omitempty"`
	Prefix  string            `json:"prefix,omitempty"`
	Command string            `json:"command,omitempty"` // builtin only: ping, login, or logout
	Source  string            `json:"source,omitempty"`  // where the entry is defined
}

// RequestRecord is one MOCA request received by a MocaRequestHandler.
type RequestRecord struct {
	Time            time.Time  `json:"time"`
	Query           string     `json:"query"` // as received
	NormalizedQuery string     `json:"normalized_query"`
	Status          int        `json:"status"`
	Match           *MatchInfo `json:"match"`     // nil when nothing handled the query
	Unmatched       bool       `json:"unmatched"` // true when no registered response matched
}

func matchInfo(e *Entry) *MatchInfo {
	if e == nil {
		return nil
	}
	return &MatchInfo{
		Type:    string(e.MatchType),
		Query:   e.Query,
		Inner:   e.Inner,
		Context: e.Context,
		Prefix:  e.Prefix,
		Source:  e.Source,
	}
}

type requestJournal struct {
	mu      sync.Mutex
	records []RequestRecord
}

func (j *requestJournal) add(rec RequestRecord) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = append(j.records, rec)
	if len(j.records) > maxRequestRecords {
		j.records = j.records[len(j.records)-maxRequestRecords:]
	}
}

func (j *requestJournal) list() []RequestRecord {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]RequestRecord{}, j.records...)
}

func (j *requestJournal) clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = nil
}

// Requests returns the MOCA requests received so far, oldest first, up to the
// most recent 1000.
func (h *MocaRequestHandler) Requests() []RequestRecord {
	return h.journal.list()
}

// ClearRequests empties the request log.
func (h *MocaRequestHandler) ClearRequests() {
	h.journal.clear()
}

type matchResult struct {
	Query           string         `json:"query"`
	NormalizedQuery string         `json:"normalized_query"`
	Match           *MatchInfo     `json:"match"`              // nil when nothing matched
	Response        *matchResponse `json:"response,omitempty"` // omitted for built-in commands
}

type matchResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message,omitempty"`
	Results string `json:"results,omitempty"`
}

// HandleMatch reports how a query would be handled, without needing a session.
// The request body is the query text, or {"query": "..."} when the
// Content-Type is application/json.
func (h *MocaRequestHandler) HandleMatch(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading request body: %v", err), http.StatusBadRequest)
		return
	}
	query := string(body)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("parsing JSON body: %v", err), http.StatusBadRequest)
			return
		}
		query = req.Query
	}
	if strings.TrimSpace(query) == "" {
		http.Error(w, "the request body must contain a query", http.StatusBadRequest)
		return
	}

	result := matchResult{Query: query, NormalizedQuery: normalizeQuery(query)}
	if cmd := builtinCommand(result.NormalizedQuery); cmd != "" {
		result.Match = &MatchInfo{Type: "builtin", Command: cmd}
	} else {
		response, entry := h.lookup.resolve(result.NormalizedQuery)
		result.Match = matchInfo(entry)
		result.Response = &matchResponse{
			Status:  response.StatusCode,
			Message: response.Message,
			Results: response.ResultSet,
		}
	}
	h.writeJSON(w, http.StatusOK, result)
}

// HandleListRequests returns the request log as {"requests": [...]}, oldest
// first. With ?unmatched=true it returns only requests that no registered
// response matched.
func (h *MocaRequestHandler) HandleListRequests(w http.ResponseWriter, r *http.Request) {
	records := h.journal.list()
	if r.URL.Query().Get("unmatched") == "true" {
		filtered := records[:0]
		for _, rec := range records {
			if rec.Unmatched {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}
	h.writeJSON(w, http.StatusOK, struct {
		Requests []RequestRecord `json:"requests"`
	}{records})
}

// HandleClearRequests empties the request log.
func (h *MocaRequestHandler) HandleClearRequests(w http.ResponseWriter, _ *http.Request) {
	h.journal.clear()
	w.WriteHeader(http.StatusNoContent)
}

func (h *MocaRequestHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) // keep result XML readable
	if err := enc.Encode(v); err != nil {
		h.logger.Error("writing JSON response", "error", err)
	}
}
