export type SqlDialect = 'PostgreSQL' | 'MSSQL' | string;

export type ParameterType =
  | 'string'
  | 'integer'
  | 'decimal'
  | 'boolean'
  | 'date'
  | 'timestamp'
  | 'uuid'
  | 'string[]'
  | 'integer[]';

export interface QueryParameter {
  name: string;
  type: ParameterType;
  description: string;
  required: boolean;
  default?: unknown;
}

export interface SavedQuery {
  id: string;
  datasourceId?: string | undefined;
  name: string;
  description?: string | undefined;
  sql: string;
  isEnabled: boolean;
  parameters: QueryParameter[];
  createdAt?: string | undefined;
  updatedAt?: string | undefined;
}

export interface QueryExecutionResult {
  columns: Array<{ name: string; type: string }>;
  rows: Record<string, unknown>[];
  rowCount: number;
}

export interface QueryValidationResult {
  valid: boolean;
  message?: string | undefined;
  warnings?: string[] | undefined;
}

export const DEFAULT_SQL_DIALECT: SqlDialect = 'PostgreSQL';
