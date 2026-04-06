package mocka

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// --- YAML-level types (unexported) ---

type matchSpec struct {
	Type      string            `yaml:"type"`
	Query     string            `yaml:"query,omitempty"`
	QueryFile string            `yaml:"query_file,omitempty"`
	Inner     string            `yaml:"inner,omitempty"`
	Context   map[string]string `yaml:"context,omitempty"`
	Prefix    string            `yaml:"prefix,omitempty"`
}

type responseSpec struct {
	Status  int    `yaml:"status"`
	Message string `yaml:"message,omitempty"`
	Results string `yaml:"results,omitempty"` // path to XML file, relative to data folder
}

type rawEntry struct {
	Match    matchSpec    `yaml:"match"`
	RespSpec responseSpec `yaml:"response"`
}

type responseFile struct {
	Responses []rawEntry `yaml:"responses"`
}

// --- FileResponseLoader ---

// FileResponseLoader loads responses from YAML files on disk.
type FileResponseLoader struct {
	dataFolder string
}

var _ ResponseLoader = (*FileResponseLoader)(nil)

// NewFileResponseLoader creates a FileResponseLoader that reads YAML response
// files from the given directory.
func NewFileResponseLoader(dataFolder string) *FileResponseLoader {
	return &FileResponseLoader{dataFolder: dataFolder}
}

// Load implements ResponseLoader by reading responses.yml from the data folder.
// A missing file is treated as an empty registry; malformed files return an error.
func (l *FileResponseLoader) Load() ([]Entry, error) {
	entries, err := loadResponseFile(filepath.Join(l.dataFolder, "responses.yml"), l.dataFolder)
	if err != nil {
		return nil, fmt.Errorf("loading responses.yml: %w", err)
	}
	return entries, nil
}

func loadResponseFile(path, dataFolder string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var f responseFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return buildEntries(f.Responses, dataFolder)
}

// buildEntries normalizes all query strings and loads any referenced XML result
// files, returning a slice of ready-to-match Entry values.
func buildEntries(raws []rawEntry, dataFolder string) ([]Entry, error) {
	entries := make([]Entry, 0, len(raws))
	for _, r := range raws {
		e := Entry{
			MatchType:  MatchType(r.Match.Type),
			StatusCode: r.RespSpec.Status,
			Message:    r.RespSpec.Message,
		}
		switch MatchType(r.Match.Type) {
		case MatchTypeExact:
			if r.Match.QueryFile != "" {
				raw, err := os.ReadFile(filepath.Join(dataFolder, r.Match.QueryFile))
				if err != nil {
					return nil, fmt.Errorf("reading query_file %s: %w", r.Match.QueryFile, err)
				}
				e.Query = normalizeQuery(string(raw))
			} else {
				e.Query = normalizeQuery(r.Match.Query)
			}
		case MatchTypePublishData:
			e.Inner = normalizeQuery(r.Match.Inner)
			// Normalize context values to lowercase for consistent comparison
			// with values extracted from the normalized (lowercased) incoming query.
			e.Context = make(map[string]string)
			for k, v := range r.Match.Context {
				e.Context[k] = strings.ToLower(v)
			}
		case MatchTypePrefix:
			e.Prefix = normalizeQuery(r.Match.Prefix)
		}
		if r.RespSpec.Results != "" {
			xmlRaw, err := os.ReadFile(filepath.Join(dataFolder, r.RespSpec.Results))
			if err != nil {
				return nil, fmt.Errorf("reading results file %s: %w", r.RespSpec.Results, err)
			}
			e.ResultSet = strings.TrimSpace(string(xmlRaw))
		}
		entries = append(entries, e)
	}
	return entries, nil
}
