package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

func TestAgentInstallSlug(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"MarketingBot":    "marketingbot",
		"My MCP Agent":    "my_mcp_agent",
		"Sales/Bot v2":    "sales_bot_v2",
		"foo   bar---baz": "foo_bar_baz",
		"  My Agent  ":    "my_agent",
		"---Agent---":     "agent",
		"":                "agent",
		"!!!":             "agent",
	}

	for input, want := range cases {
		if got := AgentInstallSlug(input); got != want {
			t.Fatalf("AgentInstallSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBundleInstallCodeIsReusableUntilExpiry(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	secret, err := security.LoadOrCreateSecret(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}

	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	service := New(store, secret, "test", func() string { return "http://127.0.0.1:18790" }, dsadapter.NewFactory(dsadapter.DefaultRegistry()))
	service.now = func() time.Time { return now }
	service.bundleCodeTTL = 20 * time.Minute

	created, err := service.CreateAgent(ctx, AgentInput{Name: "Marketing", CanQuery: true})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	manifestCode, err := service.CreateBundleInstallCode(ctx, created.ID)
	if err != nil {
		t.Fatalf("CreateBundleInstallCode: %v", err)
	}
	if manifestCode.BundleURL == "" || manifestCode.Code == "" {
		t.Fatalf("expected bundle install link to include url and code: %#v", manifestCode)
	}

	manifestBundle, err := service.BuildAgentBundleManifestByCode(ctx, "marketing", manifestCode.Code)
	if err != nil {
		t.Fatalf("BuildAgentBundleManifestByCode: %v", err)
	}
	if manifestBundle.Manifest.DownloadURL == "" || manifestBundle.Manifest.DownloadURL == manifestCode.BundleURL {
		t.Fatalf("expected manifest to carry a new download code, got %q", manifestBundle.Manifest.DownloadURL)
	}
	secondManifestBundle, err := service.BuildAgentBundleManifestByCode(ctx, "marketing", manifestCode.Code)
	if err != nil {
		t.Fatalf("second BuildAgentBundleManifestByCode: %v", err)
	}
	if secondManifestBundle.Manifest.DownloadURL == manifestBundle.Manifest.DownloadURL {
		t.Fatalf("expected repeated manifest fetches to mint fresh download codes")
	}

	downloadCode := manifestBundle.Manifest.DownloadURL[len("http://127.0.0.1:18790/bundles/marketing/download?code="):]
	if _, err := service.BuildAgentBundleDownloadByCode(ctx, "marketing", downloadCode); err != nil {
		t.Fatalf("BuildAgentBundleDownloadByCode: %v", err)
	}
	if _, err := service.BuildAgentBundleDownloadByCode(ctx, "marketing", downloadCode); err != nil {
		t.Fatalf("second BuildAgentBundleDownloadByCode: %v", err)
	}

	expiringCode, err := service.CreateBundleInstallCode(ctx, created.ID)
	if err != nil {
		t.Fatalf("CreateBundleInstallCode(expiring): %v", err)
	}
	now = now.Add(21 * time.Minute)
	if _, err := service.BuildAgentBundleManifestByCode(ctx, "marketing", expiringCode.Code); err != ErrBundleCodeExpired {
		t.Fatalf("expected expired code to fail with ErrBundleCodeExpired, got %v", err)
	}
}
