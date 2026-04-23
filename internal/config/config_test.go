package config

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeBindAddrAlwaysUsesLoopback(t *testing.T) {
	cases := []string{"", "127.0.0.1", "localhost", "0.0.0.0", "192.168.1.10"}
	for _, input := range cases {
		if got := normalizeBindAddr(input); got != DefaultBindAddr {
			t.Fatalf("normalizeBindAddr(%q) = %q, want %q", input, got, DefaultBindAddr)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"", DefaultLogLevel},
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{" info ", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tc := range cases {
		got, err := parseLogLevel(tc.in)
		if err != nil {
			t.Fatalf("parseLogLevel(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseLogLevel("chatty"); err == nil {
		t.Fatal("parseLogLevel(\"chatty\") returned no error")
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	home := t.TempDir()
	setLoadEnv(t, map[string]string{
		"HOME":                 home,
		"DATACLAW_BIND_ADDR":   "",
		"DATACLAW_PORT":        "",
		"DATACLAW_DATA_DIR":    "",
		"DATACLAW_DB_PATH":     "",
		"DATACLAW_SECRET_PATH": "",
		"DATACLAW_LOG_LEVEL":   "",
	})

	cfg, err := Load("1.2.3")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	wantDataDir := filepath.Join(home, ".dataclaw")
	if cfg.BindAddr != DefaultBindAddr {
		t.Fatalf("BindAddr = %q, want %q", cfg.BindAddr, DefaultBindAddr)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.DataDir != wantDataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantDataDir)
	}
	if cfg.SQLitePath != filepath.Join(wantDataDir, "dataclaw.sqlite") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SecretPath != filepath.Join(wantDataDir, "secret.key") {
		t.Fatalf("SecretPath = %q", cfg.SecretPath)
	}
	if cfg.Version != "1.2.3" {
		t.Fatalf("Version = %q, want %q", cfg.Version, "1.2.3")
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Fatalf("LogLevel = %v, want %v", cfg.LogLevel, DefaultLogLevel)
	}
}

func TestLoadUsesEnvOverrides(t *testing.T) {
	customDir := filepath.Join(t.TempDir(), "custom")
	setLoadEnv(t, map[string]string{
		"HOME":                 t.TempDir(),
		"DATACLAW_BIND_ADDR":   " 0.0.0.0 ",
		"DATACLAW_PORT":        " 19001 ",
		"DATACLAW_DATA_DIR":    " " + customDir + " ",
		"DATACLAW_DB_PATH":     " " + filepath.Join(customDir, "db.sqlite") + " ",
		"DATACLAW_SECRET_PATH": " " + filepath.Join(customDir, "secret.key") + " ",
		"DATACLAW_LOG_LEVEL":   " WARN ",
	})

	cfg, err := Load("dev")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.BindAddr != DefaultBindAddr {
		t.Fatalf("BindAddr = %q, want normalized %q", cfg.BindAddr, DefaultBindAddr)
	}
	if cfg.Port != 19001 {
		t.Fatalf("Port = %d, want %d", cfg.Port, 19001)
	}
	if cfg.DataDir != customDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, customDir)
	}
	if cfg.SQLitePath != filepath.Join(customDir, "db.sqlite") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SecretPath != filepath.Join(customDir, "secret.key") {
		t.Fatalf("SecretPath = %q", cfg.SecretPath)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Fatalf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelWarn)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	setLoadEnv(t, map[string]string{
		"HOME":               t.TempDir(),
		"DATACLAW_PORT":      "not-a-number",
		"DATACLAW_LOG_LEVEL": "",
	})

	_, err := Load("test")
	if err == nil {
		t.Fatal("Load() returned nil error for invalid DATACLAW_PORT")
	}
	if !strings.Contains(err.Error(), "parse DATACLAW_PORT") {
		t.Fatalf("Load() error = %v, want parse DATACLAW_PORT error", err)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	setLoadEnv(t, map[string]string{
		"HOME":               t.TempDir(),
		"DATACLAW_PORT":      "",
		"DATACLAW_LOG_LEVEL": "chatty",
	})

	_, err := Load("test")
	if err == nil {
		t.Fatal("Load() returned nil error for invalid DATACLAW_LOG_LEVEL")
	}
	if !strings.Contains(err.Error(), "invalid DATACLAW_LOG_LEVEL") {
		t.Fatalf("Load() error = %v, want invalid DATACLAW_LOG_LEVEL error", err)
	}
}

func TestConfigBaseURLHelpers(t *testing.T) {
	cfg := &Config{BindAddr: "0.0.0.0"}

	if got := cfg.BaseURL(18790); got != "http://0.0.0.0:18790" {
		t.Fatalf("BaseURL() = %q, want %q", got, "http://0.0.0.0:18790")
	}
	if got := cfg.UIBaseURL(18790); got != "http://127.0.0.1:18790" {
		t.Fatalf("UIBaseURL() = %q, want %q", got, "http://127.0.0.1:18790")
	}

	cfg.BindAddr = "127.0.0.1"
	if got := cfg.UIBaseURL(18790); got != "http://127.0.0.1:18790" {
		t.Fatalf("UIBaseURL(loopback) = %q, want %q", got, "http://127.0.0.1:18790")
	}
}

func setLoadEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range []string{
		"HOME",
		"DATACLAW_BIND_ADDR",
		"DATACLAW_PORT",
		"DATACLAW_DATA_DIR",
		"DATACLAW_DB_PATH",
		"DATACLAW_SECRET_PATH",
		"DATACLAW_LOG_LEVEL",
	} {
		value, ok := values[key]
		if !ok {
			value = ""
		}
		t.Setenv(key, value)
	}
}
