package mocka

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"

	"github.com/castingcode/mocaprotocol"
	"github.com/gin-gonic/gin"
)

var XMLDeclaration = []byte(`<?xml version="1.0" encoding="UTF-8"?>`)

type MocaRequestHandlerInterface interface {
	HandleMocaRequest(c *gin.Context)
}

func RegisterRoutes(router gin.IRoutes, handler MocaRequestHandlerInterface) {
	router.POST("/service", handler.HandleMocaRequest)
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

func (h *MocaRequestHandler) HandleMocaRequest(c *gin.Context) {
	if c.GetHeader("Content-Type") != "application/moca-xml" {
		c.Data(http.StatusOK, "text/html; charset=utf-8", generateNoContentResponse())
		return
	}
	var request mocaprotocol.MocaRequest
	if err := c.BindXML(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := normalizeQuery(request.Query.Text)
	if !strings.HasPrefix(query, "[") {
		tokens := strings.SplitN(query, " where ", 2)
		switch tokens[0] {
		case "ping":
			c.Data(http.StatusOK, "application/moca-xml", generatePingResponse())
			return
		case "login user":
			h.handleLogin(c, tokens)
			return
		case "logout user":
			sessionKey, invalidKey := h.sessions.GetSessionKey(request)
			if invalidKey != nil {
				c.Data(http.StatusOK, "application/moca-xml", invalidKey)
				return
			}
			h.sessions.Delete(sessionKey)
			c.Data(http.StatusOK, "application/moca-xml", generatePingResponse())
			return
		}
	}

	_, invalidKey := h.sessions.GetSessionKey(request)
	if invalidKey != nil {
		c.Data(http.StatusOK, "application/moca-xml", invalidKey)
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
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	body, err := xml.Marshal(mocaResponse)
	if err != nil {
		h.logger.Error("error marshalling response", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/moca-xml", append(XMLDeclaration, body...))
}

func (h *MocaRequestHandler) handleLogin(c *gin.Context, tokens []string) {
	if len(tokens) < 2 {
		c.Data(http.StatusOK, "application/moca-xml", generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
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
		c.Data(http.StatusOK, "application/moca-xml", generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
		return
	}
	response, sessionKey := generateLoginResponse(params["usr_id"])
	h.sessions.Add(sessionKey, params["usr_id"])
	c.Data(http.StatusOK, "application/moca-xml", response)
}
