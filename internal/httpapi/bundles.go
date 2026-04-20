package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ekaya-inc/dataclaw/internal/core"
)

func (a *API) handleBundleBySlug(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/bundles/")
	if path == "" || path == r.URL.Path {
		http.NotFound(w, r)
		return
	}

	slug := path
	download := false
	if cutSlug, suffix, ok := strings.Cut(path, "/"); ok {
		slug = cutSlug
		download = suffix == "download"
		if !download || cutSlug == "" {
			http.NotFound(w, r)
			return
		}
	}
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	var (
		bundle *core.AgentBundle
		err    error
	)
	if download {
		bundle, err = a.service.BuildAgentBundleDownloadByCode(r.Context(), slug, code)
	} else {
		bundle, err = a.service.BuildAgentBundleManifestByCode(r.Context(), slug, code)
	}
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, core.ErrBundleCodeRequired),
			errors.Is(err, core.ErrBundleCodeInvalid),
			errors.Is(err, core.ErrBundleCodeExpired):
			slog.Warn(
				"bundle access denied",
				"bundle_slug", slug,
				"route", bundleRouteName(download),
				"remote_addr", r.RemoteAddr,
				"reason", err.Error(),
				"code_present", strings.TrimSpace(code) != "",
			)
			status = http.StatusNotFound
		case strings.Contains(strings.ToLower(err.Error()), "not found"):
			status = http.StatusNotFound
		case strings.Contains(strings.ToLower(err.Error()), "multiple access points"):
			status = http.StatusConflict
		}
		writeJSON(w, status, response{Error: err.Error()})
		return
	}

	if download {
		w.Header().Set("Content-Type", bundle.ContentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+bundle.FileName+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle.Archive)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(bundle.Manifest)
}

func bundleRouteName(download bool) string {
	if download {
		return "bundle_download"
	}
	return "bundle_manifest"
}
