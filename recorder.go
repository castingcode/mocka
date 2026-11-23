package mocka

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/castingcode/mocaprotocol"
)

const (
	// recordedFile is where recordings are written, relative to the responses
	// folder. It is in responses.d so the folder serves them without changes.
	recordedFile = "responses.d/recorded.yml"
	// recordedDir holds recorded result and query files.
	recordedDir = "recorded"
)

// RecordingHandler forwards every MOCA request to a real MOCA server, returns
// its response unchanged, and saves the responses to non-built-in commands as
// response files that a FileResponseLoader can serve later.
//
// Recordings go to responses.d/recorded.yml in the responses folder, with
// result XML and multi-line queries in its recorded/ folder. Recording the
// same query again replaces its entry. Queries that already have an exact
// entry in the folder's other response files are not recorded, so the folder
// keeps loading; neither are responses with status 523 (invalid session) or
// 501 (command not found), which mocka returns for unmatched queries anyway.
type RecordingHandler struct {
	upstream string
	folder   string
	client   *http.Client
	logger   *slog.Logger

	mu       sync.Mutex
	file     *managedFile
	existing map[string]bool // normalized exact queries defined outside recordedFile
}

var _ MocaRequestHandlerInterface = (*RecordingHandler)(nil)

// NewRecordingHandler creates a RecordingHandler that forwards requests to
// upstream, the URL of a MOCA server's service endpoint such as
// https://moca.example.com/service, and records into the responses folder.
// It loads the folder first, so invalid response files return an error.
func NewRecordingHandler(upstream, folder string) (*RecordingHandler, error) {
	h := &RecordingHandler{
		upstream: upstream,
		folder:   folder,
		client:   &http.Client{},
		logger:   slog.Default(),
		existing: make(map[string]bool),
	}
	entries, err := NewFileResponseLoader(folder).Load()
	if err != nil {
		return nil, fmt.Errorf("loading responses folder: %w", err)
	}
	for _, e := range entries {
		if e.MatchType == MatchTypeExact && !strings.HasPrefix(e.Source, recordedFile+" ") {
			h.existing[e.Query] = true
		}
	}
	header := "# Recorded by mocka from " + upstream + ". Recording the same query again replaces its entry.\n"
	if h.file, err = loadManagedFile(folder, recordedFile, recordedDir, header); err != nil {
		return nil, fmt.Errorf("loading earlier recordings: %w", err)
	}
	return h, nil
}

// HandleMocaRequest forwards the request to the upstream server, writes its
// response back, and records it.
func (h *RecordingHandler) HandleMocaRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading request body: %v", err), http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.upstream, bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("creating upstream request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("forwarding request to upstream MOCA server", "upstream", h.upstream, "error", err)
		http.Error(w, fmt.Sprintf("forwarding request to %s: %v", h.upstream, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Error("reading upstream response", "upstream", h.upstream, "error", err)
		http.Error(w, fmt.Sprintf("reading response from %s: %v", h.upstream, err), http.StatusBadGateway)
		return
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	if resp.StatusCode != http.StatusOK || r.Header.Get("Content-Type") != "application/moca-xml" {
		return
	}
	if err := h.record(body, respBody); err != nil {
		h.logger.Error("recording response", "error", err)
	}
}

// record saves one request/response pair, unless it should be skipped.
func (h *RecordingHandler) record(reqBody, respBody []byte) error {
	var request mocaprotocol.MocaRequest
	if err := xml.Unmarshal(reqBody, &request); err != nil {
		return fmt.Errorf("parsing MOCA request: %w", err)
	}
	query := strings.TrimSpace(request.Query.Text)
	normalized := normalizeQuery(query)
	if builtinCommand(normalized) != "" {
		return nil
	}
	var response mocaprotocol.MocaResponse
	if err := xml.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("parsing MOCA response to %q: %w", normalized, err)
	}
	if response.Status == StatusInvalidSessionKey || response.Status == StatusCommandNotFound {
		h.logger.Debug("not recording response", "query", normalized, "status", response.Status)
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.existing[normalized] {
		h.logger.Info("not recording query that already has an exact entry", "query", normalized)
		return nil
	}

	entry := rawEntry{
		Match:    matchSpec{Type: string(MatchTypeExact), Query: query},
		RespSpec: responseSpec{Status: response.Status, Message: response.Message},
	}
	var results string
	if len(response.MocaResults.Metadata.Columns) > 0 {
		out, err := xml.MarshalIndent(struct {
			XMLName xml.Name `xml:"moca-results"`
			mocaprotocol.MocaResults
		}{MocaResults: response.MocaResults}, "", "    ")
		if err != nil {
			return fmt.Errorf("encoding results for %q: %w", normalized, err)
		}
		results = string(out)
	}
	if err := h.file.put(entry, results); err != nil {
		return fmt.Errorf("saving recording of %q: %w", normalized, err)
	}
	h.logger.Info("recorded response", "query", normalized, "status", response.Status)
	return nil
}
