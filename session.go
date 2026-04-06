package mocka

import (
	"encoding/xml"

	"github.com/castingcode/mocaprotocol"
	"github.com/google/uuid"
)

// SessionStore holds in-memory session key → user ID mappings.
// There is no TTL, no eviction, and no concurrency protection — this is
// intentional for a single-use test server.
type SessionStore struct {
	sessions map[string]string
}

func newSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]string)}
}

// Add registers sessionKey for userID.
func (s *SessionStore) Add(key, userID string) {
	s.sessions[key] = userID
}

// Get returns the userID for key and whether it was found.
func (s *SessionStore) Get(key string) (string, bool) {
	v, ok := s.sessions[key]
	return v, ok
}

// Delete removes the session for key.
func (s *SessionStore) Delete(key string) {
	delete(s.sessions, key)
}

// Len returns the number of active sessions.
func (s *SessionStore) Len() int {
	return len(s.sessions)
}

// GetSessionKey extracts and validates the session key from the request
// environment. Returns (key, nil) on success or ("", errorResponseBody) on
// failure.
func (s *SessionStore) GetSessionKey(request mocaprotocol.MocaRequest) (string, []byte) {
	for _, v := range request.Environment.Vars {
		if v.Name == "SESSION_KEY" {
			if _, ok := s.sessions[v.Value]; ok {
				return v.Value, nil
			}
		}
	}
	return "", generateErrorResponse(StatusInvalidSessionKey, "Invalid session key")
}

func generatePingResponse() []byte {
	body, err := xml.Marshal(mocaprotocol.MocaResponse{Status: 0})
	if err != nil {
		return nil
	}
	return append(XMLDeclaration, body...)
}

func generateErrorResponse(status int, message string) []byte {
	body, err := xml.Marshal(mocaprotocol.MocaResponse{Status: status, Message: message})
	if err != nil {
		return nil
	}
	return append(XMLDeclaration, body...)
}

func generateLoginResponse(userID string) ([]byte, string) {
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
	return []byte(`<html>
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
</html>`)
}
