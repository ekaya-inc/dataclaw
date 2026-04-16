package runtime

import (
	"fmt"
	"net"
)

func ListenIncrement(bindAddr string, startPort int, maxAttempts int) (net.Listener, int, error) {
	if maxAttempts <= 0 {
		maxAttempts = 100
	}
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindAddr, port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no available port found starting at %d", startPort)
}
