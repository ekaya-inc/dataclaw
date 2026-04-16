import type { DatasourceFormValues } from './types/datasource';

export type ProviderOption = {
  id: string;
  label: string;
  adapter: DatasourceFormValues['type'];
  defaultPort: string;
  defaultSSL: DatasourceFormValues['sslMode'];
  helperText: string;
  iconPath: string;
  urlPattern?: RegExp;
};

export const PROVIDERS: ProviderOption[] = [
  {
    id: 'mssql',
    label: 'SQL Server',
    adapter: 'mssql',
    defaultPort: '1433',
    defaultSSL: 'require',
    helperText: 'Supports SQL authentication for Microsoft SQL Server.',
    iconPath: '/icons/adapters/MSSQL.png',
  },
  {
    id: 'postgres',
    label: 'PostgreSQL',
    adapter: 'postgres',
    defaultPort: '5432',
    defaultSSL: 'require',
    helperText: 'Works for standard PostgreSQL deployments.',
    iconPath: '/icons/adapters/PostgreSQL.png',
  },
  {
    id: 'supabase',
    label: 'Supabase',
    adapter: 'postgres',
    defaultPort: '6543',
    defaultSSL: 'require',
    helperText: 'Use the pooled connection string from Supabase.',
    iconPath: '/icons/adapters/Supabase.png',
    urlPattern: /\.supabase\.(com|co)/i,
  },
  {
    id: 'neon',
    label: 'Neon',
    adapter: 'postgres',
    defaultPort: '5432',
    defaultSSL: 'require',
    helperText: 'Use the hostname from Neon connection details.',
    iconPath: '/icons/adapters/Neon.png',
    urlPattern: /\.neon\.tech/i,
  },
  {
    id: 'redshift',
    label: 'Amazon Redshift',
    adapter: 'postgres',
    defaultPort: '5439',
    defaultSSL: 'require',
    helperText: 'Uses the PostgreSQL adapter with Redshift defaults.',
    iconPath: '/icons/adapters/AmazonRedshift.png',
    urlPattern: /\.redshift\.amazonaws\.com/i,
  },
  {
    id: 'cockroachdb',
    label: 'CockroachDB',
    adapter: 'postgres',
    defaultPort: '26257',
    defaultSSL: 'verify-full',
    helperText: 'PostgreSQL wire protocol with CockroachDB defaults.',
    iconPath: '/icons/adapters/CockroachDB.png',
    urlPattern: /cockroachlabs\.cloud/i,
  },
];

export function detectProviderFromUrl(url: string): ProviderOption | undefined {
  return PROVIDERS.find((provider) => provider.urlPattern?.test(url));
}

export const QUERY_TEMPLATE = 'SELECT true AS connected';
