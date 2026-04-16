package mssql

import (
	"database/sql"
	"testing"
)

func TestWrapQueryUsesBoundedDefaultLimit(t *testing.T) {
	got := wrapQuery("SELECT * FROM accounts", 0)
	want := "SELECT TOP (100) * FROM (SELECT * FROM accounts) AS _limited"
	if got != want {
		t.Fatalf("unexpected wrapped query:\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertParamsConvertsPlaceholdersAndNamedArgs(t *testing.T) {
	query, args := convertParams("SELECT * FROM accounts WHERE id = $1 AND region = $2", []any{123, "emea"})

	if query != "SELECT * FROM accounts WHERE id = @p1 AND region = @p2" {
		t.Fatalf("unexpected converted query: %q", query)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %#v", args)
	}

	first, ok := args[0].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected first arg to be sql.NamedArg, got %#v", args[0])
	}
	second, ok := args[1].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected second arg to be sql.NamedArg, got %#v", args[1])
	}
	if first.Name != "p1" || first.Value != 123 {
		t.Fatalf("unexpected first named arg: %#v", first)
	}
	if second.Name != "p2" || second.Value != "emea" {
		t.Fatalf("unexpected second named arg: %#v", second)
	}
}

func TestPrepareQueryLeavesCTEQueriesUnwrapped(t *testing.T) {
	query := "WITH recent_accounts AS (SELECT id FROM accounts) SELECT id FROM recent_accounts"
	if got := prepareQuery(query, 25); got != query {
		t.Fatalf("expected CTE query to remain unwrapped, got %q", got)
	}
}

func TestPrepareQueryLeavesParameterizedCTEQueriesUnwrapped(t *testing.T) {
	query, args := convertParams("/* leading comment */ WITH recent_accounts AS (SELECT $1 AS id) SELECT id FROM recent_accounts WHERE id = $1", []any{123})
	got := prepareQuery(query, 25)
	if got != query {
		t.Fatalf("expected parameterized CTE query to remain unwrapped, got %q", got)
	}
	first, ok := args[0].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected sql.NamedArg, got %#v", args[0])
	}
	if first.Name != "p1" || first.Value != 123 {
		t.Fatalf("unexpected named arg: %#v", first)
	}
}
