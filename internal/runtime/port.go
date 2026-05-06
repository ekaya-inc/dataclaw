package runtime

import (
	"fmt"
	"net"
)

type ListenerRequest struct {
	Name     string
	BindAddr string
	Port     int
}

type ListenerResult struct {
	Name     string
	Listener net.Listener
	Port     int
}

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

func ListenPair(first ListenerRequest, second ListenerRequest, maxAttempts int) (ListenerResult, ListenerResult, error) {
	firstListener, firstPort, err := ListenIncrement(first.BindAddr, first.Port, maxAttempts)
	if err != nil {
		return ListenerResult{}, ListenerResult{}, fmt.Errorf("listen %s: %w", listenerName(first), err)
	}
	firstResult := ListenerResult{Name: listenerName(first), Listener: firstListener, Port: firstPort}
	secondListener, secondPort, err := ListenIncrement(second.BindAddr, second.Port, maxAttempts)
	if err != nil {
		_ = firstListener.Close()
		return ListenerResult{}, ListenerResult{}, fmt.Errorf("listen %s: %w", listenerName(second), err)
	}
	secondResult := ListenerResult{Name: listenerName(second), Listener: secondListener, Port: secondPort}
	return firstResult, secondResult, nil
}

func listenerName(req ListenerRequest) string {
	if req.Name == "" {
		return "listener"
	}
	return req.Name
}
