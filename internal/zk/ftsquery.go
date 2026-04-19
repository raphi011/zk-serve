package zk

import "strings"

// ConvertQuery transforms a Google-like search string into an FTS5 MATCH
// expression. Reimplemented from scratch based on zk's query semantics.
//
// Rules:
//   - bare terms are quoted:        foo       → "foo"
//   - quoted phrases preserved:     "foo bar" → "foo bar"
//   - prefix wildcard:              foo*      → "foo"*
//   - negation:                     -foo      → NOT "foo"
//   - pipe = OR:                    foo|bar   → "foo" OR "bar"
//   - AND / OR / NOT pass through as operators
//   - column prefix:                title:foo → title:"foo"
func ConvertQuery(input string) string {
	var out strings.Builder
	var term strings.Builder
	inQuote := false

	writeSpace := func() {
		if out.Len() > 0 {
			out.WriteByte(' ')
		}
	}

	flushQuoted := func() {
		t := term.String()
		term.Reset()
		if t == "" {
			return
		}
		writeSpace()
		out.WriteByte('"')
		out.WriteString(t)
		out.WriteByte('"')
	}

	flushTerm := func() {
		t := term.String()
		term.Reset()
		if t == "" {
			return
		}

		if t == "AND" || t == "OR" || t == "NOT" {
			writeSpace()
			out.WriteString(t)
			return
		}

		negated := false
		if strings.HasPrefix(t, "-") {
			negated = true
			t = t[1:]
			if t == "" {
				return
			}
		}

		prefix := false
		if strings.HasSuffix(t, "*") {
			prefix = true
			t = t[:len(t)-1]
		}

		col := ""
		if idx := strings.IndexByte(t, ':'); idx > 0 {
			col = t[:idx+1]
			t = t[idx+1:]
		}

		writeSpace()
		if negated {
			out.WriteString("NOT ")
		}
		out.WriteString(col)
		out.WriteByte('"')
		out.WriteString(t)
		out.WriteByte('"')
		if prefix {
			out.WriteByte('*')
		}
	}

	for _, r := range input {
		switch {
		case r == '"':
			if inQuote {
				flushQuoted()
				inQuote = false
			} else {
				flushTerm()
				inQuote = true
			}
		case r == '|' && !inQuote:
			flushTerm()
			writeSpace()
			out.WriteString("OR")
		case r == ' ' && !inQuote:
			flushTerm()
		default:
			term.WriteRune(r)
		}
	}

	if inQuote {
		// Unclosed quote — flush remaining content as unquoted terms.
		flushTerm()
	} else {
		flushTerm()
	}

	return out.String()
}
