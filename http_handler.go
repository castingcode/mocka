package mocka

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/castingcode/mocaprotocol"
)

var XMLDeclaration = []byte(`<?xml version="1.0" encoding="UTF-8"?>`)

type Router interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	Handle(pattern string, handler http.Handler)
}

type MocaRequestHandlerInterface interface {
	HandleMocaRequest(w http.ResponseWriter, r *http.Request)
}

func RegisterRoutes(router Router, handler MocaRequestHandlerInterface) {
	router.HandleFunc("POST /service", handler.HandleMocaRequest)
}

type MocaRequestHandler struct {
	lookup   *ResponseLookup
	sessions *SessionStore
	journal  *requestJournal
	logger   *slog.Logger
}

var _ MocaRequestHandlerInterface = (*MocaRequestHandler)(nil)

func NewMocaRequestHandler(lookup *ResponseLookup) *MocaRequestHandler {
	return &MocaRequestHandler{
		lookup:   lookup,
		sessions: newSessionStore(),
		journal:  &requestJournal{},
		logger:   slog.Default(),
	}
}

func (h *MocaRequestHandler) HandleMocaRequest(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/moca-xml" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(generateNoContentResponse())
		return
	}
	var request mocaprotocol.MocaRequest
	if err := xml.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := normalizeQuery(request.Query.Text)
	rec := RequestRecord{Time: time.Now(), Query: request.Query.Text, NormalizedQuery: query}
	defer func() { h.journal.add(rec) }()

	if cmd := builtinCommand(query); cmd != "" {
		rec.Match = &MatchInfo{Type: "builtin", Command: cmd}
		switch cmd {
		case "ping":
			writeMocaResponse(w, generatePingResponse())
		case "logout":
			sessionKey, invalidKey := h.sessions.GetSessionKey(request)
			if invalidKey != nil {
				rec.Status = StatusInvalidSessionKey
				writeMocaResponse(w, invalidKey)
				return
			}
			h.sessions.Delete(sessionKey)
			writeMocaResponse(w, generatePingResponse())
		case "login":
			inner, _ := loginInnerQuery(query)
			rec.Status = h.handleLogin(w, strings.SplitN(inner, " where ", 2))
		}
		return
	}

	_, invalidKey := h.sessions.GetSessionKey(request)
	if invalidKey != nil {
		rec.Status = StatusInvalidSessionKey
		writeMocaResponse(w, invalidKey)
		return
	}
	response, entry := h.lookup.resolve(query)
	rec.Status = response.StatusCode
	rec.Match = matchInfo(entry)
	rec.Unmatched = entry == nil

	mocaResponse := mocaprotocol.MocaResponse{
		Status:  response.StatusCode,
		Message: response.Message,
	}
	if response.ResultSet != "" {
		if err := xml.Unmarshal([]byte(response.ResultSet), &mocaResponse.MocaResults); err != nil {
			h.logger.Error("error unmarshalling response", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	body, err := xml.Marshal(mocaResponse)
	if err != nil {
		h.logger.Error("error marshalling response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeMocaResponse(w, append(XMLDeclaration, body...))
}

// handleLogin writes the login response and returns its MOCA status.
func (h *MocaRequestHandler) handleLogin(w http.ResponseWriter, tokens []string) int {
	if len(tokens) < 2 {
		writeMocaResponse(w, generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
		return 802
	}
	params := make(map[string]string)
	for _, cond := range strings.Split(tokens[1], " and ") {
		parts := strings.SplitN(cond, " = ", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "'")
			val = strings.Trim(val, `"`)
			params[strings.TrimSpace(parts[0])] = val
		}
	}
	if params["usr_pswd"] == "" {
		writeMocaResponse(w, generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
		return 802
	}
	response, sessionKey := generateLoginResponse(params["usr_id"])
	h.sessions.Add(sessionKey, params["usr_id"])
	writeMocaResponse(w, response)
	return StatusOK
}

// builtinCommand returns "ping", "login", or "logout" when the normalized
// query is one of the commands mocka handles itself, and "" otherwise.
func builtinCommand(query string) string {
	if strings.HasPrefix(query, "[") {
		return ""
	}
	switch strings.SplitN(query, " where ", 2)[0] {
	case "ping":
		return "ping"
	case "logout user":
		return "logout"
	}
	if _, ok := loginInnerQuery(query); ok {
		return "login"
	}
	return ""
}

func writeMocaResponse(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/moca-xml")
	w.Write(body)
}

// isLoginCommand returns true if the query is a login user command, optionally
// wrapped in a publish data block (with or without a where clause).
func isLoginCommand(query string) bool {
	_, ok := loginInnerQuery(query)
	return ok
}

// loginInnerQuery returns the inner "login user ..." portion of the query if
// the query is a login command (either bare or wrapped in a publish data
// block, with or without a where clause). The second return value reports
// whether the query is a login command.
func loginInnerQuery(query string) (string, bool) {
	normalizedQuery := normalizeQuery(query)

	if strings.HasPrefix(normalizedQuery, "publish data") {
		pipeIdx := strings.Index(normalizedQuery, "| {")
		if pipeIdx < 0 {
			return "", false
		}
		innerPart := strings.TrimSpace(normalizedQuery[pipeIdx+3:])
		if !strings.HasSuffix(innerPart, "}") {
			return "", false
		}
		inner := normalizeQuery(strings.TrimSuffix(innerPart, "}"))
		if !strings.HasPrefix(inner, "login user") {
			return "", false
		}
		return inner, true
	}

	if strings.HasPrefix(normalizedQuery, "login user") {
		return normalizedQuery, true
	}
	return "", false
}
