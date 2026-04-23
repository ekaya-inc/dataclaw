package core

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

const (
	openClawBundleKind        = "agent-bundle.v1"
	bundleRootPrefix          = "dataclaw-"
	bundleCodePurposeManifest = "manifest"
	bundleCodePurposeDownload = "download"
)

var bundleArchiveModTime = time.Unix(0, 0).UTC()

var (
	ErrBundleCodeRequired = errors.New("bundle access code is required")
	ErrBundleCodeInvalid  = errors.New("bundle access code is invalid")
	ErrBundleCodeExpired  = errors.New("bundle access code has expired")
)

type bundleAccessCode struct {
	Slug      string
	Purpose   string
	ExpiresAt time.Time
}

type BundleFileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type BundleEnvFile struct {
	Target string            `json:"target"`
	Mode   string            `json:"mode"`
	Values map[string]string `json:"values"`
}

type BundleMCPServer struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type BundleHook struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Enable bool   `json:"enable"`
}

type BundleInstall struct {
	Skills     []BundleFileMapping        `json:"skills"`
	EnvFiles   []BundleEnvFile            `json:"envFiles,omitempty"`
	MCPServers map[string]BundleMCPServer `json:"mcpServers"`
	Hooks      []BundleHook               `json:"hooks"`
}

type BundleManifest struct {
	Kind        string        `json:"kind"`
	ID          string        `json:"id"`
	Name        string        `json:"name,omitempty"`
	Version     string        `json:"version,omitempty"`
	DownloadURL string        `json:"downloadUrl"`
	SHA256      string        `json:"sha256,omitempty"`
	Install     BundleInstall `json:"install"`
}

type AgentBundle struct {
	Slug        string
	FileName    string
	Archive     []byte
	Manifest    BundleManifest
	ContentType string
}

type BundleInstallCode struct {
	Slug      string    `json:"slug"`
	Code      string    `json:"code"`
	BundleURL string    `json:"bundle_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func AgentInstallSlug(name string) string {
	lower := strings.ToLower(name)
	var out []byte
	lastUnderscore := false
	for i := 0; i < len(lower); i++ {
		ch := lower[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			out = append(out, ch)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			out = append(out, '_')
			lastUnderscore = true
		}
	}
	slug := strings.Trim(string(out), "_")
	if slug == "" {
		return "agent"
	}
	return slug
}

func (s *Service) CreateBundleInstallCode(ctx context.Context, id string) (*BundleInstallCode, error) {
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, errors.New("agent not found")
	}
	slug := AgentInstallSlug(agent.Name)
	code, expiresAt, err := s.issueBundleCode(slug, bundleCodePurposeManifest)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(s.uiBaseURL(), "/")
	return &BundleInstallCode{
		Slug:      slug,
		Code:      code,
		BundleURL: baseURL + "/bundles/" + slug + "?code=" + code,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) BuildAgentBundleManifestByCode(ctx context.Context, slug, code string) (*AgentBundle, error) {
	if err := s.validateBundleCode(slug, bundleCodePurposeManifest, code); err != nil {
		return nil, err
	}
	downloadCode, _, err := s.issueBundleCode(strings.TrimSpace(slug), bundleCodePurposeDownload)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(s.uiBaseURL(), "/")
	downloadURL := baseURL + "/bundles/" + strings.TrimSpace(slug) + "/download?code=" + downloadCode
	return s.buildAgentBundle(ctx, slug, downloadURL)
}

func (s *Service) BuildAgentBundleDownloadByCode(ctx context.Context, slug, code string) (*AgentBundle, error) {
	if err := s.validateBundleCode(slug, bundleCodePurposeDownload, code); err != nil {
		return nil, err
	}
	return s.buildAgentBundle(ctx, slug, "")
}

func (s *Service) buildAgentBundle(ctx context.Context, slug string, downloadURL string) (*AgentBundle, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, errors.New("bundle slug is required")
	}

	agent, plainKey, err := s.getAgentByInstallSlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(s.uiBaseURL(), "/")
	bundleName := bundleRootPrefix + slug
	skillPath := "skills/" + bundleName
	if strings.TrimSpace(downloadURL) == "" {
		downloadURL = baseURL + "/bundles/" + slug + "/download"
	}
	mcpURL := baseURL + "/mcp/" + slug

	datasource, err := s.GetDatasource(ctx)
	if err != nil {
		return nil, err
	}

	var accessibleQueries []*storepkg.ApprovedQuery
	if datasource != nil && (agent.CanManageApprovedQueries || agent.ApprovedQueryScope != storepkg.ApprovedQueryScopeNone) {
		accessibleQueries, err = s.ListQueriesForAgent(ctx, agent)
		if err != nil {
			return nil, err
		}
	}

	files := map[string]string{
		skillPath + "/SKILL.md":                   buildBundleSkillMarkdown(agent, bundleName, mcpURL, datasource, accessibleQueries),
		skillPath + "/references/dataclaw-api.md": buildBundleReferenceMarkdown(agent, datasource, accessibleQueries),
	}

	archive, err := buildBundleArchive(files)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(archive)
	manifest := BundleManifest{
		Kind:        openClawBundleKind,
		ID:          bundleName,
		Name:        "DataClaw " + agent.Name,
		Version:     s.Version(),
		DownloadURL: downloadURL,
		SHA256:      hex.EncodeToString(sum[:]),
		Install: BundleInstall{
			Skills: []BundleFileMapping{
				{Source: skillPath, Target: skillPath},
			},
			EnvFiles: []BundleEnvFile{
				{
					Target: ".env",
					Mode:   "merge",
					Values: map[string]string{
						"DATACLAW_BASE_URL": baseURL,
						"DATACLAW_API_KEY":  plainKey,
					},
				},
			},
			MCPServers: map[string]BundleMCPServer{
				bundleName: {
					URL: mcpURL,
					Headers: map[string]string{
						"Authorization": "Bearer " + plainKey,
					},
				},
			},
			Hooks: []BundleHook{},
		},
	}

	return &AgentBundle{
		Slug:        slug,
		FileName:    bundleName + ".zip",
		Archive:     archive,
		Manifest:    manifest,
		ContentType: "application/zip",
	}, nil
}

func (s *Service) issueBundleCode(slug, purpose string) (string, time.Time, error) {
	slug = strings.TrimSpace(slug)
	purpose = strings.TrimSpace(purpose)
	if slug == "" {
		return "", time.Time{}, errors.New("bundle slug is required")
	}
	if purpose == "" {
		return "", time.Time{}, errors.New("bundle code purpose is required")
	}
	expiresAt := s.now().Add(s.bundleCodeTTL)

	s.bundleCodesMu.Lock()
	defer s.bundleCodesMu.Unlock()
	s.cleanupBundleCodesLocked()

	for attempts := 0; attempts < 8; attempts++ {
		code, err := randomInstallCode(16)
		if err != nil {
			return "", time.Time{}, err
		}
		if _, exists := s.bundleCodes[code]; exists {
			continue
		}
		s.bundleCodes[code] = bundleAccessCode{
			Slug:      slug,
			Purpose:   purpose,
			ExpiresAt: expiresAt,
		}
		return code, expiresAt, nil
	}
	return "", time.Time{}, errors.New("could not allocate bundle access code")
}

func (s *Service) validateBundleCode(slug, purpose, code string) error {
	slug = strings.TrimSpace(slug)
	purpose = strings.TrimSpace(purpose)
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrBundleCodeRequired
	}

	s.bundleCodesMu.Lock()
	defer s.bundleCodesMu.Unlock()

	entry, ok := s.bundleCodes[code]
	if !ok {
		return ErrBundleCodeInvalid
	}
	if entry.ExpiresAt.Before(s.now()) {
		delete(s.bundleCodes, code)
		return ErrBundleCodeExpired
	}
	if entry.Slug != slug || entry.Purpose != purpose {
		return ErrBundleCodeInvalid
	}
	s.cleanupBundleCodesLocked()
	return nil
}

func (s *Service) cleanupBundleCodesLocked() {
	now := s.now()
	for code, entry := range s.bundleCodes {
		if entry.ExpiresAt.Before(now) {
			delete(s.bundleCodes, code)
		}
	}
}

func (s *Service) getAgentByInstallSlug(ctx context.Context, slug string) (*storepkg.Agent, string, error) {
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return nil, "", err
	}

	var matched *storepkg.Agent
	for _, agent := range agents {
		if AgentInstallSlug(agent.Name) != slug {
			continue
		}
		if matched != nil {
			return nil, "", fmt.Errorf("multiple access points resolve to bundle slug %q", slug)
		}
		matched = agent
	}
	if matched == nil {
		return nil, "", errors.New("access point not found")
	}

	plainKey, err := security.DecryptString(s.secret, matched.APIKeyEncrypted)
	if err != nil {
		return nil, "", err
	}
	return matched, plainKey, nil
}

func buildBundleArchive(files map[string]string) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		header := &zip.FileHeader{
			Name:     name,
			Method:   zip.Store,
			Modified: bundleArchiveModTime,
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildBundleSkillMarkdown(agent *storepkg.Agent, bundleName, mcpURL string, datasource *storepkg.Datasource, queries []*storepkg.ApprovedQuery) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + bundleName + "\n")
	b.WriteString("description: Use the DataClaw access point named " + yamlSingleQuoted(agent.Name) + " for database access after the local DataClaw server and this access point are already installed and configured\n")
	b.WriteString("---\n\n")
	b.WriteString("# " + bundleName + "\n\n")
	b.WriteString("Use this skill when you need database access through the DataClaw access point named `" + agent.Name + "`.\n\n")
	b.WriteString("## Prerequisites\n\n")
	b.WriteString("- Assume DataClaw is already installed and running locally.\n")
	b.WriteString("- Assume the `" + agent.Name + "` access point already exists at the MCP URL below.\n")
	b.WriteString("- Do not try to install, bootstrap, or reconfigure DataClaw from this skill.\n")
	b.WriteString("- If the access point is unavailable, stop and report the missing prerequisite.\n\n")
	b.WriteString("## Connection\n\n")
	b.WriteString("- MCP server: `" + bundleName + "`\n")
	b.WriteString("- MCP URL: `" + mcpURL + "`\n")
	if datasource != nil {
		b.WriteString("- Datasource: `" + datasource.Name + "` (`" + datasource.Type + "`)\n")
	}
	b.WriteString("\n## Allowed tools\n\n")
	b.WriteString("- The capabilities below come from this configured access point.\n")
	for _, tool := range bundleToolLines(agent, queries) {
		b.WriteString("- " + tool + "\n")
	}
	b.WriteString("\nSee `references/dataclaw-api.md` for permission details and approved-query guidance.\n")
	return b.String()
}

func buildBundleReferenceMarkdown(agent *storepkg.Agent, datasource *storepkg.Datasource, queries []*storepkg.ApprovedQuery) string {
	var b strings.Builder
	b.WriteString("# DataClaw Access Point Reference\n\n")
	b.WriteString("## Prerequisites\n\n")
	b.WriteString("- DataClaw must already be installed and running.\n")
	b.WriteString("- The local MCP endpoint for this access point must already exist.\n")
	b.WriteString("- This reference documents and uses the access point. It does not install or configure DataClaw.\n")
	b.WriteString("## Access point\n\n")
	b.WriteString("- Name: `" + agent.Name + "`\n")
	b.WriteString("- Raw query: " + yesNo(agent.CanQuery) + "\n")
	b.WriteString("- Raw execute: " + yesNo(agent.CanExecute) + "\n")
	b.WriteString("- Manage approved queries: " + yesNo(agent.CanManageApprovedQueries) + "\n")
	b.WriteString("- Approved query scope: `" + string(agent.ApprovedQueryScope) + "`\n")
	if datasource != nil {
		b.WriteString("\n## Datasource\n\n")
		b.WriteString("- Name: `" + datasource.Name + "`\n")
		b.WriteString("- Type: `" + datasource.Type + "`\n")
		if strings.TrimSpace(datasource.Provider) != "" {
			b.WriteString("- Provider: `" + datasource.Provider + "`\n")
		}
	}
	b.WriteString("\n## Tool guidance\n\n")
	for _, tool := range bundleToolLines(agent, queries) {
		b.WriteString("- " + tool + "\n")
	}
	b.WriteString("\n## Approved queries\n\n")
	if len(queries) == 0 {
		b.WriteString("No approved queries are currently exposed through this access point.\n")
		return b.String()
	}
	for _, query := range queries {
		b.WriteString("- `" + query.ID + "`: " + strings.TrimSpace(query.NaturalLanguagePrompt) + "\n")
	}
	return b.String()
}

func bundleToolLines(agent *storepkg.Agent, queries []*storepkg.ApprovedQuery) []string {
	lines := make([]string, 0, 4)
	if agent.CanQuery {
		lines = append(lines, "The `query` tool is available for read-only SQL.")
	} else {
		lines = append(lines, "The `query` tool is not available.")
	}
	if agent.CanExecute {
		lines = append(lines, "The `execute` tool is available for DDL or DML.")
	} else {
		lines = append(lines, "The `execute` tool is not available.")
	}
	switch agent.ApprovedQueryScope {
	case storepkg.ApprovedQueryScopeAll:
		lines = append(lines, fmt.Sprintf("All approved queries are available (%d currently defined).", len(queries)))
	case storepkg.ApprovedQueryScopeSelected:
		lines = append(lines, fmt.Sprintf("A selected set of approved queries is available (%d exposed).", len(queries)))
	default:
		lines = append(lines, "Approved-query tools are not exposed through this access point.")
	}
	if agent.CanManageApprovedQueries {
		lines = append(lines, "Approved query management tools are available.")
	}
	return lines
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func yamlSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func randomInstallCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("code length must be positive")
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(random[i])%len(alphabet)]
	}
	return string(buf), nil
}
