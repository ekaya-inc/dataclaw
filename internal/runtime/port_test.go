package runtime

import (
	"net"
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
