package config

import "testing"

func TestNormalizeBindAddrAlwaysUsesLoopback(t *testing.T) {
	cases := []string{"", "127.0.0.1", "localhost", "0.0.0.0", "192.168.1.10"}
	for _, input := range cases {
		if got := normalizeBindAddr(input); got != DefaultBindAddr {
			t.Fatalf("normalizeBindAddr(%q) = %q, want %q", input, got, DefaultBindAddr)
		}
	}
}
