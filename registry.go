package mocka

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
)

// --- Registry ---

// ResponseLookup resolves queries to canned responses using a ResponseLoader.
// The admin API can add, replace, and remove entries at runtime and reload
// them from the loader, so access is guarded by a mutex.
type ResponseLookup struct {
	mu      sync.RWMutex
	loader  ResponseLoader
	entries []Entry
	nextID  int // last ID assigned; IDs are never reused, even across reloads
	logger  *slog.Logger
}

// NewResponseLookup creates a ResponseLookup by loading entries from loader.
func NewResponseLookup(loader ResponseLoader) (*ResponseLookup, error) {
	r := &ResponseLookup{loader: loader, logger: slog.Default()}
	if err := r.reload(); err != nil {
		return nil, err
	}
	if len(r.entries) == 0 {
		r.logger.Warn("no responses loaded; every non-built-in query will return command not found")
	} else {
		r.logger.Info("responses loaded", "count", len(r.entries))
	}
	return r, nil
}

// GetResponse returns the matching response for the already-normalized query string.
func (r *ResponseLookup) GetResponse(query string) Response {
	resp, _ := r.resolve(query)
	return resp
}

// resolve returns the matching response and a copy of the entry that produced
// it, or a nil entry when nothing matched.
func (r *ResponseLookup) resolve(query string) (Response, *Entry) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resp, e := resolveQuery(query, r.entries, r.logger)
	if e == nil {
		return resp, nil
	}
	cp := *e
	return resp, &cp
}

// reload replaces all entries, including runtime ones, with a fresh load from
// the loader. On error the current entries are kept.
func (r *ResponseLookup) reload() error {
	loaded, err := r.loader.Load()
	if err != nil {
		return err
	}
	// Clone so runtime changes never write into a loader's own slice, such as
	// the one InMemoryResponseLoader returns on every Load.
	entries := slices.Clone(loaded)
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range entries {
		r.nextID++
		entries[i].id = r.nextID
	}
	r.entries = entries
	return nil
}

// list returns a copy of the current entries in match order.
func (r *ResponseLookup) list() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.entries)
}

// put adds e, or replaces the entry that matches the same queries in place.
// It returns the stored entry, with its new ID, and whether it replaced one.
func (r *ResponseLookup) put(e Entry) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	e.id = r.nextID
	key := duplicateKey(e)
	for i := range r.entries {
		if duplicateKey(r.entries[i]) == key {
			r.entries[i] = e
			return e, true
		}
	}
	r.entries = append(r.entries, e)
	return e, false
}

// remove deletes the entry with the given ID.
func (r *ResponseLookup) remove(id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.IndexFunc(r.entries, func(e Entry) bool { return e.id == id })
	if i < 0 {
		return fmt.Errorf("no response with id %d", id)
	}
	r.entries = slices.Delete(r.entries, i, i+1)
	return nil
}
