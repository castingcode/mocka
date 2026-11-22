package mocka

import (
	"fmt"
	"log/slog"
	"strings"
)

// matchQuery implements the three-step matching hierarchy defined in
// docs/architecture.md. It is the single entry point for query resolution.
//
// Order:
//  1. Exact match
//  2. Publish-data contextual match (with context, then without)
//  3. Prefix match (longest prefix wins)
//  4. No match → StatusCommandNotFound
func matchQuery(query string, entries []Entry, logger *slog.Logger) Response {
	logger.Debug("matching query", "query", query)

	// 1. Exact match
	for _, e := range entries {
		if e.MatchType == MatchTypeExact && e.Query == query {
			logger.Debug("exact match", "query", query)
			return e.response()
		}
	}

	// 2. Publish-data contextual match
	if pd, ok := parsePublishData(query); ok {
		// Contextual match (entry has context that matches)
		if r, ok := findPublishData(pd, entries, true); ok {
			logger.Debug("publish_data contextual match", "inner", pd.inner)
			return r
		}
		// Generic fallback (entry has no context)
		if r, ok := findPublishData(pd, entries, false); ok {
			logger.Debug("publish_data generic match", "inner", pd.inner)
			return r
		}
	}

	// 3. Prefix match (the longest matching prefix wins)
	var best *Entry
	for i, e := range entries {
		if e.MatchType == MatchTypePrefix && strings.HasPrefix(query, e.Prefix) &&
			(best == nil || len(e.Prefix) > len(best.Prefix)) {
			best = &entries[i]
		}
	}
	if best != nil {
		logger.Debug("prefix match", "prefix", best.Prefix)
		return best.response()
	}

	// 4. No match
	logger.Debug("no match", "query", query)
	return Response{
		StatusCode: StatusCommandNotFound,
		Message:    fmt.Sprintf("Command (%s) not found", query),
	}
}

// publishDataParsed holds the components extracted from a
// "publish data where k=v... | { inner }" query.
type publishDataParsed struct {
	inner   string
	context map[string]string
}

// parsePublishData parses a publish-data contextual query. Returns (parsed,
// true) if the query matches the pattern, (zero, false) otherwise.
func parsePublishData(query string) (publishDataParsed, bool) {
	const pfx = "publish data where "
	if !strings.HasPrefix(query, pfx) {
		return publishDataParsed{}, false
	}
	pipeIdx := strings.Index(query, "| {")
	if pipeIdx < 0 {
		return publishDataParsed{}, false
	}
	whereClause := strings.TrimSpace(query[len(pfx):pipeIdx])
	innerPart := strings.TrimSpace(query[pipeIdx+3:])
	if !strings.HasSuffix(innerPart, "}") {
		return publishDataParsed{}, false
	}
	inner := normalizeQuery(strings.TrimSuffix(innerPart, "}"))

	ctx := make(map[string]string)
	for _, cond := range strings.Split(whereClause, " and ") {
		parts := strings.SplitN(cond, " = ", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), `'"`))
			ctx[key] = val
		}
	}
	return publishDataParsed{inner: inner, context: ctx}, true
}

// findPublishData searches entries for a publish_data match.
// When withContext is true it only considers entries that have context and
// whose context key/value pairs all appear in pd.context (order-insensitive);
// the entry with the most context pairs wins, and the first registered wins a tie.
// When withContext is false it only considers entries without context.
func findPublishData(pd publishDataParsed, entries []Entry, withContext bool) (Response, bool) {
	var best *Entry
	for i, e := range entries {
		if e.MatchType != MatchTypePublishData || e.Inner != pd.inner {
			continue
		}
		hasCtx := len(e.Context) > 0
		if withContext && hasCtx && contextMatches(e.Context, pd.context) &&
			(best == nil || len(e.Context) > len(best.Context)) {
			best = &entries[i]
		}
		if !withContext && !hasCtx {
			return e.response(), true
		}
	}
	if best == nil {
		return Response{}, false
	}
	return best.response(), true
}

// contextMatches returns true if every key/value pair in registered is present
// in parsed (order-insensitive; extra keys in parsed are ignored).
func contextMatches(registered, parsed map[string]string) bool {
	for k, v := range registered {
		if parsed[k] != v {
			return false
		}
	}
	return true
}
