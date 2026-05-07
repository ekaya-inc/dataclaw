package mssql

import (
	"fmt"
	"strings"
)

func ValidateReadOnlyTemplate(sqlQuery string) error {
	if hasTopLevelKeyword(sqlQuery, "TOP") {
		return fmt.Errorf("approved-query template must not include a top-level TOP clause; remove it and use the tool's limit/offset arguments instead")
	}
	if hasTopLevelKeyword(sqlQuery, "OFFSET") || hasTopLevelKeyword(sqlQuery, "FETCH") {
		return fmt.Errorf("approved-query template must not include a top-level OFFSET/FETCH NEXT clause; remove it and use the tool's limit/offset arguments instead")
	}
	if hasTopLevelKeyword(sqlQuery, "LIMIT") {
		return fmt.Errorf("approved-query template must not include LIMIT (not valid T-SQL); use the tool's limit/offset arguments instead")
	}
	if marker, ok := findBareNamedBindMarker(sqlQuery); ok {
		return fmt.Errorf("approved-query template must not include native bind marker %q; use {{parameter_name}} placeholders instead", marker)
	}
	return nil
}

// findBareNamedBindMarker walks sqlQuery looking for `@name` outside of
// strings, identifier delimiters, and comments. Double-`@@` system functions
// (e.g. @@VERSION, @@ROWCOUNT) are accepted.
func findBareNamedBindMarker(sqlQuery string) (string, bool) {
	for i := 0; i < len(sqlQuery); {
		switch {
		case strings.HasPrefix(sqlQuery[i:], "--"):
			i = skipLineComment(sqlQuery, i+2)
		case strings.HasPrefix(sqlQuery[i:], "/*"):
			i = skipBlockComment(sqlQuery, i+2)
		case sqlQuery[i] == '\'':
			i = skipSingleQuotedString(sqlQuery, i+1)
		case sqlQuery[i] == '"':
			i = skipDelimitedIdentifier(sqlQuery, i+1, '"')
		case sqlQuery[i] == '[':
			i = skipBracketIdentifier(sqlQuery, i+1)
		case sqlQuery[i] == '`':
			i = skipDelimitedIdentifier(sqlQuery, i+1, '`')
		case sqlQuery[i] == '@':
			if i+1 < len(sqlQuery) && sqlQuery[i+1] == '@' {
				i += 2
				for i < len(sqlQuery) && isWordPart(sqlQuery[i]) {
					i++
				}
				continue
			}
			start := i
			i++
			if i < len(sqlQuery) && isWordStart(sqlQuery[i]) {
				for i < len(sqlQuery) && isWordPart(sqlQuery[i]) {
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
