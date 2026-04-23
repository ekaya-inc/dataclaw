package app

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
