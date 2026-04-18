package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
	sqltmpl "github.com/ekaya-inc/dataclaw/pkg/sql"
)

type Service struct {
	store     *storepkg.Store
	secret    []byte
	adapters  dsadapter.Factory
	uiBaseURL func() string
	version   string
}

func New(store *storepkg.Store, secret []byte, version string, uiBaseURL func() string, adapters dsadapter.Factory) *Service {
	if adapters == nil {
		adapters = dsadapter.NewFactory(dsadapter.DefaultRegistry())
	}
	return &Service{
		store:     store,
		secret:    secret,
		adapters:  adapters,
		uiBaseURL: uiBaseURL,
		version:   version,
	}
}

func (s *Service) Close() error { return nil }

func (s *Service) DatasourceTypes() []dsadapter.AdapterInfo {
	if s.adapters == nil {
		return nil
	}
	return s.adapters.ListTypes()
}

func (s *Service) DatasourceTypeInfo(dsType string) (dsadapter.AdapterInfo, bool) {
	if s.adapters == nil {
		return dsadapter.AdapterInfo{}, false
	}
	return s.adapters.TypeInfo(dsType)
}

func (s *Service) Status() map[string]any {
	baseURL := s.uiBaseURL()
	port := 0
	if parsed, err := url.Parse(baseURL); err == nil {
		port, _ = strconv.Atoi(parsed.Port())
	}
	ds, _ := s.store.GetDatasource(context.Background())
	agentCount, _ := s.store.CountAgents(context.Background())
	return map[string]any{
		"name":                  "dataclaw",
		"version":               s.version,
		"base_url":              baseURL,
		"mcp_url":               baseURL + "/mcp",
		"port":                  port,
		"datasource_configured": ds != nil,
		"agent_count":           agentCount,
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
		if err := validateDatasourceUpdate(existing, ds, s.adapters); err != nil {
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
		if s.adapters == nil || !s.adapters.SupportsType(ds.Type) {
			return nil, fmt.Errorf("unsupported datasource type: %s", ds.Type)
		}
		if err := s.TestDatasource(ctx, ds); err != nil {
			return nil, err
		}
	}
	if err := s.encryptDatasource(ds); err != nil {
		return nil, err
	}
	if err := s.store.SaveDatasource(ctx, ds); err != nil {
		return nil, err
	}
	return s.GetDatasource(ctx)
}

func (s *Service) DeleteDatasource(ctx context.Context) error {
	// Approved queries cascade-delete via the FK on approved_queries.datasource_id.
	// Agents remain so permissions/config survive datasource resets, but MCP tools will fail closed until a datasource is restored.
	if err := s.store.DeleteDatasource(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) TestDatasource(ctx context.Context, ds *storepkg.Datasource) error {
	if ds == nil {
		return errors.New("datasource is required")
	}
	if s.adapters == nil {
		return errors.New("datasource adapter factory is not configured")
	}
	tester, err := s.adapters.NewConnectionTester(ctx, ds.Type, ds.Config)
	if err != nil {
		return err
	}
	defer tester.Close()
	return tester.TestConnection(ctx)
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
	if q.NaturalLanguagePrompt == "" {
		return nil, errors.New("natural_language_prompt is required")
	}
	q.DatasourceID = ds.ID
	normalized, err := validateStoredQueryForStorage(q.SQLQuery, q.Parameters, q.AllowsModification)
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
	if q.NaturalLanguagePrompt == "" {
		return nil, errors.New("natural_language_prompt is required")
	}
	normalized, err := validateStoredQueryForStorage(q.SQLQuery, q.Parameters, q.AllowsModification)
	if err != nil {
		return nil, err
	}
	existing.NaturalLanguagePrompt = q.NaturalLanguagePrompt
	existing.AdditionalContext = q.AdditionalContext
	existing.SQLQuery = normalized
	existing.AllowsModification = q.AllowsModification
	existing.Parameters = q.Parameters
	existing.OutputColumns = q.OutputColumns
	existing.Constraints = q.Constraints
	if err := s.store.UpdateQuery(ctx, existing); err != nil {
		return nil, err
	}
	return s.store.GetQuery(ctx, id)
}

func (s *Service) DeleteQuery(ctx context.Context, id string) error {
	return s.store.DeleteQuery(ctx, id)
}

func (s *Service) ValidateQuerySQL(sqlQuery string, parameters []models.QueryParameter, allowsModification bool) (string, error) {
	return validateStoredQueryForStorage(sqlQuery, parameters, allowsModification)
}

func (s *Service) ValidateRawSQL(sqlQuery string) (string, error) {
	return validateReadOnlySQL(sqlQuery)
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
	executor, err := s.adapters.NewQueryExecutor(ctx, ds.Type, ds.Config)
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return executor.Query(ctx, normalized, limit)
}

func (s *Service) TestDraftQuery(ctx context.Context, sqlQuery string, parameters []models.QueryParameter, values map[string]any, allowsModification bool, limit int) (*QueryResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	effectiveValues, err := prepareExecutionParameterValues(parameters, values)
	if err != nil {
		return nil, err
	}
	if injectionResults := sqltmpl.CheckAllParameters(effectiveValues); len(injectionResults) > 0 {
		return nil, fmt.Errorf("potential SQL injection detected in parameter '%s'", injectionResults[0].ParamName)
	}
	if adapterInfo, ok := s.DatasourceTypeInfo(ds.Type); ok && hasArrayParameters(parameters, effectiveValues) && !adapterInfo.Capabilities.SupportsArrayParameters {
		name := adapterInfo.DisplayName
		if name == "" {
			name = ds.Type
		}
		return nil, fmt.Errorf("array parameters are not supported for %s draft-query execution yet", name)
	}
	executor, err := s.adapters.NewQueryExecutor(ctx, ds.Type, ds.Config)
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	if allowsModification {
		return executor.ExecuteDMLQuery(ctx, sqlQuery, parameters, effectiveValues, limit)
	}
	return executor.QueryWithParameters(ctx, sqlQuery, parameters, effectiveValues, limit)
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
	effectiveValues, err := prepareExecutionParameterValues(q.Parameters, values)
	if err != nil {
		return nil, err
	}
	if injectionResults := sqltmpl.CheckAllParameters(effectiveValues); len(injectionResults) > 0 {
		return nil, fmt.Errorf("potential SQL injection detected in parameter '%s'", injectionResults[0].ParamName)
	}
	if adapterInfo, ok := s.DatasourceTypeInfo(ds.Type); ok && hasArrayParameters(q.Parameters, effectiveValues) && !adapterInfo.Capabilities.SupportsArrayParameters {
		name := adapterInfo.DisplayName
		if name == "" {
			name = ds.Type
		}
		return nil, fmt.Errorf("array parameters are not supported for %s saved-query execution yet", name)
	}
	executor, err := s.adapters.NewQueryExecutor(ctx, ds.Type, ds.Config)
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	if q.AllowsModification {
		return executor.ExecuteDMLQuery(ctx, q.SQLQuery, q.Parameters, effectiveValues, limit)
	}
	return executor.QueryWithParameters(ctx, q.SQLQuery, q.Parameters, effectiveValues, limit)
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

func validateDatasourceUpdate(existing, next *storepkg.Datasource, adapters dsadapter.Factory) error {
	if existing == nil || next == nil {
		return nil
	}
	if existing.Type != next.Type || normalizeDatasourceProvider(existing) != normalizeDatasourceProvider(next) {
		return errors.New("datasource connection settings cannot be changed after creation; remove and recreate the datasource")
	}
	if adapters == nil {
		return errors.New("datasource adapter factory is not configured")
	}
	existingFingerprint, err := adapters.ConfigFingerprint(existing.Type, existing.Config)
	if err != nil {
		return err
	}
	nextFingerprint, err := adapters.ConfigFingerprint(next.Type, next.Config)
	if err != nil {
		return err
	}
	if existingFingerprint != nextFingerprint {
		return errors.New("datasource connection settings cannot be changed after creation; remove and recreate the datasource")
	}
	return nil
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
