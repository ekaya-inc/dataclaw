package mssql

import "testing"

func TestFromMapSupportsLegacyUserAndDatabaseNames(t *testing.T) {
	cfg, err := FromMap(map[string]any{
		"host": "sql.example.com",
		"user": "analyst",
		"name": "warehouse",
	})
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if cfg.Username != "analyst" || cfg.Database != "warehouse" {
		t.Fatalf("expected legacy config fields to be preserved, got %#v", cfg)
	}
	if cfg.Port != 1433 || cfg.Encrypt {
		t.Fatalf("expected mssql defaults, got %#v", cfg)
	}
}

func TestBuildConnectionStringSupportsTrustServerCertificate(t *testing.T) {
	connStr := buildConnectionString(&Config{
		Host:                   "sql.example.com",
		Port:                   1433,
		Username:               "alice@example.com",
		Password:               "s3cret!",
		Database:               "warehouse",
		Encrypt:                true,
		TrustServerCertificate: true,
		ConnectionTimeout:      15,
	})

	want := "sqlserver://alice%40example.com:s3cret%21@sql.example.com:1433?TrustServerCertificate=true&connection+timeout=15&database=warehouse&encrypt=true"
	if connStr != want {
		t.Fatalf("unexpected mssql conn string:\n got: %q\nwant: %q", connStr, want)
	}
}
