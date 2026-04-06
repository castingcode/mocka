package mocka

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// creteEmptyResponseFiles creates an empty responses.yml in a temporary directory and returns the directory path.
func creteEmptyResponseFiles(t *testing.T) string {
	t.Helper()
	responseDirectory := t.TempDir()
	f, err := os.Create(fmt.Sprintf("%s/responses.yml", responseDirectory))
	if err != nil {
		t.Fatal(err)
		return responseDirectory
	}
	f.Close()
	return responseDirectory
}

// createResponseDir copies all files (non-recursive) from srcDir into a temp
// directory and returns it. An empty responses.yml is created if not present
// in srcDir.
func createResponseDir(t *testing.T, srcDir string) string {
	t.Helper()
	tempDir := t.TempDir()
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("createResponseDir: reading %s: %v", srcDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			t.Fatalf("createResponseDir: reading file %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, e.Name()), data, 0644); err != nil {
			t.Fatalf("createResponseDir: writing file %s: %v", e.Name(), err)
		}
	}
	p := filepath.Join(tempDir, "responses.yml")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		f, err := os.Create(p)
		if err != nil {
			t.Fatalf("createResponseDir: creating responses.yml: %v", err)
		}
		f.Close()
	}
	return tempDir
}

// compact normalises whitespace for string comparison in tests.
func compact(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
