package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBundleManifestAndDownloadRequireCodesButAllowReuseWithinExpiry(t *testing.T) {
	api := newTestAPI(t)

	createRec := performJSONRequest(t, api, http.MethodPost, "/api/agents", map[string]any{
		"name":      "Marketing",
		"can_query": true,
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	createdAgent := decodeData(t, createRec)["agent"].(map[string]any)
	agentID := createdAgent["id"].(string)
	apiKey := createdAgent["api_key"].(string)

	noCodeRec := performJSONRequest(t, api, http.MethodGet, "/bundles/marketing", nil)
	if noCodeRec.Code != http.StatusNotFound {
		t.Fatalf("expected manifest without code to return 404, got %d: %s", noCodeRec.Code, noCodeRec.Body.String())
	}

	codeRec := performJSONRequest(t, api, http.MethodPost, "/api/agents/"+agentID+"/bundle-code", map[string]any{})
	if codeRec.Code != http.StatusOK {
		t.Fatalf("expected bundle-code status 200, got %d: %s", codeRec.Code, codeRec.Body.String())
	}
	bundleInstall := decodeData(t, codeRec)["bundle_install"].(map[string]any)
	manifestURL := bundleInstall["bundle_url"].(string)
	if !strings.Contains(manifestURL, "/bundles/marketing?code=") {
		t.Fatalf("expected coded manifest url, got %#v", manifestURL)
	}
	manifestCode := strings.TrimPrefix(manifestURL, "http://127.0.0.1:18790/bundles/marketing?code=")

	manifestRec := performJSONRequest(t, api, http.MethodGet, "/bundles/marketing?code="+manifestCode, nil)
	if manifestRec.Code != http.StatusOK {
		t.Fatalf("expected manifest status 200, got %d: %s", manifestRec.Code, manifestRec.Body.String())
	}
	if got := manifestRec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content-type, got %q", got)
	}

	var manifest map[string]any
	if err := json.Unmarshal(manifestRec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if got := manifest["kind"]; got != "agent-bundle.v1" {
		t.Fatalf("expected bundle kind, got %#v", got)
	}
	if got := manifest["id"]; got != "dataclaw-marketing" {
		t.Fatalf("expected manifest id dataclaw-marketing, got %#v", got)
	}
	downloadURL := manifest["downloadUrl"].(string)
	if !strings.Contains(downloadURL, "/bundles/marketing/download?code=") {
		t.Fatalf("expected download url to include a fresh code, got %#v", downloadURL)
	}
	downloadCode := strings.TrimPrefix(downloadURL, "http://127.0.0.1:18790/bundles/marketing/download?code=")
	if downloadCode == manifestCode {
		t.Fatalf("expected download code to differ from manifest code")
	}

	install := manifest["install"].(map[string]any)
	skills := install["skills"].([]any)
	skill := skills[0].(map[string]any)
	if skill["source"] != "skills/dataclaw-marketing" || skill["target"] != "skills/dataclaw-marketing" {
		t.Fatalf("unexpected skill mapping: %#v", skill)
	}

	envFiles := install["envFiles"].([]any)
	envFile := envFiles[0].(map[string]any)
	values := envFile["values"].(map[string]any)
	if values["DATACLAW_BASE_URL"] != "http://127.0.0.1:18790" {
		t.Fatalf("unexpected DATACLAW_BASE_URL: %#v", values["DATACLAW_BASE_URL"])
	}
	if values["DATACLAW_API_KEY"] != apiKey {
		t.Fatalf("expected DATACLAW_API_KEY to match created api key")
	}

	mcpServers := install["mcpServers"].(map[string]any)
	mcpServer := mcpServers["dataclaw-marketing"].(map[string]any)
	if mcpServer["url"] != "http://127.0.0.1:18790/mcp/marketing" {
		t.Fatalf("unexpected mcp url: %#v", mcpServer["url"])
	}
	headers := mcpServer["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer "+apiKey {
		t.Fatalf("unexpected auth header: %#v", headers["Authorization"])
	}
	if hooks := install["hooks"].([]any); len(hooks) != 0 {
		t.Fatalf("expected hooks to be empty, got %#v", hooks)
	}

	reusedManifestRec := performJSONRequest(t, api, http.MethodGet, "/bundles/marketing?code="+manifestCode, nil)
	if reusedManifestRec.Code != http.StatusOK {
		t.Fatalf("expected reused manifest code to continue working, got %d: %s", reusedManifestRec.Code, reusedManifestRec.Body.String())
	}

	downloadRec := performJSONRequest(t, api, http.MethodGet, "/bundles/marketing/download?code="+downloadCode, nil)
	if downloadRec.Code != http.StatusOK {
		t.Fatalf("expected download status 200, got %d: %s", downloadRec.Code, downloadRec.Body.String())
	}
	if got := downloadRec.Header().Get("Content-Type"); !strings.Contains(got, "application/zip") {
		t.Fatalf("expected zip content-type, got %q", got)
	}
	if got := downloadRec.Header().Get("Content-Disposition"); !strings.Contains(got, `dataclaw-marketing.zip`) {
		t.Fatalf("expected attachment filename, got %q", got)
	}

	sum := sha256.Sum256(downloadRec.Body.Bytes())
	if manifest["sha256"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("manifest sha256 does not match downloaded archive")
	}

	reader, err := zip.NewReader(bytes.NewReader(downloadRec.Body.Bytes()), int64(downloadRec.Body.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	gotFiles := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", file.Name, err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatalf("read zip entry %q: %v", file.Name, err)
		}
		_ = rc.Close()
		gotFiles[file.Name] = buf.String()
	}
	if _, ok := gotFiles["skills/dataclaw-marketing/SKILL.md"]; !ok {
		t.Fatalf("bundle missing SKILL.md: %#v", gotFiles)
	}
	if _, ok := gotFiles["skills/dataclaw-marketing/references/dataclaw-api.md"]; !ok {
		t.Fatalf("bundle missing dataclaw-api.md: %#v", gotFiles)
	}
	if !strings.HasPrefix(gotFiles["skills/dataclaw-marketing/SKILL.md"], "---\nname: dataclaw-marketing\n") {
		t.Fatalf("expected SKILL.md to start with YAML frontmatter, got %q", gotFiles["skills/dataclaw-marketing/SKILL.md"])
	}

	reusedDownloadCodeRec := performJSONRequest(t, api, http.MethodGet, "/bundles/marketing/download?code="+downloadCode, nil)
	if reusedDownloadCodeRec.Code != http.StatusOK {
		t.Fatalf("expected reused download code to continue working, got %d: %s", reusedDownloadCodeRec.Code, reusedDownloadCodeRec.Body.String())
	}
}

func TestBundleSlugConflictReturnsConflict(t *testing.T) {
	api := newTestAPI(t)

	var firstID string
	for idx, name := range []string{"A B", "A-B"} {
		rec := performJSONRequest(t, api, http.MethodPost, "/api/agents", map[string]any{"name": name, "can_query": true})
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected create status 201 for %q, got %d: %s", name, rec.Code, rec.Body.String())
		}
		if idx == 0 {
			firstID = decodeData(t, rec)["agent"].(map[string]any)["id"].(string)
		}
	}

	codeRec := performJSONRequest(t, api, http.MethodPost, "/api/agents/"+firstID+"/bundle-code", map[string]any{})
	if codeRec.Code != http.StatusOK {
		t.Fatalf("expected bundle-code status 200, got %d: %s", codeRec.Code, codeRec.Body.String())
	}
	bundleInstall := decodeData(t, codeRec)["bundle_install"].(map[string]any)
	manifestCode := strings.TrimPrefix(bundleInstall["bundle_url"].(string), "http://127.0.0.1:18790/bundles/a_b?code=")

	rec := performJSONRequest(t, api, http.MethodGet, "/bundles/a_b?code="+manifestCode, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected slug conflict status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
