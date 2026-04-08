package mocka

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"

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
	logger   *slog.Logger
}

var _ MocaRequestHandlerInterface = (*MocaRequestHandler)(nil)

func NewMocaRequestHandler(lookup *ResponseLookup) *MocaRequestHandler {
	return &MocaRequestHandler{
		lookup:   lookup,
		sessions: newSessionStore(),
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
	if !strings.HasPrefix(query, "[") {
		tokens := strings.SplitN(query, " where ", 2)
		switch tokens[0] {
		case "ping":
			writeMocaResponse(w, generatePingResponse())
			return
		case "login user":
			h.handleLogin(w, tokens)
			return
		case "logout user":
			sessionKey, invalidKey := h.sessions.GetSessionKey(request)
			if invalidKey != nil {
				writeMocaResponse(w, invalidKey)
				return
			}
			h.sessions.Delete(sessionKey)
			writeMocaResponse(w, generatePingResponse())
			return
		}
	}

	_, invalidKey := h.sessions.GetSessionKey(request)
	if invalidKey != nil {
		writeMocaResponse(w, invalidKey)
		return
	}
	response := h.lookup.GetResponse(query)

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

func (h *MocaRequestHandler) handleLogin(w http.ResponseWriter, tokens []string) {
	if len(tokens) < 2 {
		writeMocaResponse(w, generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
		return
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
		return
	}
	response, sessionKey := generateLoginResponse(params["usr_id"])
	h.sessions.Add(sessionKey, params["usr_id"])
	writeMocaResponse(w, response)
}

func writeMocaResponse(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/moca-xml")
	w.Write(body)
}
