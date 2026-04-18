package core

import (
	"errors"
	"fmt"
	"io"
	"strings"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/pkg/models"
	sqltmpl "github.com/ekaya-inc/dataclaw/pkg/sql"
)

func validateReadOnlySQL(sqlQuery string) (string, error) {
	result := sqltmpl.ValidateAndNormalize(sqlQuery)
	if result.Error != nil {
		return "", result.Error
	}
	normalized := result.NormalizedSQL
	tokens := tokenizeSQL(normalized)
	if len(tokens) == 0 {
		return "", errors.New("sql is required")
	}
	first := tokens[0].Text
	if first != "SELECT" && first != "WITH" {
		return "", errors.New("only read-only SELECT or WITH statements are allowed")
	}
	if containsMutatingKeyword(tokens) {
		return "", errors.New("only read-only SELECT or WITH statements are allowed")
	}
	switch first {
	case "SELECT":
		if hasSelectInto(tokens, 0) {
			return "", errors.New("SELECT INTO is not allowed in read-only queries")
		}
	case "WITH":
		mainStart, err := withMainStatementStart(tokens)
		if err != nil {
			return "", err
		}
		if mainStart >= len(tokens) || tokens[mainStart].Text != "SELECT" {
			return "", errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if hasSelectInto(tokens[mainStart:], 0) {
			return "", errors.New("SELECT INTO is not allowed in read-only queries")
		}
	}
	return normalized, nil
}

type sqlToken struct {
	Text  string
	Depth int
}

func tokenizeSQL(sqlQuery string) []sqlToken {
	tokens := make([]sqlToken, 0, 32)
	depth := 0
	for i := 0; i < len(sqlQuery); {
		switch {
		case isSQLWhitespace(sqlQuery[i]):
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
		case isSQLWordStart(sqlQuery[i]):
			start := i
			i++
			for i < len(sqlQuery) && isSQLWordPart(sqlQuery[i]) {
				i++
			}
			tokens = append(tokens, sqlToken{Text: strings.ToUpper(sqlQuery[start:i]), Depth: depth})
		case sqlQuery[i] == '(':
			tokens = append(tokens, sqlToken{Text: "(", Depth: depth})
			depth++
			i++
		case sqlQuery[i] == ')':
			if depth > 0 {
				depth--
			}
			tokens = append(tokens, sqlToken{Text: ")", Depth: depth})
			i++
		case sqlQuery[i] == ',':
			tokens = append(tokens, sqlToken{Text: ",", Depth: depth})
			i++
		default:
			i++
		}
	}
	return tokens
}

func containsMutatingKeyword(tokens []sqlToken) bool {
	for _, token := range tokens {
		switch token.Text {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "ALTER", "CREATE", "DROP", "TRUNCATE":
			return true
		}
	}
	return false
}

func withMainStatementStart(tokens []sqlToken) (int, error) {
	if len(tokens) == 0 || tokens[0].Text != "WITH" {
		return -1, errors.New("only read-only SELECT or WITH statements are allowed")
	}
	i := 1
	if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "RECURSIVE" {
		i++
	}
	for {
		if i >= len(tokens) || tokens[i].Depth != 0 || !isSQLIdentifierToken(tokens[i].Text) {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		i++
		if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "(" {
			var err error
			i, err = skipTokenGroup(tokens, i)
			if err != nil {
				return -1, errors.New("only read-only SELECT or WITH statements are allowed")
			}
		}
		if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "AS" {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		i++
		if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "NOT" {
			i++
			if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "MATERIALIZED" {
				return -1, errors.New("only read-only SELECT or WITH statements are allowed")
			}
			i++
		} else if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "MATERIALIZED" {
			i++
		}
		if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "(" {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		var err error
		i, err = skipTokenGroup(tokens, i)
		if err != nil {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if i >= len(tokens) {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if tokens[i].Depth == 0 && tokens[i].Text == "," {
			i++
			continue
		}
		return i, nil
	}
}

func hasSelectInto(tokens []sqlToken, depth int) bool {
	seenSelect := false
	for _, token := range tokens {
		if token.Depth != depth {
			continue
		}
		switch token.Text {
		case "SELECT":
			seenSelect = true
		case "INTO":
			if seenSelect {
				return true
			}
		case "FROM":
			if seenSelect {
				return false
			}
		}
	}
	return false
}

func skipTokenGroup(tokens []sqlToken, start int) (int, error) {
	if start >= len(tokens) || tokens[start].Text != "(" {
		return -1, io.ErrUnexpectedEOF
	}
	targetDepth := tokens[start].Depth
	for i := start + 1; i < len(tokens); i++ {
		if tokens[i].Text == ")" && tokens[i].Depth == targetDepth {
			return i + 1, nil
		}
	}
	return -1, io.ErrUnexpectedEOF
}

func isSQLWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func isSQLWordStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isSQLWordPart(char byte) bool {
	return isSQLWordStart(char) || char == '$' || (char >= '0' && char <= '9')
}

func isSQLIdentifierToken(token string) bool {
	switch token {
	case "AS", "SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "MERGE":
		return false
	default:
		return token != "" && token != "," && token != "(" && token != ")"
	}
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

func validateStoredSQL(sqlQuery string, params []models.QueryParameter) (string, error) {
	result := sqltmpl.ValidateAndNormalize(sqlQuery)
	if result.Error != nil {
		return "", result.Error
	}
	normalized := result.NormalizedSQL
	if normalized == "" {
		return "", errors.New("sql is required")
	}
	if err := sqltmpl.ValidateParameterDefinitions(normalized, params); err != nil {
		return "", err
	}
	if problems := sqltmpl.FindParametersInStringLiterals(normalized); len(problems) > 0 {
		return "", fmt.Errorf("parameters inside string literals are not allowed: %s", strings.Join(problems, ", "))
	}
	return normalized, nil
}

func validateStoredReadOnlySQL(sqlQuery string, params []models.QueryParameter) (string, error) {
	normalized, err := validateStoredSQL(sqlQuery, params)
	if err != nil {
		return "", err
	}
	return validateReadOnlySQL(normalized)
}

func validateStoredDMLSQL(sqlQuery string, params []models.QueryParameter) (string, error) {
	normalized, err := validateStoredSQL(sqlQuery, params)
	if err != nil {
		return "", err
	}
	return dsadapter.ValidateDMLSQL(normalized)
}

func validateStoredQueryForStorage(sqlQuery string, params []models.QueryParameter, allowsModification bool) (string, error) {
	if allowsModification {
		return validateStoredDMLSQL(sqlQuery, params)
	}
	return validateStoredReadOnlySQL(sqlQuery, params)
}

func prepareReadOnlyParameterizedQuery(sqlQuery string, params []models.QueryParameter, values map[string]any) (string, []any, error) {
	return dsadapter.PrepareReadOnlyParameterizedQuery(sqlQuery, params, values)
}
