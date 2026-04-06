package mocka

import (
	"strings"
)

// InMemoryResponseLoaderOption configures an InMemoryResponseLoader.
type InMemoryResponseLoaderOption func(*InMemoryResponseLoader)

// InMemoryResponseLoader loads response entries from an in-memory slice.
// It is intended for use by projects that import mocka as a test dependency
// and need to register canned responses programmatically alongside httptest.
type InMemoryResponseLoader struct {
	entries []Entry
}

var _ ResponseLoader = (*InMemoryResponseLoader)(nil)

// NewInMemoryResponseLoader creates an InMemoryResponseLoader configured by
// the given options. Options are applied in order; all entry-producing options
// append to the entry list.
func NewInMemoryResponseLoader(opts ...InMemoryResponseLoaderOption) *InMemoryResponseLoader {
	l := &InMemoryResponseLoader{}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load implements ResponseLoader by returning the in-memory entries.
func (l *InMemoryResponseLoader) Load() ([]Entry, error) {
	return l.entries, nil
}

// WithEntries appends a pre-built slice of entries to the loader.
func WithEntries(entries []Entry) InMemoryResponseLoaderOption {
	return func(l *InMemoryResponseLoader) {
		l.entries = append(l.entries, entries...)
	}
}

// WithExactMatch appends an exact-match entry for the given query.
// The query is normalized before storage.
func WithExactMatch(query string, resp Response) InMemoryResponseLoaderOption {
	return func(l *InMemoryResponseLoader) {
		l.entries = append(l.entries, Entry{
			MatchType:  MatchTypeExact,
			Query:      normalizeQuery(query),
			StatusCode: resp.StatusCode,
			Message:    resp.Message,
			ResultSet:  resp.ResultSet,
		})
	}
}

// WithPrefixMatch appends a prefix-match entry.
// The prefix is normalized before storage.
func WithPrefixMatch(prefix string, resp Response) InMemoryResponseLoaderOption {
	return func(l *InMemoryResponseLoader) {
		l.entries = append(l.entries, Entry{
			MatchType:  MatchTypePrefix,
			Prefix:     normalizeQuery(prefix),
			StatusCode: resp.StatusCode,
			Message:    resp.Message,
			ResultSet:  resp.ResultSet,
		})
	}
}

// WithPublishDataMatch appends a generic publish-data entry with no context,
// matching any publish-data query whose inner command equals inner regardless
// of what context keys the query carries.
// The inner command is normalized before storage.
func WithPublishDataMatch(inner string, resp Response) InMemoryResponseLoaderOption {
	return func(l *InMemoryResponseLoader) {
		l.entries = append(l.entries, Entry{
			MatchType:  MatchTypePublishData,
			Inner:      normalizeQuery(inner),
			StatusCode: resp.StatusCode,
			Message:    resp.Message,
			ResultSet:  resp.ResultSet,
		})
	}
}

// WithContextualPublishDataMatch appends a contextual publish-data entry that
// only matches when the incoming query carries all of the specified context
// key/value pairs. Context values are lowercased for consistent comparison
// with the normalized incoming query.
// The inner command is normalized before storage.
func WithContextualPublishDataMatch(inner string, context map[string]string, resp Response) InMemoryResponseLoaderOption {
	normalized := make(map[string]string, len(context))
	for k, v := range context {
		normalized[k] = strings.ToLower(v)
	}
	return func(l *InMemoryResponseLoader) {
		l.entries = append(l.entries, Entry{
			MatchType:  MatchTypePublishData,
			Inner:      normalizeQuery(inner),
			Context:    normalized,
			StatusCode: resp.StatusCode,
			Message:    resp.Message,
			ResultSet:  resp.ResultSet,
		})
	}
}

