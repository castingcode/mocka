package mocka

import (
	"strings"
	"text/scanner"
)

// normalizeQuery lowercases q, normalizes whitespace inside embedded SQL ([...])
// and Groovy ([[...]]) blocks, canonicalizes double quotes to single quotes in
// local syntax, adds spaces around = signs, and collapses all remaining
// whitespace to single spaces with leading/trailing whitespace trimmed.
// All MOCA query comparisons must go through this function.
func normalizeQuery(q string) string {
	q = strings.ToLower(q)
	q = processQuerySegments(q)
	q = strings.ReplaceAll(q, "=", " = ")
	return strings.Join(strings.Fields(q), " ")
}

// processQuerySegments scans q character by character and applies two
// transformations:
//
//  1. Inside embedded SQL ([...]) and Groovy ([[...]]) blocks: whitespace is
//     trimmed and collapsed, but quotes are preserved as-is.
//
//  2. In local syntax (everywhere else): double quotes are canonicalized to
//     single quotes so that 'foo' and "foo" compare equal.
func processQuerySegments(q string) string {
	var s scanner.Scanner
	s.Init(strings.NewReader(q))
	s.Error = func(_ *scanner.Scanner, _ string) {} // suppress default stderr output

	var out strings.Builder
	for ch := s.Next(); ch != scanner.EOF; ch = s.Next() {
		switch {
		case ch == '[' && s.Peek() == '[':
			s.Next() // consume second '['
			inner := scanUntil(&s, "]]")
			out.WriteString("[[")
			out.WriteString(strings.Join(strings.Fields(inner), " "))
			out.WriteString("]]")
		case ch == '[':
			inner := scanUntil(&s, "]")
			out.WriteString("[")
			out.WriteString(strings.Join(strings.Fields(inner), " "))
			out.WriteString("]")
		case ch == '"':
			// Canonicalize double quotes to single quotes in local syntax.
			out.WriteRune('\'')
		default:
			out.WriteRune(ch)
		}
	}
	return out.String()
}

// scanUntil reads from s until delim is found, returning all characters read
// before the delimiter. The delimiter itself is consumed but not returned.
// delim must be one or two characters.
func scanUntil(s *scanner.Scanner, delim string) string {
	d0 := rune(delim[0])
	twoChar := len(delim) == 2
	d1 := rune(0)
	if twoChar {
		d1 = rune(delim[1])
	}

	var buf strings.Builder
	for {
		ch := s.Next()
		if ch == scanner.EOF {
			break
		}
		if ch == d0 {
			if !twoChar {
				break
			}
			if s.Peek() == d1 {
				s.Next() // consume second delimiter character
				break
			}
		}
		buf.WriteRune(ch)
	}
	return buf.String()
}
