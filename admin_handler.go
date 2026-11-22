package mocka

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/castingcode/mocaprotocol"
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
	HandleListResponses(w http.ResponseWriter, r *http.Request)
	HandlePutResponse(w http.ResponseWriter, r *http.Request)
	HandleDeleteResponse(w http.ResponseWriter, r *http.Request)
	HandleReload(w http.ResponseWriter, r *http.Request)
}

var _ AdminHandlerInterface = (*MocaRequestHandler)(nil)

// RegisterAdminRoutes registers the admin endpoints under /_mocka/.
func RegisterAdminRoutes(router Router, handler AdminHandlerInterface) {
	router.HandleFunc("POST /_mocka/match", handler.HandleMatch)
	router.HandleFunc("GET /_mocka/requests", handler.HandleListRequests)
	router.HandleFunc("DELETE /_mocka/requests", handler.HandleClearRequests)
	router.HandleFunc("GET /_mocka/responses", handler.HandleListResponses)
	router.HandleFunc("POST /_mocka/responses", handler.HandlePutResponse)
	router.HandleFunc("DELETE /_mocka/responses/{id}", handler.HandleDeleteResponse)
	router.HandleFunc("POST /_mocka/reload", handler.HandleReload)
}

// MatchInfo describes what handled a query: a registered entry, or one of the
// built-in commands.
type MatchInfo struct {
	ID      int               `json:"id,omitempty"` // identifies a registered entry, e.g. for DELETE /_mocka/responses/{id}
	Type    string            `json:"type"`         // exact, publish_data, prefix, or builtin
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
		ID:      e.id,
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
	Results string `json:"results,omitempty"` // the <moca-results> XML itself, not a file path
}

// responseView is a registered entry as the admin API shows and accepts it.
type responseView struct {
	Match    *MatchInfo    `json:"match"`
	Response matchResponse `json:"response"`
}

func newResponseView(e Entry) responseView {
	return responseView{
		Match:    matchInfo(&e),
		Response: matchResponse{Status: e.StatusCode, Message: e.Message, Results: e.ResultSet},
	}
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

// HandleListResponses returns the registered entries in match order as
// {"responses": [...]}.
func (h *MocaRequestHandler) HandleListResponses(w http.ResponseWriter, _ *http.Request) {
	entries := h.lookup.list()
	views := make([]responseView, 0, len(entries))
	for _, e := range entries {
		views = append(views, newResponseView(e))
	}
	h.writeJSON(w, http.StatusOK, struct {
		Responses []responseView `json:"responses"`
	}{views})
}

// HandlePutResponse registers a response from a JSON body with the same match
// fields as the response files, and the result XML itself in
// response.results. An entry that matches the same queries is replaced in
// place (200); otherwise the entry is added (201). Runtime entries last until
// they are deleted or the responses are reloaded.
func (h *MocaRequestHandler) HandlePutResponse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Match    matchSpec     `json:"match"`
		Response matchResponse `json:"response"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("parsing JSON body: %v", err), http.StatusBadRequest)
		return
	}
	e, err := matchEntry(req.Match, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid match: %v", err), http.StatusBadRequest)
		return
	}
	e.StatusCode = req.Response.Status
	e.Message = req.Response.Message
	e.ResultSet = strings.TrimSpace(req.Response.Results)
	if e.ResultSet != "" {
		var probe mocaprotocol.MocaResponse
		if err := xml.Unmarshal([]byte(e.ResultSet), &probe.MocaResults); err != nil {
			http.Error(w, fmt.Sprintf("invalid response.results XML: %v", err), http.StatusBadRequest)
			return
		}
	}
	e.Source = "admin API"

	stored, replaced := h.lookup.put(e)
	h.logger.Info("response registered", "id", stored.id, "type", stored.MatchType, "replaced", replaced)
	status := http.StatusCreated
	if replaced {
		status = http.StatusOK
	}
	h.writeJSON(w, status, newResponseView(stored))
}

// HandleDeleteResponse removes the registered entry with the id in the path.
func (h *MocaRequestHandler) HandleDeleteResponse(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid id %q", r.PathValue("id")), http.StatusBadRequest)
		return
	}
	if err := h.lookup.remove(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.logger.Info("response removed", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReload reloads the responses from the loader, such as the response
// files, discarding runtime entries. If loading fails, the current entries
// are kept and the error is returned.
func (h *MocaRequestHandler) HandleReload(w http.ResponseWriter, _ *http.Request) {
	if err := h.lookup.reload(); err != nil {
		h.logger.Warn("reloading responses failed; keeping the current responses", "error", err)
		http.Error(w, fmt.Sprintf("reloading responses: %v; the current responses were kept", err), http.StatusInternalServerError)
		return
	}
	count := len(h.lookup.list())
	h.logger.Info("responses reloaded", "count", count)
	h.writeJSON(w, http.StatusOK, struct {
		Count int `json:"count"`
	}{count})
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
