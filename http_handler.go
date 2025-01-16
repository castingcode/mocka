package mocka

import (
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/castingcode/mocaprotocol"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var XMLDeclaration = []byte(`<?xml version="1.0" encoding="UTF-8"?>`)

type MocaRequestHandlerInterface interface {
	HandleMocaRequest(c *gin.Context)
}

func RegisterRoutes(router gin.IRoutes, handler MocaRequestHandlerInterface) {
	router.POST("/service", handler.HandleMocaRequest)
}

/*
 <var name="SESSION_KEY" value=";uid=CYCLEUSER07|sid=5498bc9a-6a70-4720-8615-45166567db93|dt= |sec=ALL;igzFGgb9UdWfaR4dHEjKOUaMW2" />
 523 Invalid session key
*/

type MocaRequestHandler struct {
	lookup   *ResponseLookup
	sessions map[string]string
}

func NewMocaRequestHandler(lookup *ResponseLookup) *MocaRequestHandler {
	return &MocaRequestHandler{
		lookup:   lookup,
		sessions: make(map[string]string),
	}
}

func (h *MocaRequestHandler) HandleMocaRequest(c *gin.Context) {
	if c.GetHeader("Content-Type") != "application/xml-moca" {
		c.Data(http.StatusOK, "text/html; charset=utf-8", generateNoContentResponse())
		return
	}
	var request mocaprotocol.MocaRequest
	if err := c.BindXML(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := strings.TrimSpace(request.Query.Text)
	// if it's groovy or sql, leave it alone;
	// else, we'll standardize the spacing and then check if this is a ping or login user command
	if !strings.HasPrefix(query, "[") {
		query = strings.Join(strings.Fields(query), " ")
		tokens := strings.SplitN(query, " where ", 2)
		if tokens[0] == "ping" {
			c.Data(http.StatusOK, "application/xml-moca", generatePingResponse())
			return
		}
		if tokens[0] == "login user" {
			conditions := strings.Split(tokens[1], " and ")
			params := make(map[string]string)
			for _, condition := range conditions {
				parts := strings.SplitN(condition, "=", 2)
				if len(parts) == 2 {
					value := strings.TrimSpace(parts[1])
					// can use single or double quotes
					value = strings.Trim(value, "'")
					value = strings.Trim(value, `"`)
					params[strings.TrimSpace(parts[0])] = value
				}
			}
			userID := params["usr_id"]
			password := params["usr_pswd"]

			// any password is fine for now
			if password == "" {
				c.Data(http.StatusOK, "application/xml-moca", generateErrorResponse(802, "Missing argument: Password (usr_pswd)"))
				return
			}

			response, sessionKey := generateLoginResponse(userID)
			h.sessions[sessionKey] = userID
			c.Data(http.StatusOK, "application/xml-moca", response)
			return
		}
		if tokens[0] == "logout user" {
			sessionKey, invalidKey := h.GetSessionKey(request)
			if invalidKey != nil {
				c.Data(http.StatusOK, "application/xml-moca", invalidKey)
				return
			}
			delete(h.sessions, sessionKey)
			c.Data(http.StatusOK, "application/xml-moca", generatePingResponse())
			return
		}
		// handle options to run with and without where clause
	}

	// now, make sure they have a valid session key

	response := h.lookup.GetResponse("super", "noop")

	// TODO: Get the session key from the request
	// if no session key, see if this is a ping or login user command
	// otherwise, return an error
	// if session key, get the response from the lookup table by full command; else by simple noun/verb

	c.Data(http.StatusOK, "application/xml-moca", []byte(response.ResultSet))
}

// returns either the session key or an error response
func (h *MocaRequestHandler) GetSessionKey(request mocaprotocol.MocaRequest) (string, []byte) {
	for _, v := range request.Environment.Vars {
		if v.Name == "SESSION_KEY" {
			if _, ok := h.sessions[v.Value]; ok {
				return v.Value, nil
			}
		}
	}
	response := mocaprotocol.MocaResponse{
		Status:  523,
		Message: "Invalid session key",
	}
	body, _ := xml.Marshal(response)
	return "", append(XMLDeclaration, body...)
}

func generatePingResponse() []byte {
	response := mocaprotocol.MocaResponse{
		Status: 0,
	}
	body, err := xml.Marshal(response)
	if err != nil {
		return nil
	}
	return append(XMLDeclaration, body...)
}

func generateErrorResponse(status int, message string) []byte {
	response := mocaprotocol.MocaResponse{
		Status:  status,
		Message: message,
	}
	body, err := xml.Marshal(response)
	if err != nil {
		return nil
	}
	return append(XMLDeclaration, body...)
}

func generateLoginResponse(userID string) ([]byte, string) {
	// we'll just use a simple UUID for the session key for now.
	// if need be in the future, we'll more accurately mock this
	sessionKey := uuid.NewString()
	response := mocaprotocol.MocaResponse{
		MocaResults: mocaprotocol.MocaResults{
			Metadata: mocaprotocol.Metadata{
				Columns: []mocaprotocol.Column{
					{Name: "usr_id", Type: mocaprotocol.MocaString, Nullable: "true", Length: "0"},
					{Name: "locale_id", Type: mocaprotocol.MocaString, Nullable: "true", Length: "0"},
					{Name: "addon_id", Type: mocaprotocol.MocaString, Nullable: "true", Length: "0"},
					{Name: "cust_lvl", Type: mocaprotocol.MocaInt, Nullable: "true", Length: "0"},
					{Name: "session_key", Type: mocaprotocol.MocaString, Nullable: "true", Length: "0"},
					{Name: "pswd_expir", Type: mocaprotocol.MocaInt, Nullable: "true", Length: "0"},
					{Name: "pswd_expir_dte", Type: mocaprotocol.MocaDate, Nullable: "true", Length: "0"},
					{Name: "pswd_disable", Type: mocaprotocol.MocaInt, Nullable: "true", Length: "0"},
					{Name: "pswd_chg_flg", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
					{Name: "pswd_expir_flg", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
					{Name: "pswd_warn_flg", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
					{Name: "srv_typ", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
					{Name: "super_usr_flg", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
					{Name: "ext_ath_flg", Type: mocaprotocol.MocaFlag, Nullable: "true", Length: "0"},
				},
			},
			Data: mocaprotocol.Data{
				Rows: []mocaprotocol.Row{
					{Fields: []mocaprotocol.Field{
						{Value: userID},
						{Value: "US_ENGLISH"},
						{Value: "WM,lm,SEAMLES,SEAMLES,3pl"},
						{Value: "10"},
						{Value: sessionKey},
						{Null: "true"},
						{Null: "true"},
						{Value: "6008"},
						{Value: "0"},
						{Value: "0"},
						{Value: "0"},
						{Value: "DEVELOPMENT"},
						{Value: "1"},
						{Value: "0"},
					}},
				},
			},
		},
	}
	body, err := xml.Marshal(response)
	if err != nil {
		return nil, sessionKey
	}
	return append(XMLDeclaration, body...), sessionKey
}

func generateNoContentResponse() []byte {
	content := `<html>
<head>
<meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
<title>Error 500 java.lang.NullPointerException</title>
</head>
<body><h2>HTTP ERROR 500 java.lang.NullPointerException</h2>
<table>
<tr><th>URI:</th><td>/service</td></tr>
<tr><th>STATUS:</th><td>500</td></tr>
<tr><th>MESSAGE:</th><td>java.lang.NullPointerException</td></tr>
</table>

</body>
</html>`
	return []byte(content)
}
