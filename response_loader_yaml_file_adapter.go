package mocka

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// --- YAML-level types (unexported) ---

// matchSpec is the match section of a response, shared by the YAML files and
// the admin API.
type matchSpec struct {
	Type      string            `yaml:"type" json:"type"`
	Query     string            `yaml:"query,omitempty" json:"query,omitempty"`
	QueryFile string            `yaml:"query_file,omitempty" json:"query_file,omitempty"`
	Inner     string            `yaml:"inner,omitempty" json:"inner,omitempty"`
	Context   map[string]string `yaml:"context,omitempty" json:"context,omitempty"`
	Prefix    string            `yaml:"prefix,omitempty" json:"prefix,omitempty"`
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

// Load implements ResponseLoader. It reads responses.yml (or responses.yaml)
// from the data folder, followed by every *.yml and *.yaml file in the
// responses.d subfolder in lexical order. Missing files are treated as an empty
// registry. Malformed or invalid entries, unknown fields, and duplicate matches
// across all files return an error.
func (l *FileResponseLoader) Load() ([]Entry, error) {
	files, err := l.responseFiles()
	if err != nil {
		return nil, fmt.Errorf("finding response files: %w", err)
	}
	var entries []Entry
	seen := make(map[string]string) // duplicate key -> location of first definition
	for _, file := range files {
		fileEntries, err := l.loadResponseFile(file, seen)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", file, err)
		}
		entries = append(entries, fileEntries...)
	}
	return entries, nil
}

// responseFiles returns the response file paths to load, relative to the data
// folder.
func (l *FileResponseLoader) responseFiles() ([]string, error) {
	var files []string
	for _, name := range []string{"responses.yml", "responses.yaml"} {
		_, err := os.Stat(filepath.Join(l.dataFolder, name))
		if err == nil {
			files = append(files, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("checking %s: %w", name, err)
		}
	}
	if len(files) > 1 {
		return nil, fmt.Errorf("both responses.yml and responses.yaml exist; keep only one")
	}
	var extra []string
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(l.dataFolder, "responses.d", pattern))
		if err != nil {
			return nil, fmt.Errorf("listing responses.d: %w", err)
		}
		for _, m := range matches {
			extra = append(extra, filepath.Join("responses.d", filepath.Base(m)))
		}
	}
	slices.Sort(extra)
	return append(files, extra...), nil
}

func (l *FileResponseLoader) loadResponseFile(file string, seen map[string]string) ([]Entry, error) {
	data, err := os.ReadFile(filepath.Join(l.dataFolder, file))
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var f responseFile
	if err := yaml.UnmarshalWithOptions(data, &f, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}
	entries := make([]Entry, 0, len(f.Responses))
	for i, r := range f.Responses {
		e, err := l.buildEntry(r)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}
		location := fmt.Sprintf("%s entry %d", file, i+1)
		key := duplicateKey(e)
		if first, ok := seen[key]; ok {
			return nil, fmt.Errorf("entry %d: duplicate %s match (first defined in %s)", i+1, e.MatchType, first)
		}
		seen[key] = location
		e.Source = location
		entries = append(entries, e)
	}
	return entries, nil
}

// buildEntry validates r, normalizes its query strings, and loads any
// referenced query and XML result files.
func (l *FileResponseLoader) buildEntry(r rawEntry) (Entry, error) {
	e, err := matchEntry(r.Match, func(name string) (string, error) {
		raw, err := os.ReadFile(filepath.Join(l.dataFolder, name))
		return string(raw), err
	})
	if err != nil {
		return Entry{}, err
	}
	e.StatusCode = r.RespSpec.Status
	e.Message = r.RespSpec.Message
	if r.RespSpec.Results != "" {
		xmlRaw, err := os.ReadFile(filepath.Join(l.dataFolder, r.RespSpec.Results))
		if err != nil {
			return Entry{}, fmt.Errorf("reading results file %s: %w", r.RespSpec.Results, err)
		}
		e.ResultSet = strings.TrimSpace(string(xmlRaw))
	}
	return e, nil
}

// matchEntry validates m and returns an Entry with its normalized match
// fields set. readQueryFile reads a query_file; when it is nil, query_file is
// rejected.
func matchEntry(m matchSpec, readQueryFile func(name string) (string, error)) (Entry, error) {
	e := Entry{MatchType: MatchType(m.Type)}
	// used lists the match fields that are set, so each type can reject the
	// fields that belong to other types.
	used := map[string]bool{
		"query":      m.Query != "",
		"query_file": m.QueryFile != "",
		"inner":      m.Inner != "",
		"context":    len(m.Context) > 0,
		"prefix":     m.Prefix != "",
	}
	var allowed []string
	switch e.MatchType {
	case MatchTypeExact:
		allowed = []string{"query", "query_file"}
		switch {
		case used["query"] && used["query_file"]:
			return Entry{}, fmt.Errorf("exact match sets both query and query_file; use one")
		case used["query_file"] && readQueryFile == nil:
			return Entry{}, fmt.Errorf("query_file is not supported here; send the query text in query")
		case used["query_file"]:
			raw, err := readQueryFile(m.QueryFile)
			if err != nil {
				return Entry{}, fmt.Errorf("reading query_file %s: %w", m.QueryFile, err)
			}
			e.Query = normalizeQuery(raw)
		default:
			e.Query = normalizeQuery(m.Query)
		}
		if e.Query == "" {
			return Entry{}, fmt.Errorf("exact match needs a non-empty query or query_file")
		}
	case MatchTypePublishData:
		allowed = []string{"inner", "context"}
		e.Inner = normalizeQuery(m.Inner)
		if e.Inner == "" {
			return Entry{}, fmt.Errorf("publish_data match needs a non-empty inner")
		}
		// Normalize context values to lowercase for consistent comparison
		// with values extracted from the normalized (lowercased) incoming query.
		e.Context = make(map[string]string)
		for k, v := range m.Context {
			e.Context[k] = strings.ToLower(v)
		}
	case MatchTypePrefix:
		allowed = []string{"prefix"}
		e.Prefix = normalizeQuery(m.Prefix)
		if e.Prefix == "" {
			return Entry{}, fmt.Errorf("prefix match needs a non-empty prefix")
		}
	default:
		return Entry{}, fmt.Errorf("unknown match type %q (want exact, publish_data, or prefix)", m.Type)
	}
	for _, field := range []string{"query", "query_file", "inner", "context", "prefix"} {
		if used[field] && !slices.Contains(allowed, field) {
			return Entry{}, fmt.Errorf("%s is not used by %s matches", field, e.MatchType)
		}
	}
	return e, nil
}

// duplicateKey identifies what an entry matches, so two entries that match the
// same queries produce the same key.
func duplicateKey(e Entry) string {
	switch e.MatchType {
	case MatchTypeExact:
		return "exact\x00" + e.Query
	case MatchTypePrefix:
		return "prefix\x00" + e.Prefix
	default:
		pairs := make([]string, 0, len(e.Context))
		for k, v := range e.Context {
			pairs = append(pairs, k+"="+v)
		}
		slices.Sort(pairs)
		return "publish_data\x00" + e.Inner + "\x00" + strings.Join(pairs, "\x00")
	}
}
