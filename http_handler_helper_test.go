package mocka

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
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

// creteEmptyResponseFiles creates empty response files in a temporary directory and returns the directory path
func creteEmptyResponseFiles(t *testing.T) string {
	t.Helper()
	responseDirectory := t.TempDir()
	f, err := os.Create(fmt.Sprintf("%s/responses.yml", responseDirectory))
	if err != nil {
		t.Fatal(err)
		return responseDirectory
	}
	f.Close()
	uf, err := os.Create(fmt.Sprintf("%s/user_responses.yml", responseDirectory))
	if err != nil {
		t.Fatal(err)
		return responseDirectory
	}
	uf.Close()
	return responseDirectory
}

// createResponseFiles creates response files in a temporary directory and returns the directory path
func createResponseFiles(t *testing.T, responseFilePath, userResponseFilePath string) string {
	t.Helper()
	responseDirectory := t.TempDir()
	if responseFilePath == "" {
		f, err := os.Create(fmt.Sprintf("%s/responses.yml", responseDirectory))
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
		f.Close()
	} else {
		data, err := os.ReadFile(responseFilePath)
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
		err = os.WriteFile(fmt.Sprintf("%s/responses.yml", responseDirectory), []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
	}
	if userResponseFilePath == "" {
		f, err := os.Create(fmt.Sprintf("%s/user_responses.yml", responseDirectory))
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
		f.Close()
	} else {
		data, err := os.ReadFile(userResponseFilePath)
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
		err = os.WriteFile(fmt.Sprintf("%s/user_responses.yml", responseDirectory), []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
			return responseDirectory
		}
	}
	return responseDirectory
}
