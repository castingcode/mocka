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
	logger := slog.Default()
	if len(entries) == 0 {
		logger.Warn("no responses loaded; every non-built-in query will return command not found")
	} else {
		logger.Info("responses loaded", "count", len(entries))
	}
	return &ResponseLookup{
		entries: entries,
		logger:  logger,
	}, nil
}

// GetResponse returns the matching response for the already-normalized query string.
func (r *ResponseLookup) GetResponse(query string) Response {
	return matchQuery(query, r.entries, r.logger)
}
