package mocka

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// matchQuery implements the matching hierarchy defined in
// docs/architecture.md. It is the single entry point for query resolution.
//
// Order:
//  1. Exact match
//  2. Publish-data match (most matching context pairs wins; no context is the fallback)
//  3. Command match (most matching args wins; no args is the fallback)
//  4. Prefix match (longest prefix wins)
//  5. No match → StatusCommandNotFound
func matchQuery(query string, entries []Entry, logger *slog.Logger) Response {
	r, _ := resolveQuery(query, entries, logger)
	return r
}

// resolveQuery is matchQuery that also returns the matched entry, or nil when
// nothing matched.
func resolveQuery(query string, entries []Entry, logger *slog.Logger) (Response, *Entry) {
	logger.Debug("matching query", "query", query)

	// 1. Exact match
	for i, e := range entries {
		if e.MatchType == MatchTypeExact && e.Query == query {
			logger.Debug("exact match", "query", query)
			return e.response(), &entries[i]
		}
	}

	// 2. Publish-data match
	if pd, ok := parsePublishData(query); ok {
		if e := findMostSpecific(entries, MatchTypePublishData, pd.inner, pd.context); e != nil {
			logger.Debug("publish_data match", "inner", pd.inner, "context", e.Context)
			return e.response(), e
		}
	}

	// 3. Command match
	if cmd, ok := parseCommand(query); ok {
		if e := findMostSpecific(entries, MatchTypeCommand, cmd.name, cmd.args); e != nil {
			logger.Debug("command match", "command", cmd.name, "args", e.Args)
			return e.response(), e
		}
	}

	// 4. Prefix match (the longest matching prefix wins)
	var best *Entry
	for i, e := range entries {
		if e.MatchType == MatchTypePrefix && strings.HasPrefix(query, e.Prefix) &&
			(best == nil || len(e.Prefix) > len(best.Prefix)) {
			best = &entries[i]
		}
	}
	if best != nil {
		logger.Debug("prefix match", "prefix", best.Prefix)
		return best.response(), best
	}

	// 5. No match
	logger.Debug("no match", "query", query)
	return Response{
		StatusCode: StatusCommandNotFound,
		Message:    fmt.Sprintf("Command (%s) not found", query),
	}, nil
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

// findMostSpecific returns the entry of type t whose name is name and whose
// key/value pairs all appear in got (order-insensitive; extra pairs in got are
// ignored), or nil. The entry with the most pairs wins, so an entry without
// pairs is the fallback; the first registered wins a tie.
func findMostSpecific(entries []Entry, t MatchType, name string, got map[string]string) *Entry {
	var best *Entry
	bestPairs := -1
	for i := range entries {
		e := &entries[i]
		if e.MatchType != t {
			continue
		}
		entryName, pairs := e.contextual()
		if entryName == name && pairsMatch(pairs, got) && len(pairs) > bestPairs {
			best, bestPairs = e, len(pairs)
		}
	}
	return best
}

// pairsMatch returns true if every key/value pair in registered is present in
// got.
func pairsMatch(registered, got map[string]string) bool {
	for k, v := range registered {
		if gv, ok := got[k]; !ok || gv != v {
			return false
		}
	}
	return true
}

// parsedCommand holds a single local-syntax command and its arguments.
type parsedCommand struct {
	name string
	args map[string]string
}

var (
	commandName = regexp.MustCompile(`^[a-z0-9_]+( [a-z0-9_]+)*$`)
	argName     = regexp.MustCompile(`^@?[a-z0-9_.]+$`)
)

// parseCommand parses a normalized query of the form
// "<command> [where <name> = <value> [and ...]]". It returns false for
// anything else, such as pipelines, command blocks, SQL, or Groovy.
// Conditions that are not "<name> = <value>" are ignored; quoted values are
// unquoted.
func parseCommand(query string) (parsedCommand, bool) {
	name, where, hasWhere := strings.Cut(query, " where ")
	if !commandName.MatchString(name) {
		return parsedCommand{}, false
	}
	cmd := parsedCommand{name: name, args: make(map[string]string)}
	if !hasWhere {
		return cmd, true
	}
	conds, ok := splitConditions(where)
	if !ok {
		return parsedCommand{}, false
	}
	for _, cond := range conds {
		k, v, found := strings.Cut(cond, " = ")
		if found && argName.MatchString(k) {
			cmd.args[k] = unquoteValue(v)
		}
	}
	return cmd, true
}

// splitConditions splits a where clause on " and " outside single quotes.
// It returns false when the clause continues into another command.
func splitConditions(where string) ([]string, bool) {
	var conds []string
	start, inQuote := 0, false
	for i := 0; i < len(where); i++ {
		switch c := where[i]; {
		case c == '\'':
			inQuote = !inQuote // a doubled '' inside a string toggles twice
		case inQuote:
		case strings.ContainsRune("|;&{}[]", rune(c)):
			return nil, false
		case strings.HasPrefix(where[i:], " and "):
			conds = append(conds, where[start:i])
			i += len(" and ") - 1
			start = i + 1
		}
	}
	if inQuote {
		return nil, false
	}
	return append(conds, where[start:]), true
}

// unquoteValue strips the single quotes around a MOCA string value and
// unescapes doubled quotes inside it. Other values are returned unchanged.
func unquoteValue(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}
