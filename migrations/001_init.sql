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

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  api_key_encrypted TEXT NOT NULL,
  can_query INTEGER NOT NULL DEFAULT 0,
  can_execute INTEGER NOT NULL DEFAULT 0,
  approved_query_scope TEXT NOT NULL DEFAULT 'none' CHECK (approved_query_scope IN ('none', 'all', 'selected')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_used_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_name_lower ON agents(LOWER(name));

CREATE TABLE IF NOT EXISTS agent_approved_queries (
  agent_id TEXT NOT NULL,
  query_id TEXT NOT NULL,
  PRIMARY KEY(agent_id, query_id),
  FOREIGN KEY(agent_id) REFERENCES agents(id) ON DELETE CASCADE,
  FOREIGN KEY(query_id) REFERENCES approved_queries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_approved_queries_query_id ON agent_approved_queries(query_id);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
