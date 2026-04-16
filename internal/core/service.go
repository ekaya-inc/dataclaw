package core

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type Service struct {
	store     *storepkg.Store
	secret    []byte
	executor  *datasourceExecutor
	tester    func(context.Context, *storepkg.Datasource) error
	uiBaseURL func() string
	version   string
}

func New(store *storepkg.Store, secret []byte, version string, uiBaseURL func() string) *Service {
	return &Service{
		store:     store,
		secret:    secret,
		executor:  &datasourceExecutor{},
		tester:    testDatasourceConnection,
		uiBaseURL: uiBaseURL,
		version:   version,
	}
}

func (s *Service) Close() error { return s.executor.Close() }

func (s *Service) Status() map[string]any {
	baseURL := s.uiBaseURL()
	port := 0
	if parsed, err := url.Parse(baseURL); err == nil {
		port, _ = strconv.Atoi(parsed.Port())
	}
	ds, _ := s.store.GetDatasource(context.Background())
	return map[string]any{
		"name":                  "dataclaw",
		"version":               s.version,
		"base_url":              baseURL,
		"mcp_url":               baseURL + "/mcp",
		"port":                  port,
		"datasource_configured": ds != nil,
	}
}

func (s *Service) GetDatasource(ctx context.Context) (*storepkg.Datasource, error) {
	ds, err := s.store.GetDatasource(ctx)
	if err != nil || ds == nil {
		return ds, err
	}
	if err := s.decryptDatasource(ds); err != nil {
		return nil, err
	}
	return ds, nil
}

func (s *Service) UpsertDatasource(ctx context.Context, ds *storepkg.Datasource) (*storepkg.Datasource, error) {
	if ds == nil {
		return nil, errors.New("datasource is required")
	}
	if ds.Name == "" {
		return nil, errors.New("datasource name is required")
	}
	existing, err := s.GetDatasource(ctx)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := validateDatasourceUpdate(existing, ds); err != nil {
			return nil, err
		}
		ds = &storepkg.Datasource{
			ID:        existing.ID,
			Name:      ds.Name,
			Type:      existing.Type,
			Provider:  existing.Provider,
			Config:    cloneDatasourceConfig(existing.Config),
			CreatedAt: existing.CreatedAt,
		}
	} else {
		if ds.Type == "" {
			return nil, errors.New("datasource type is required")
		}
		if ds.Type != "postgres" && ds.Type != "mssql" {
			return nil, fmt.Errorf("unsupported datasource type: %s", ds.Type)
		}
		if err := s.tester(ctx, ds); err != nil {
			return nil, err
		}
	}
	if err := s.encryptDatasource(ds); err != nil {
		return nil, err
	}
	if err := s.store.SaveDatasource(ctx, ds); err != nil {
		return nil, err
	}
	_ = s.executor.Close()
	return s.GetDatasource(ctx)
}

func (s *Service) DeleteDatasource(ctx context.Context) error {
	if err := s.store.DeleteDatasource(ctx); err != nil {
		return err
	}
	return s.executor.Close()
}

func (s *Service) TestDatasource(ctx context.Context, ds *storepkg.Datasource) error {
	return s.tester(ctx, ds)
}

func (s *Service) ListQueries(ctx context.Context) ([]*storepkg.ApprovedQuery, error) {
	return s.store.ListQueries(ctx)
}

func (s *Service) GetQuery(ctx context.Context, id string) (*storepkg.ApprovedQuery, error) {
	return s.store.GetQuery(ctx, id)
}

func (s *Service) CreateQuery(ctx context.Context, q *storepkg.ApprovedQuery) (*storepkg.ApprovedQuery, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	if q.Name == "" {
		return nil, errors.New("query name is required")
	}
	q.DatasourceID = ds.ID
	normalized, err := validateStoredSQL(q.SQLQuery, q.Parameters)
	if err != nil {
		return nil, err
	}
	q.SQLQuery = normalized
	if err := s.store.CreateQuery(ctx, q); err != nil {
		return nil, err
	}
	return s.store.GetQuery(ctx, q.ID)
}

func (s *Service) UpdateQuery(ctx context.Context, id string, q *storepkg.ApprovedQuery) (*storepkg.ApprovedQuery, error) {
	existing, err := s.store.GetQuery(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("query not found")
	}
	normalized, err := validateStoredSQL(q.SQLQuery, q.Parameters)
	if err != nil {
		return nil, err
	}
	existing.Name = q.Name
	existing.Description = q.Description
	existing.SQLQuery = normalized
	existing.Parameters = q.Parameters
	existing.IsEnabled = q.IsEnabled
	if err := s.store.UpdateQuery(ctx, existing); err != nil {
		return nil, err
	}
	return s.store.GetQuery(ctx, id)
}

func (s *Service) DeleteQuery(ctx context.Context, id string) error {
	return s.store.DeleteQuery(ctx, id)
}

func (s *Service) ValidateQuerySQL(sqlQuery string, parameters []models.QueryParameter, readOnly bool) (string, error) {
	if readOnly {
		return validateReadOnlySQL(sqlQuery)
	}
	return validateStoredSQL(sqlQuery, parameters)
}

func (s *Service) TestRawQuery(ctx context.Context, sqlQuery string, limit int) (*QueryResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := validateReadOnlySQL(sqlQuery)
	if err != nil {
		return nil, err
	}
	db, err := s.executor.open(ctx, ds)
	if err != nil {
		return nil, err
	}
	return executeQueryRows(ctx, db, normalized, nil, limit)
}

func (s *Service) TestDraftQuery(ctx context.Context, sqlQuery string, parameters []models.QueryParameter, limit int) (*QueryResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	prepared, args, err := prepareReadOnlyParameterizedQuery(ds.Type, sqlQuery, parameters, nil)
	if err != nil {
		return nil, err
	}
	db, err := s.executor.open(ctx, ds)
	if err != nil {
		return nil, err
	}
	return executeQueryRows(ctx, db, prepared, args, limit)
}

func (s *Service) ExecuteStoredQuery(ctx context.Context, id string, values map[string]any, limit int) (*QueryResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	q, err := s.store.GetQuery(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, errors.New("query not found")
	}
	prepared, args, err := resolveSQLAndArgs(ds.Type, q.SQLQuery, q.Parameters, values)
	if err != nil {
		return nil, err
	}
	db, err := s.executor.open(ctx, ds)
	if err != nil {
		return nil, err
	}
	return executeQueryRows(ctx, db, prepared, args, limit)
}

func (s *Service) EnsureOpenClawKey(ctx context.Context) (*storepkg.OpenClawCredential, error) {
	cred, err := s.store.GetOpenClawCredential(ctx)
	if err != nil {
		return nil, err
	}
	if cred != nil {
		plain, err := security.DecryptString(s.secret, cred.APIKey)
		if err != nil {
			return nil, err
		}
		cred.APIKey = plain
		return cred, nil
	}
	plain, encrypted, err := generateAPIKey(s.secret)
	if err != nil {
		return nil, err
	}
	createdAt := time.Now().UTC()
	if err := s.store.SaveOpenClawCredential(ctx, encrypted, createdAt); err != nil {
		return nil, err
	}
	return &storepkg.OpenClawCredential{APIKey: plain, CreatedAt: createdAt, UpdatedAt: createdAt}, nil
}

func (s *Service) RotateOpenClawKey(ctx context.Context) (*storepkg.OpenClawCredential, error) {
	plain, encrypted, err := generateAPIKey(s.secret)
	if err != nil {
		return nil, err
	}
	createdAt := time.Now().UTC()
	if err := s.store.SaveOpenClawCredential(ctx, encrypted, createdAt); err != nil {
		return nil, err
	}
	return &storepkg.OpenClawCredential{APIKey: plain, CreatedAt: createdAt, UpdatedAt: createdAt}, nil
}

func (s *Service) ValidateOpenClawKey(ctx context.Context, key string) (bool, error) {
	cred, err := s.store.GetOpenClawCredential(ctx)
	if err != nil {
		return false, err
	}
	if cred == nil {
		return false, nil
	}
	plain, err := security.DecryptString(s.secret, cred.APIKey)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(plain), []byte(key)) == 1, nil
}

func (s *Service) requireDatasource(ctx context.Context) (*storepkg.Datasource, error) {
	ds, err := s.store.GetDatasource(ctx)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, errors.New("no datasource configured")
	}
	if err := s.decryptDatasource(ds); err != nil {
		return nil, err
	}
	return ds, nil
}

func generateAPIKey(secret []byte) (string, string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain := "dclw-" + base64.RawURLEncoding.EncodeToString(buf)
	encrypted, err := security.EncryptString(secret, plain)
	if err != nil {
		return "", "", err
	}
	return plain, encrypted, nil
}

func (s *Service) encryptDatasource(ds *storepkg.Datasource) error {
	if ds == nil || ds.Config == nil {
		return nil
	}
	raw, err := json.Marshal(ds.Config)
	if err != nil {
		return err
	}
	encrypted, err := security.EncryptString(s.secret, string(raw))
	if err != nil {
		return err
	}
	ds.ConfigEncrypted = encrypted
	return nil
}

func (s *Service) decryptDatasource(ds *storepkg.Datasource) error {
	if ds == nil {
		return nil
	}
	if ds.Config != nil {
		return nil
	}
	if ds.ConfigEncrypted == "" {
		ds.Config = map[string]any{}
		return nil
	}
	decrypted, err := security.DecryptString(s.secret, ds.ConfigEncrypted)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(decrypted), &cfg); err != nil {
		return err
	}
	ds.Config = cfg
	return nil
}

type datasourceConnectionSettings struct {
	Type                   string
	Provider               string
	Host                   string
	Port                   int
	User                   string
	Password               string
	Database               string
	SSLMode                string
	Encrypt                bool
	TrustServerCertificate bool
}

func validateDatasourceUpdate(existing, next *storepkg.Datasource) error {
	if existing == nil || next == nil {
		return nil
	}
	if datasourceConnectionFingerprint(existing) != datasourceConnectionFingerprint(next) {
		return errors.New("datasource connection settings cannot be changed after creation; remove and recreate the datasource")
	}
	return nil
}

func datasourceConnectionFingerprint(ds *storepkg.Datasource) datasourceConnectionSettings {
	if ds == nil {
		return datasourceConnectionSettings{}
	}
	return datasourceConnectionSettings{
		Type:                   ds.Type,
		Provider:               normalizeDatasourceProvider(ds),
		Host:                   stringValue(ds.Config["host"]),
		Port:                   intValue(ds.Config["port"], 0),
		User:                   stringValue(firstNonNil(ds.Config["username"], ds.Config["user"])),
		Password:               stringValue(ds.Config["password"]),
		Database:               stringValue(firstNonNil(ds.Config["database"], ds.Config["name"])),
		SSLMode:                stringValue(ds.Config["ssl_mode"]),
		Encrypt:                boolValue(ds.Config["encrypt"], false),
		TrustServerCertificate: boolValue(ds.Config["trust_server_certificate"], false),
	}
}

func normalizeDatasourceProvider(ds *storepkg.Datasource) string {
	if ds == nil || ds.Provider == "" {
		if ds == nil {
			return ""
		}
		return ds.Type
	}
	return ds.Provider
}

func cloneDatasourceConfig(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	cloned := make(map[string]any, len(config))
	for key, value := range config {
		cloned[key] = value
	}
	return cloned
}
