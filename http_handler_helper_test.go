package mocka

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"testing"

	"github.com/castingcode/mocaprotocol"
)

type TestRequestOption func(*mocaprotocol.MocaRequest)

func WithSessionKey(key string) TestRequestOption {
	return func(r *mocaprotocol.MocaRequest) {
		r.Environment.Vars = append(r.Environment.Vars, mocaprotocol.Var{Name: "SESSION_KEY", Value: key})
	}
}

func buildRequest(t *testing.T, command string, options ...TestRequestOption) *http.Request {
	t.Helper()
	request := &mocaprotocol.MocaRequest{
		Autocommit: "true",
		Query:      mocaprotocol.Query{Text: command},
	}
	for _, option := range options {
		option(request)
	}
	requestBody, err := xml.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/service", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/moca-xml")
	return req
}
