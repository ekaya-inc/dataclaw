CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS datasources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT '',
  config_encrypted TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS approved_queries (
  id TEXT PRIMARY KEY,
  datasource_id TEXT NOT NULL,
  natural_language_prompt TEXT NOT NULL,
  additional_context TEXT NOT NULL DEFAULT '',
  sql_query TEXT NOT NULL,
  allows_modification INTEGER NOT NULL DEFAULT 0,
  parameters_json TEXT NOT NULL DEFAULT '[]',
  output_columns_json TEXT NOT NULL DEFAULT '[]',
  constraints TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(datasource_id) REFERENCES datasources(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_approved_queries_datasource ON approved_queries(datasource_id);

CREATE TABLE IF NOT EXISTS openclaw_credentials (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  api_key_encrypted TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
