export type DatasourceType = 'postgres' | 'mssql';

export type SSLMode = 'disable' | 'allow' | 'prefer' | 'require' | 'verify-ca' | 'verify-full';

export interface DatasourceRecord {
  id: string;
  type: DatasourceType;
  provider?: string | undefined;
  displayName: string;
  database: string;
  host: string;
  port: number;
  username?: string | undefined;
  password?: string | undefined;
  sslMode?: SSLMode | undefined;
  options?: Record<string, unknown> | undefined;
  createdAt?: string | undefined;
  updatedAt?: string | undefined;
}

export interface DatasourceFormValues {
  type: DatasourceType;
  provider: string;
  displayName: string;
  host: string;
  port: string;
  database: string;
  username: string;
  password: string;
  sslMode: SSLMode;
  encrypt: boolean;
  trustServerCertificate: boolean;
}

export interface TestConnectionResult {
  success: boolean;
  message: string;
}

export interface RuntimeStatus {
  version?: string | undefined;
  baseUrl?: string | undefined;
  port?: number | undefined;
  datasourceConfigured?: boolean | undefined;
}
