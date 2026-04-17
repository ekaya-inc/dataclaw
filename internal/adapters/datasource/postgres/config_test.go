package postgres

import "testing"

func TestFromMapUsesLegacyDatabaseNameAndDefaults(t *testing.T) {
	cfg, err := FromMap(map[string]any{
		"host": "db.example.com",
		"name": "warehouse",
	})
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if cfg.Database != "warehouse" {
		t.Fatalf("expected database from legacy name field, got %#v", cfg)
	}
	if cfg.Port != 5432 || cfg.SSLMode != "disable" {
		t.Fatalf("expected postgres defaults, got %#v", cfg)
	}
}

func TestBuildConnectionStringEscapesCredentials(t *testing.T) {
	connStr := buildConnectionString(&Config{
		Host:     "db.example.com",
		Port:     5432,
		User:     "alice@corp",
		Password: "p@ss/word",
		Database: "warehouse/db",
		SSLMode:  "verify-full",
	})

	want := "postgresql://alice%40corp:p%40ss%2Fword@db.example.com:5432/warehouse%2Fdb?sslmode=verify-full"
	if connStr != want {
		t.Fatalf("unexpected postgres conn string:\n got: %q\nwant: %q", connStr, want)
	}
}
