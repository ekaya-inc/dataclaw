package uifs

import (
	"embed"
	"io/fs"
	"os"
)

// DirEnv, when set, redirects Load to serve the UI from that directory on
// disk instead of from the embedded production bundle. Intended for the
// `make dev` / `make dev-ui` dev loop where Vite rebuilds ui/dist on save.
const DirEnv = "DATACLAW_UI_DIR"

//go:embed all:dist
var embedded embed.FS

// Load returns the UI filesystem rooted at the bundle directory
// (i.e. with index.html at the top level).
func Load() (fs.FS, error) {
	if dir := os.Getenv(DirEnv); dir != "" {
		return os.DirFS(dir), nil
	}
	return fs.Sub(embedded, "dist")
}

// Source describes where Load will read the UI from. Returns the
// DATACLAW_UI_DIR value when set, otherwise "embedded".
func Source() string {
	if dir := os.Getenv(DirEnv); dir != "" {
		return dir
	}
	return "embedded"
}
