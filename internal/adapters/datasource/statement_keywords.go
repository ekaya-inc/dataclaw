package datasource

import (
	"errors"
	"strings"
)

var ErrExecuteStatementType = errors.New("execute only accepts single-statement DDL or DML")

func FirstStatementKeyword(sqlQuery string) string {
	tokens := tokenizeSQL(sqlQuery)
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0].Text
}

func ContainsStatementKeyword(sqlQuery string, keyword string) bool {
	needle := strings.ToUpper(strings.TrimSpace(keyword))
	if needle == "" {
		return false
	}
	for _, token := range tokenizeSQL(sqlQuery) {
		if token.Text == needle {
			return true
		}
	}
	return false
}

func SupportsExecuteStatement(sqlQuery string) bool {
	switch FirstStatementKeyword(sqlQuery) {
	case "INSERT", "UPDATE", "DELETE", "MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME":
		return true
	default:
		return false
	}
}
