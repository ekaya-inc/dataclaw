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

export interface OutputColumn {
  name: string;
  type: string;
  description: string;
}

export interface SavedQuery {
  id: string;
  datasourceId?: string | undefined;
  naturalLanguagePrompt: string;
  additionalContext: string;
  sql: string;
  allowsModification: boolean;
  parameters: QueryParameter[];
  outputColumns: OutputColumn[];
  constraints: string;
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
