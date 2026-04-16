import type { DatasourceFormValues } from './types/datasource';

export type ProviderOption = {
  id: string;
  label: string;
  adapter: DatasourceFormValues['type'];
  defaultPort: string;
  defaultSSL: DatasourceFormValues['sslMode'];
  helperText: string;
};

export const PROVIDERS: ProviderOption[] = [
  {
    id: 'postgres',
    label: 'PostgreSQL',
    adapter: 'postgres',
    defaultPort: '5432',
    defaultSSL: 'require',
    helperText: 'Works for standard PostgreSQL deployments.',
  },
  {
    id: 'supabase',
    label: 'Supabase',
    adapter: 'postgres',
    defaultPort: '6543',
    defaultSSL: 'require',
    helperText: 'Use the pooled connection string from Supabase.',
  },
  {
    id: 'neon',
    label: 'Neon',
    adapter: 'postgres',
    defaultPort: '5432',
    defaultSSL: 'require',
    helperText: 'Use the hostname from Neon connection details.',
  },
  {
    id: 'redshift',
    label: 'Amazon Redshift',
    adapter: 'postgres',
    defaultPort: '5439',
    defaultSSL: 'require',
    helperText: 'Uses the PostgreSQL adapter with Redshift defaults.',
  },
  {
    id: 'cockroachdb',
    label: 'CockroachDB',
    adapter: 'postgres',
    defaultPort: '26257',
    defaultSSL: 'verify-full',
    helperText: 'PostgreSQL wire protocol with CockroachDB defaults.',
  },
  {
    id: 'mssql',
    label: 'SQL Server',
    adapter: 'mssql',
    defaultPort: '1433',
    defaultSSL: 'require',
    helperText: 'Supports SQL authentication for Microsoft SQL Server.',
  },
];

export const QUERY_TEMPLATE = 'SELECT true AS connected';
