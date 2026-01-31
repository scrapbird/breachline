package app

import (
	"strings"
	"time"
	"unicode"

	"breachline/app/settings"
	"breachline/app/timestamps"
)

// term represents a single parsed query component
type term struct {
	field string // empty means search across all fields
	op    string // = or != or contains or prefix or exists or annotated
	value string
	neg   bool
}

// splitPipesTopLevel splits a string into stages by '|' outside of quotes.
func splitPipesTopLevel(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		if inQuote == 0 && r == '|' {
			// end of stage
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

// parseColumnsStage extracts a 'columns' stage from the query, returning the
// selected column indices and the cleaned query without the columns stage.
// Syntax example: columns colA, colB, colC
func parseColumnsStage(query string, header []string) ([]int, string) {
	stages := splitPipesTopLevel(query)
	if len(stages) == 0 {
		return nil, query
	}

	// Build header lookup (case-insensitive)
	idxMap := make(map[string]int, len(header))
	for i, h := range header {
		idxMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	var colIdxs []int
	var kept []string
	for _, st := range stages {
		if st == "" {
			continue
		}
		toks := splitRespectingQuotes(st)
		if len(toks) > 0 && strings.EqualFold(toks[0], "columns") {
			// Collect the rest of tokens as the columns spec string
			spec := strings.TrimSpace(strings.Join(toks[1:], " "))
			// Split by comma
			parts := strings.Split(spec, ",")
			for _, p := range parts {
				name := strings.ToLower(strings.TrimSpace(unquoteIfQuoted(p)))
				if name == "" {
					continue
				}
				if idx, ok := idxMap[name]; ok {
					colIdxs = append(colIdxs, idx)
				}
			}
			// Do not keep this stage in the cleaned query
			continue
		}
		kept = append(kept, st)
	}

	cleaned := strings.TrimSpace(strings.Join(kept, " | "))
	if len(colIdxs) == 0 {
		return nil, cleaned
	}
	return colIdxs, cleaned
}

// extractTimeFilters scans the query for special time filters: "after VALUE" and/or "before VALUE" (space-separated only).
// It returns pointers to epoch ms values if present and the cleaned query with those tokens removed.
func extractTimeFilters(query string) (after *int64, before *int64, cleaned string) {
	toks := splitRespectingQuotes(query)
	if len(toks) == 0 {
		return nil, nil, strings.TrimSpace(query)
	}
	var out []string
	now := time.Now()
	// Use DisplayTimezone for timezone-less absolute timestamps in SPL filters
	eff := settings.GetEffectiveSettings()
	loc := timestamps.GetLocationForTZ(eff.DisplayTimezone)
	i := 0
	for i < len(toks) {
		t := toks[i]
		tl := strings.ToLower(strings.TrimSpace(t))
		// Handle spaced form: after VALUE
		if tl == "after" && i+1 < len(toks) {
			val := strings.TrimSpace(unquoteIfQuoted(toks[i+1]))
			if ms, ok := timestamps.ParseFlexibleTime(val, now, loc); ok {
				after = &ms
				i += 2
				continue
			}
			// If value is invalid, keep tokens as-is
		} else if tl == "before" && i+1 < len(toks) {
			val := strings.TrimSpace(unquoteIfQuoted(toks[i+1]))
			if ms, ok := timestamps.ParseFlexibleTime(val, now, loc); ok {
				before = &ms
				i += 2
				continue
			}
		}
		// default: keep token
		out = append(out, t)
		i++
	}
	cleaned = strings.TrimSpace(strings.Join(out, " "))
	return
}

// unquoteIfQuoted removes matching single or double quotes around a string, if present.
func unquoteIfQuoted(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// splitRespectingQuotes splits by whitespace outside quotes and also treats '|' as a separator
// when outside quotes, to support simple piping syntax.
func splitRespectingQuotes(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		// Treat '|' as a separator (like whitespace) when outside quotes, to support piping syntax
		if inQuote == 0 && (unicode.IsSpace(r) || r == '|') {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
