package postgres

import (
	"fmt"
	"strings"
)

func ValidateReadOnlyTemplate(sqlQuery string) error {
	keywords := scanTopLevelKeywords(sqlQuery)
	for _, kw := range keywords {
		switch kw {
		case "LIMIT":
			return fmt.Errorf("approved-query template must not include a top-level LIMIT clause; remove it and use the tool's limit/offset arguments instead")
		case "OFFSET":
			return fmt.Errorf("approved-query template must not include a top-level OFFSET clause; remove it and use the tool's limit/offset arguments instead")
		}
	}
	if marker, ok := findNumberedBindMarker(sqlQuery); ok {
		return fmt.Errorf("approved-query template must not include native bind marker %q; use {{parameter_name}} placeholders instead", marker)
	}
	return nil
}

// scanTopLevelKeywords collects identifier-shaped tokens that appear at
// parenthesis depth zero, skipping comments, string literals, dollar-quoted
// strings, and quoted identifiers. Tokens are upper-cased for caller comparison.
func scanTopLevelKeywords(sqlQuery string) []string {
	keywords := make([]string, 0, 16)
	depth := 0
	for i := 0; i < len(sqlQuery); {
		switch {
		case isPGWhitespace(sqlQuery[i]) || sqlQuery[i] == ';' || sqlQuery[i] == ',':
			i++
		case strings.HasPrefix(sqlQuery[i:], "--"):
			i = skipPGLineComment(sqlQuery, i+2)
		case strings.HasPrefix(sqlQuery[i:], "/*"):
			i = skipPGBlockComment(sqlQuery, i+2)
		case sqlQuery[i] == '\'':
			i = skipPGSingleQuotedString(sqlQuery, i+1)
		case sqlQuery[i] == '"':
			i = skipPGQuotedIdentifier(sqlQuery, i+1)
		case sqlQuery[i] == '$':
			next, ok := skipPGDollarQuotedString(sqlQuery, i)
			if ok {
				i = next
			} else {
				i++
			}
		case sqlQuery[i] == '(':
			depth++
			i++
		case sqlQuery[i] == ')':
			if depth > 0 {
				depth--
			}
			i++
		default:
			if !isPGWordStart(sqlQuery[i]) {
				i++
				continue
			}
			start := i
			i++
			for i < len(sqlQuery) && isPGWordPart(sqlQuery[i]) {
				i++
			}
			if depth == 0 {
				keywords = append(keywords, strings.ToUpper(sqlQuery[start:i]))
			}
		}
	}
	return keywords
}

// findNumberedBindMarker walks sqlQuery looking for `$N` (digits) outside of
// strings, identifiers, comments, and dollar-quoted bodies. The dollar-quote
// opener `$tag$...$tag$` and `$$...$$` are treated as a single skipped span.
func findNumberedBindMarker(sqlQuery string) (string, bool) {
	for i := 0; i < len(sqlQuery); {
		switch {
		case strings.HasPrefix(sqlQuery[i:], "--"):
			i = skipPGLineComment(sqlQuery, i+2)
		case strings.HasPrefix(sqlQuery[i:], "/*"):
			i = skipPGBlockComment(sqlQuery, i+2)
		case sqlQuery[i] == '\'':
			i = skipPGSingleQuotedString(sqlQuery, i+1)
		case sqlQuery[i] == '"':
			i = skipPGQuotedIdentifier(sqlQuery, i+1)
		case sqlQuery[i] == '$':
			if next, ok := skipPGDollarQuotedString(sqlQuery, i); ok {
				i = next
				continue
			}
			start := i
			i++
			if i < len(sqlQuery) && sqlQuery[i] >= '0' && sqlQuery[i] <= '9' {
				for i < len(sqlQuery) && sqlQuery[i] >= '0' && sqlQuery[i] <= '9' {
					i++
				}
				return sqlQuery[start:i], true
			}
		default:
			i++
		}
	}
	return "", false
}

func isPGWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func isPGWordStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isPGWordPart(char byte) bool {
	return isPGWordStart(char) || (char >= '0' && char <= '9')
}

func skipPGLineComment(sqlQuery string, start int) int {
	for start < len(sqlQuery) && sqlQuery[start] != '\n' {
		start++
	}
	return start
}

func skipPGBlockComment(sqlQuery string, start int) int {
	depth := 1
	for start+1 < len(sqlQuery) {
		switch {
		case sqlQuery[start] == '/' && sqlQuery[start+1] == '*':
			depth++
			start += 2
		case sqlQuery[start] == '*' && sqlQuery[start+1] == '/':
			depth--
			start += 2
			if depth == 0 {
				return start
			}
		default:
			start++
		}
	}
	return len(sqlQuery)
}

func skipPGSingleQuotedString(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == '\'' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == '\'' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipPGQuotedIdentifier(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == '"' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == '"' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

// skipPGDollarQuotedString recognizes `$tag$ ... $tag$` (and `$$ ... $$`).
// Returns the new index past the closing delimiter and true if the cursor
// at start opens a dollar-quoted string. Otherwise returns start and false.
func skipPGDollarQuotedString(sqlQuery string, start int) (int, bool) {
	if start >= len(sqlQuery) || sqlQuery[start] != '$' {
		return start, false
	}
	tagEnd := start + 1
	for tagEnd < len(sqlQuery) && (isPGWordPart(sqlQuery[tagEnd]) || sqlQuery[tagEnd] == '_') {
		tagEnd++
	}
	if tagEnd >= len(sqlQuery) || sqlQuery[tagEnd] != '$' {
		return start, false
	}
	delimiter := sqlQuery[start : tagEnd+1]
	closeIdx := strings.Index(sqlQuery[tagEnd+1:], delimiter)
	if closeIdx < 0 {
		return len(sqlQuery), true
	}
	return tagEnd + 1 + closeIdx + len(delimiter), true
}
