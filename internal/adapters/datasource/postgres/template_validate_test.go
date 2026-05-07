package postgres

import (
	"strings"
	"testing"
)

func TestValidateReadOnlyTemplateAccepts(t *testing.T) {
	cases := []string{
		"SELECT id FROM users ORDER BY id",
		"WITH t AS (SELECT id FROM users LIMIT 5) SELECT * FROM t ORDER BY id",
		"SELECT * FROM (SELECT id FROM users LIMIT 10) sub ORDER BY id",
		"SELECT name FROM t WHERE marker = '$1' ORDER BY name",
		"SELECT $tag$body with $1 inside$tag$ AS literal",
		"SELECT name FROM t WHERE id = {{user_id}} ORDER BY name",
	}
	for _, sql := range cases {
		if err := ValidateReadOnlyTemplate(sql); err != nil {
			t.Errorf("expected accepted, got %v\nsql: %s", err, sql)
		}
	}
}

func TestValidateReadOnlyTemplateRejectsTopLevelPagination(t *testing.T) {
	cases := []struct {
		sql      string
		fragment string
	}{
		{"SELECT id FROM users ORDER BY id LIMIT 10", "LIMIT"},
		{"SELECT id FROM users ORDER BY id OFFSET 5", "OFFSET"},
		{"SELECT id FROM users ORDER BY id LIMIT 10 OFFSET 5", "LIMIT"},
	}
	for _, tc := range cases {
		err := ValidateReadOnlyTemplate(tc.sql)
		if err == nil {
			t.Errorf("expected rejection containing %q, got nil\nsql: %s", tc.fragment, tc.sql)
			continue
		}
		if !strings.Contains(err.Error(), tc.fragment) {
			t.Errorf("expected error to mention %q, got %q\nsql: %s", tc.fragment, err.Error(), tc.sql)
		}
	}
}

func TestValidateReadOnlyTemplateRejectsNumberedBindMarker(t *testing.T) {
	cases := []string{
		"SELECT id FROM users WHERE id = $1 ORDER BY id",
		"SELECT $2, name FROM t ORDER BY name",
	}
	for _, sql := range cases {
		err := ValidateReadOnlyTemplate(sql)
		if err == nil {
			t.Errorf("expected rejection, got nil\nsql: %s", sql)
			continue
		}
		if !strings.Contains(err.Error(), "$") {
			t.Errorf("expected error to mention the bind marker, got %q\nsql: %s", err.Error(), sql)
		}
	}
}
