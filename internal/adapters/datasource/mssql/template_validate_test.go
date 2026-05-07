package mssql

import (
	"strings"
	"testing"
)

func TestValidateReadOnlyTemplateAccepts(t *testing.T) {
	cases := []string{
		"SELECT OrderKey FROM dbo.orders ORDER BY OrderKey DESC",
		"SELECT @@VERSION",
		"SELECT @@ROWCOUNT, name FROM sys.objects",
		"WITH t AS (SELECT TOP 5 * FROM dbo.orders ORDER BY OrderKey) SELECT * FROM t ORDER BY OrderKey",
		"SELECT * FROM (SELECT TOP 10 OrderKey FROM dbo.orders ORDER BY OrderKey) AS sub ORDER BY OrderKey",
		"SELECT name FROM dbo.t WHERE marker = '@status' AND label = '/* @x */' ORDER BY name",
		"SELECT [@status] FROM dbo.t ORDER BY [@status]",
		"SELECT name FROM dbo.t WHERE id = {{user_id}} ORDER BY name",
	}
	for _, sql := range cases {
		if err := ValidateReadOnlyTemplate(sql); err != nil {
			t.Errorf("expected accepted, got %v\nsql: %s", err, sql)
		}
	}
}

func TestValidateReadOnlyTemplateRejectsTopLevelTokens(t *testing.T) {
	cases := []struct {
		sql      string
		fragment string
	}{
		{"SELECT TOP 10 OrderKey FROM dbo.orders ORDER BY OrderKey DESC", "TOP"},
		{"SELECT OrderKey FROM dbo.orders ORDER BY OrderKey OFFSET 10 ROWS FETCH NEXT 5 ROWS ONLY", "OFFSET/FETCH"},
		{"SELECT OrderKey FROM dbo.orders ORDER BY OrderKey LIMIT 10", "LIMIT"},
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

func TestValidateReadOnlyTemplateRejectsBareNamedBindMarker(t *testing.T) {
	cases := []string{
		"SELECT OrderKey FROM dbo.orders WHERE CustomerKey = @customer ORDER BY OrderKey",
		"SELECT @user_id, name FROM dbo.t ORDER BY name",
	}
	for _, sql := range cases {
		err := ValidateReadOnlyTemplate(sql)
		if err == nil {
			t.Errorf("expected rejection, got nil\nsql: %s", sql)
			continue
		}
		if !strings.Contains(err.Error(), "@") {
			t.Errorf("expected error to mention the bind marker, got %q\nsql: %s", err.Error(), sql)
		}
	}
}
