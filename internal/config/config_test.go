package config

import (
	"log/slog"
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
