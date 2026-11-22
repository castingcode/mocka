package mocka

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/castingcode/mocaprotocol"
	"github.com/goccy/go-yaml"
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
	recorded []rawEntry      // recordedFile entries, in order
	index    map[string]int  // normalized query -> index in recorded
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
		index:    make(map[string]int),
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
	if err := h.loadRecorded(); err != nil {
		return nil, fmt.Errorf("loading %s: %w", recordedFile, err)
	}
	return h, nil
}

// loadRecorded reads earlier recordings so new ones are added to them.
func (h *RecordingHandler) loadRecorded() error {
	data, err := os.ReadFile(filepath.Join(h.folder, recordedFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	var f responseFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}
	for _, r := range f.Responses {
		query := r.Match.Query
		if r.Match.QueryFile != "" {
			raw, err := os.ReadFile(filepath.Join(h.folder, r.Match.QueryFile))
			if err != nil {
				return fmt.Errorf("reading query_file %s: %w", r.Match.QueryFile, err)
			}
			query = string(raw)
		}
		h.index[normalizeQuery(query)] = len(h.recorded)
		h.recorded = append(h.recorded, r)
	}
	return nil
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

	name := recordingName(normalized)
	entry := rawEntry{
		Match:    matchSpec{Type: string(MatchTypeExact), Query: query},
		RespSpec: responseSpec{Status: response.Status, Message: response.Message},
	}
	if strings.Contains(query, "\n") {
		entry.Match.Query = ""
		entry.Match.QueryFile = recordedDir + "/queries/" + name + ".txt"
		if err := h.writeFile(entry.Match.QueryFile, []byte(query+"\n")); err != nil {
			return err
		}
	}
	if len(response.MocaResults.Metadata.Columns) > 0 {
		out, err := xml.MarshalIndent(struct {
			XMLName xml.Name `xml:"moca-results"`
			mocaprotocol.MocaResults
		}{MocaResults: response.MocaResults}, "", "    ")
		if err != nil {
			return fmt.Errorf("encoding results for %q: %w", normalized, err)
		}
		entry.RespSpec.Results = recordedDir + "/" + name + ".xml"
		if err := h.writeFile(entry.RespSpec.Results, append(out, '\n')); err != nil {
			return err
		}
	}

	if i, ok := h.index[normalized]; ok {
		// Remove files the earlier recording used that this one does not.
		old := h.recorded[i]
		for _, f := range []string{old.Match.QueryFile, old.RespSpec.Results} {
			if f != "" && f != entry.Match.QueryFile && f != entry.RespSpec.Results {
				if err := os.Remove(filepath.Join(h.folder, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("removing %s: %w", f, err)
				}
			}
		}
		h.recorded[i] = entry
	} else {
		h.index[normalized] = len(h.recorded)
		h.recorded = append(h.recorded, entry)
	}
	data, err := yaml.Marshal(responseFile{Responses: h.recorded})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", recordedFile, err)
	}
	header := "# Recorded by mocka from " + h.upstream + ". Recording the same query again replaces its entry.\n"
	if err := h.writeFile(recordedFile, append([]byte(header), data...)); err != nil {
		return err
	}
	h.logger.Info("recorded response", "query", normalized, "status", response.Status, "results", entry.RespSpec.Results)
	return nil
}

// writeFile writes data to name, relative to the responses folder, through a
// temporary file so a reader never sees a partial file.
func (h *RecordingHandler) writeFile(name string, data []byte) error {
	path := filepath.Join(h.folder, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating folder for %s: %w", name, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replacing %s: %w", name, err)
	}
	return nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// recordingName returns a file name for a normalized query: a readable slug of
// its start plus a hash of the whole query, so different queries never share
// a file and recording the same query again reuses its files.
func recordingName(normalized string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(normalized, "-"), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	sum := sha256.Sum256([]byte(normalized))
	hash := hex.EncodeToString(sum[:4])
	if slug == "" {
		return hash
	}
	return slug + "-" + hash
}
