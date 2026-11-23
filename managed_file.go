package mocka

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// managedFile is a response file that mocka writes itself, such as recorded
// responses or responses created in the UI. It sits in responses.d, so the
// folder serves its entries, and keeps their result XML and multi-line
// queries in a folder of its own. Entries are identified by duplicateKey, so
// putting an entry that matches the same queries replaces it.
type managedFile struct {
	folder string // the responses folder
	file   string // relative to folder, such as responses.d/recorded.yml
	dir    string // relative to folder; holds result and query files
	header string // comment written at the top of the file

	entries []rawEntry
	keys    []string // duplicateKey of each entry
}

// loadManagedFile reads the managed file if it exists.
func loadManagedFile(folder, file, dir, header string) (*managedFile, error) {
	m := &managedFile{folder: folder, file: file, dir: dir, header: header}
	data, err := os.ReadFile(filepath.Join(folder, file))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	var f responseFile
	if err := yaml.UnmarshalWithOptions(data, &f, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", file, err)
	}
	for i, r := range f.Responses {
		e, err := matchEntry(r.Match, func(name string) (string, error) {
			raw, err := os.ReadFile(filepath.Join(folder, name))
			return string(raw), err
		})
		if err != nil {
			return nil, fmt.Errorf("%s entry %d: %w", file, i+1, err)
		}
		m.entries = append(m.entries, r)
		m.keys = append(m.keys, duplicateKey(e))
	}
	return m, nil
}

// owns reports whether source, an Entry.Source, refers to this file.
func (m *managedFile) owns(source string) bool {
	return strings.HasPrefix(source, m.file+" ")
}

// put adds r, or replaces the entry that matches the same queries, writing
// results (result XML, may be empty) and any multi-line query to files and
// saving the managed file. r.Match must not use query_file.
func (m *managedFile) put(r rawEntry, results string) error {
	e, err := matchEntry(r.Match, nil)
	if err != nil {
		return fmt.Errorf("invalid match: %w", err)
	}
	key := duplicateKey(e)
	name := managedFileName(entryLabel(e), key)

	if strings.Contains(r.Match.Query, "\n") {
		r.Match.QueryFile = m.dir + "/queries/" + name + ".txt"
		if err := m.writeFile(r.Match.QueryFile, []byte(strings.TrimSpace(r.Match.Query)+"\n")); err != nil {
			return err
		}
		r.Match.Query = ""
	}
	r.RespSpec.Results = ""
	if results != "" {
		r.RespSpec.Results = m.dir + "/" + name + ".xml"
		if err := m.writeFile(r.RespSpec.Results, []byte(strings.TrimSpace(results)+"\n")); err != nil {
			return err
		}
	}

	if i := slices.Index(m.keys, key); i >= 0 {
		if err := m.removeFiles(m.entries[i], r); err != nil {
			return err
		}
		m.entries[i] = r
	} else {
		m.entries = append(m.entries, r)
		m.keys = append(m.keys, key)
	}
	return m.save()
}

// remove deletes the entry with the given duplicateKey and its files. It
// reports whether the entry existed.
func (m *managedFile) remove(key string) (bool, error) {
	i := slices.Index(m.keys, key)
	if i < 0 {
		return false, nil
	}
	if err := m.removeFiles(m.entries[i], rawEntry{}); err != nil {
		return false, err
	}
	m.entries = slices.Delete(m.entries, i, i+1)
	m.keys = slices.Delete(m.keys, i, i+1)
	return true, m.save()
}

// removeFiles deletes the files old uses that replacement does not.
func (m *managedFile) removeFiles(old, replacement rawEntry) error {
	for _, f := range []string{old.Match.QueryFile, old.RespSpec.Results} {
		if f == "" || f == replacement.Match.QueryFile || f == replacement.RespSpec.Results {
			continue
		}
		if err := os.Remove(filepath.Join(m.folder, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", f, err)
		}
	}
	return nil
}

func (m *managedFile) save() error {
	data, err := yaml.Marshal(responseFile{Responses: m.entries})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", m.file, err)
	}
	return m.writeFile(m.file, append([]byte(m.header), data...))
}

// writeFile writes data to name, relative to the responses folder, through a
// temporary file so a reader never sees a partial file.
func (m *managedFile) writeFile(name string, data []byte) error {
	path := filepath.Join(m.folder, name)
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

// entryLabel is the readable part of an entry's file names.
func entryLabel(e Entry) string {
	switch e.MatchType {
	case MatchTypeExact:
		return e.Query
	case MatchTypePrefix:
		return e.Prefix
	default:
		name, pairs := e.contextual()
		keys := make([]string, 0, len(pairs))
		for k := range pairs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			name += " " + k + " " + pairs[k]
		}
		return name
	}
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// managedFileName returns a file name for an entry: a readable slug of its
// label plus a hash of its key, so different entries never share a file and
// putting the same entry again reuses its files.
func managedFileName(label, key string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(label), "-"), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	sum := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(sum[:4])
	if slug == "" {
		return hash
	}
	return slug + "-" + hash
}
