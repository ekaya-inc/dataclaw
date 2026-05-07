package app

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

func TestRegisterUIRoutes(t *testing.T) {
	uiFS := fstest.MapFS{
		"index.html":     {Data: []byte("<!doctype html><html>spa</html>")},
		"assets/app.js":  {Data: []byte("console.log('ready');")},
		"assets/app.css": {Data: []byte("body { color: black; }")},
	}

	mux := http.NewServeMux()
	registerUIRoutes(mux, uiFS)

	tests := []struct {
		name           string
		path           string
		wantStatusCode int
		wantBody       string
		wantType       string
	}{
		{
			name:           "serves index at root",
			path:           "/",
			wantStatusCode: http.StatusOK,
			wantBody:       "<!doctype html><html>spa</html>",
			wantType:       "text/html",
		},
		{
			name:           "serves asset from filesystem",
			path:           "/assets/app.js",
			wantStatusCode: http.StatusOK,
			wantBody:       "console.log('ready');",
			wantType:       "text/javascript",
		},
		{
			name:           "falls back to index for spa route",
			path:           "/workspaces/abc",
			wantStatusCode: http.StatusOK,
			wantBody:       "<!doctype html><html>spa</html>",
			wantType:       "text/html",
		},
		{
			name:           "rejects api routes",
			path:           "/api/datasources",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "rejects mcp routes",
			path:           "/mcp",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatusCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatusCode)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantType != "" && !strings.Contains(rec.Header().Get("Content-Type"), tc.wantType) {
				t.Fatalf("Content-Type = %q, want substring %q", rec.Header().Get("Content-Type"), tc.wantType)
			}
		})
	}
}

func TestRegisterUIRoutesReturnsInternalServerErrorWhenIndexMissing(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux, fstest.MapFS{
		"assets/app.js": {Data: []byte("console.log('ready');")},
	})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "index.html missing") {
		t.Fatalf("body = %q, want missing index error", rec.Body.String())
	}
}

func TestLogRequestsPassesThroughAndLogsRequest(t *testing.T) {
	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "http://example.com/health", nil)
	rec := httptest.NewRecorder()
	logRequests(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "ok")
	}

	output := buf.String()
	for _, want := range []string{"level=DEBUG", "msg=request", "method=POST", "path=/health", "duration_ms="} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output = %q, want substring %q", output, want)
		}
	}
}

func TestWaitForShutdownSignalLetsActiveRequestsDrain(t *testing.T) {
	release := make(chan struct{})
	requestStarted := make(chan struct{})
	server, baseURL := startBlockingTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-release
		_, _ = w.Write([]byte("done"))
	})

	responseDone := make(chan error, 1)
	go func() {
		resp, err := http.Get(baseURL)
		if err != nil {
			responseDone <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			responseDone <- errors.New(resp.Status)
			return
		}
		responseDone <- nil
	}()
	waitForChannel(t, requestStarted, "request to start")

	sig := make(chan os.Signal, 2)
	shutdownDone := make(chan struct{})
	go func() {
		waitForShutdownSignal(sig, server)
		close(shutdownDone)
	}()

	sig <- syscall.SIGINT
	assertChannelBlocked(t, shutdownDone, 100*time.Millisecond, "shutdown should wait for active request")

	close(release)
	waitForChannel(t, shutdownDone, "graceful shutdown to finish")
	if err := <-responseDone; err != nil {
		t.Fatalf("request error = %v, want nil", err)
	}
}

func TestWaitForShutdownSignalClosesActiveRequestsOnSecondSignal(t *testing.T) {
	requestStarted := make(chan struct{})
	server, baseURL := startBlockingTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	})

	responseDone := make(chan error, 1)
	go func() {
		resp, err := http.Get(baseURL)
		if err != nil {
			responseDone <- err
			return
		}
		defer resp.Body.Close()
		responseDone <- nil
	}()
	waitForChannel(t, requestStarted, "request to start")

	sig := make(chan os.Signal, 2)
	shutdownDone := make(chan struct{})
	go func() {
		waitForShutdownSignal(sig, server)
		close(shutdownDone)
	}()

	sig <- syscall.SIGINT
	assertChannelBlocked(t, shutdownDone, 100*time.Millisecond, "first shutdown should wait for active request")

	sig <- syscall.SIGINT
	waitForChannel(t, shutdownDone, "forced shutdown after second signal")
	waitForChannel(t, responseDone, "active request to be closed")
}

func startBlockingTestServer(t *testing.T, handler http.HandlerFunc) (*http.Server, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	return server, "http://" + ln.Addr().String()
}

func waitForChannel[T any](t *testing.T, ch <-chan T, name string) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
		var zero T
		return zero
	}
}

func assertChannelBlocked[T any](t *testing.T, ch <-chan T, timeout time.Duration, failure string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal(failure)
	case <-time.After(timeout):
	}
}
