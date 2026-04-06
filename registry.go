package mocka

import (
	"log/slog"
)

// --- Registry ---

// ResponseLookup resolves queries to canned responses using a ResponseLoader.
type ResponseLookup struct {
	entries []Entry
	logger  *slog.Logger
}

// NewResponseLookup creates a ResponseLookup by loading entries from loader.
func NewResponseLookup(loader ResponseLoader) (*ResponseLookup, error) {
	entries, err := loader.Load()
	if err != nil {
		return nil, err
	}
	return &ResponseLookup{
		entries: entries,
		logger:  slog.Default(),
	}, nil
}

// GetResponse returns the matching response for the already-normalized query string.
func (r *ResponseLookup) GetResponse(query string) Response {
	return matchQuery(query, r.entries, r.logger)
}
