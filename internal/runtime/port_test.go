package runtime

import (
	"net"
	"strings"
	"testing"
)

func TestListenIncrementUsesStartPortWhenAvailable(t *testing.T) {
	ln, port, err := ListenIncrement("127.0.0.1", 19090, 5)
	if err != nil {
		t.Fatalf("ListenIncrement returned error: %v", err)
	}
	defer ln.Close()
	if port != 19090 {
		t.Fatalf("expected port 19090, got %d", port)
	}
}

func TestListenIncrementSkipsBusyPort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:19091")
	if err != nil {
		t.Fatalf("listen occupied port: %v", err)
	}
	defer occupied.Close()

	ln, port, err := ListenIncrement("127.0.0.1", 19091, 5)
	if err != nil {
		t.Fatalf("ListenIncrement returned error: %v", err)
	}
	defer ln.Close()
	if port != 19092 {
		t.Fatalf("expected fallback port 19092, got %d", port)
	}
}

func TestListenPairProbesListenersIndependently(t *testing.T) {
	occupiedAdmin, err := net.Listen("tcp", "127.0.0.1:19093")
	if err != nil {
		t.Fatalf("listen occupied admin port: %v", err)
	}
	defer occupiedAdmin.Close()

	admin, mcp, err := ListenPair(
		ListenerRequest{Name: "admin", BindAddr: "127.0.0.1", Port: 19093},
		ListenerRequest{Name: "mcp", BindAddr: "127.0.0.1", Port: 19093},
		5,
	)
	if err != nil {
		t.Fatalf("ListenPair returned error: %v", err)
	}
	defer admin.Listener.Close()
	defer mcp.Listener.Close()

	if admin.Port != 19094 {
		t.Fatalf("admin port = %d, want 19094", admin.Port)
	}
	if mcp.Port != 19095 {
		t.Fatalf("mcp port = %d, want 19095", mcp.Port)
	}
}

func TestListenPairClosesFirstListenerOnSecondFailure(t *testing.T) {
	occupiedSecond, err := net.Listen("tcp", "127.0.0.1:19097")
	if err != nil {
		t.Fatalf("listen occupied second port: %v", err)
	}
	defer occupiedSecond.Close()

	admin, _, err := ListenPair(
		ListenerRequest{Name: "admin", BindAddr: "127.0.0.1", Port: 19096},
		ListenerRequest{Name: "mcp", BindAddr: "127.0.0.1", Port: 19097},
		1,
	)
	if err == nil {
		if admin.Listener != nil {
			admin.Listener.Close()
		}
		t.Fatal("ListenPair returned nil error")
	}
	if !strings.Contains(err.Error(), "listen mcp") {
		t.Fatalf("error = %v, want listen mcp context", err)
	}

	probe, probePort, err := ListenIncrement("127.0.0.1", 19096, 1)
	if err != nil {
		t.Fatalf("first listener was not closed after failure: %v", err)
	}
	defer probe.Close()
	if probePort != 19096 {
		t.Fatalf("probe port = %d, want 19096", probePort)
	}
}
