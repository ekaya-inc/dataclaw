// Package sql provides SQL validation utilities.
package sql

import (
	"errors"
	"strings"
)

var (
	// ErrMultipleStatements indicates the query contains multiple SQL statements.
	ErrMultipleStatements = errors.New("multiple SQL statements not allowed; only single statements are permitted")
)

// ValidationResult contains the normalized SQL and any validation errors.
type ValidationResult struct {
	NormalizedSQL string
	Error         error
}

type topLevelKeyword struct {
	Text string
	Pos  int
}

// ValidateAndNormalize checks SQL for multiple statements and strips the trailing semicolon.
//
// The validation order is:
// 1. Strip trailing semicolon and whitespace (normalize)
// 2. Check for multiple statements (any remaining semicolons outside string literals)
func ValidateAndNormalize(sqlQuery string) ValidationResult {
	normalized := NormalizeStatement(sqlQuery)
	if normalized == "" {
		return ValidationResult{NormalizedSQL: normalized}
	}

	if err := detectMultipleStatements(normalized); err != nil {
		return ValidationResult{Error: err}
	}

	return ValidationResult{NormalizedSQL: normalized}
}

// NormalizeStatement trims surrounding whitespace and removes a trailing semicolon.
func NormalizeStatement(sqlQuery string) string {
	sqlQuery = strings.TrimSpace(sqlQuery)
	if sqlQuery == "" {
		return sqlQuery
	}

	return stripTrailingSemicolon(sqlQuery)
}

// detectMultipleStatements checks if the SQL contains multiple statements
// by looking for any semicolons outside of quoted/delimited sections.
// Since we've already stripped the trailing semicolon, any remaining semicolon
// indicates multiple statements.
func detectMultipleStatements(sqlQuery string) error {
	semicolons := findSemicolonsOutsideDelimitedSections(sqlQuery)
	if len(semicolons) == 0 {
		return nil
	}
	if allowsRoutineDefinitionSemicolons(sqlQuery, semicolons) {
		return nil
	}
	if hasSemicolonOutsideStrings(sqlQuery) {
		return ErrMultipleStatements
	}
	return nil
}

// hasSemicolonOutsideStrings returns true if the SQL contains any semicolon
// outside of quoted/delimited sections.
func hasSemicolonOutsideStrings(sqlQuery string) bool {
	return len(findSemicolonsOutsideDelimitedSections(sqlQuery)) > 0
}

func findSemicolonsOutsideDelimitedSections(sqlQuery string) []int {
	positions := make([]int, 0, 2)
	for i := 0; i < len(sqlQuery); {
		switch {
		case isWhitespace(sqlQuery[i]):
			i++
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
		case sqlQuery[i] == '$':
			next, ok := skipDollarQuotedString(sqlQuery, i)
			if ok {
				i = next
				continue
			}
			i++
		case sqlQuery[i] == ';':
			positions = append(positions, i)
			i++
		default:
			i++
		}
	}
	return positions
}

func allowsRoutineDefinitionSemicolons(sqlQuery string, semicolonPositions []int) bool {
	keywords := scanTopLevelKeywords(sqlQuery)
	bodyStart, ok := routineDefinitionBodyStart(keywords)
	if !ok {
		return false
	}
	for _, position := range semicolonPositions {
		if position < bodyStart {
			return false
		}
	}
	return true
}

func routineDefinitionBodyStart(keywords []topLevelKeyword) (int, bool) {
	if len(keywords) < 2 {
		return 0, false
	}
	statementKindIndex := -1
	switch keywords[0].Text {
	case "ALTER":
		if isRoutineDefinitionKeyword(keywords[1].Text) {
			statementKindIndex = 1
		}
	case "CREATE":
		i := 1
		if i+1 < len(keywords) && keywords[i].Text == "OR" && (keywords[i+1].Text == "ALTER" || keywords[i+1].Text == "REPLACE") {
			i += 2
		}
		if i < len(keywords) && isRoutineDefinitionKeyword(keywords[i].Text) {
			statementKindIndex = i
		}
	}
	if statementKindIndex < 0 {
		return 0, false
	}
	for i := statementKindIndex + 1; i < len(keywords); i++ {
		if keywords[i].Text == "AS" || keywords[i].Text == "BEGIN" {
			return keywords[i].Pos, true
		}
	}
	return 0, false
}

func isRoutineDefinitionKeyword(keyword string) bool {
	switch keyword {
	case "PROC", "PROCEDURE", "FUNCTION", "TRIGGER":
		return true
	default:
		return false
	}
}

func scanTopLevelKeywords(sqlQuery string) []topLevelKeyword {
	keywords := make([]topLevelKeyword, 0, 16)
	depth := 0
	for i := 0; i < len(sqlQuery); {
		switch {
		case isWhitespace(sqlQuery[i]):
			i++
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
		case sqlQuery[i] == '$':
			next, ok := skipDollarQuotedString(sqlQuery, i)
			if ok {
				i = next
				continue
			}
			i++
		case sqlQuery[i] == '(':
			depth++
			i++
		case sqlQuery[i] == ')':
			if depth > 0 {
				depth--
			}
			i++
		case isWordStart(sqlQuery[i]):
			start := i
			i++
			for i < len(sqlQuery) && isWordPart(sqlQuery[i]) {
				i++
			}
			if depth == 0 {
				keywords = append(keywords, topLevelKeyword{
					Text: strings.ToUpper(sqlQuery[start:i]),
					Pos:  start,
				})
			}
		default:
			i++
		}
	}
	return keywords
}

func isWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func isWordStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isWordPart(char byte) bool {
	return isWordStart(char) || char == '$' || (char >= '0' && char <= '9')
}

func skipLineComment(sqlQuery string, start int) int {
	for start < len(sqlQuery) && sqlQuery[start] != '\n' {
		start++
	}
	return start
}

func skipBlockComment(sqlQuery string, start int) int {
	for start+1 < len(sqlQuery) {
		if sqlQuery[start] == '*' && sqlQuery[start+1] == '/' {
			return start + 2
		}
		start++
	}
	return len(sqlQuery)
}

func skipSingleQuotedString(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == '\'' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == '\'' {
				start += 2
				continue
			}
			if start > 0 && sqlQuery[start-1] == '\\' {
				start++
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipDelimitedIdentifier(sqlQuery string, start int, delimiter byte) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == delimiter {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == delimiter {
				start += 2
				continue
			}
			if start > 0 && sqlQuery[start-1] == '\\' {
				start++
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipBracketIdentifier(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == ']' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == ']' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipDollarQuotedString(sqlQuery string, start int) (int, bool) {
	end := start + 1
	for end < len(sqlQuery) && ((sqlQuery[end] >= 'A' && sqlQuery[end] <= 'Z') || (sqlQuery[end] >= 'a' && sqlQuery[end] <= 'z') || (sqlQuery[end] >= '0' && sqlQuery[end] <= '9') || sqlQuery[end] == '_') {
		end++
	}
	if end >= len(sqlQuery) || sqlQuery[end] != '$' {
		return start, false
	}
	delimiter := sqlQuery[start : end+1]
	closeIdx := strings.Index(sqlQuery[end+1:], delimiter)
	if closeIdx < 0 {
		return len(sqlQuery), true
	}
	return end + 1 + closeIdx + len(delimiter), true
}

// stripTrailingSemicolon removes a trailing semicolon and any whitespace after it.
func stripTrailingSemicolon(sqlQuery string) string {
	// Trim trailing whitespace first
	sqlQuery = strings.TrimRight(sqlQuery, " \t\n\r")

	// Remove trailing semicolon if present
	if strings.HasSuffix(sqlQuery, ";") {
		sqlQuery = strings.TrimSuffix(sqlQuery, ";")
		// Trim any whitespace that was before the semicolon
		sqlQuery = strings.TrimRight(sqlQuery, " \t\n\r")
	}

	return sqlQuery
}
