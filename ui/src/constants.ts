import type { DatasourceAdapterInfo, DatasourceFormValues } from './types/datasource';

export type ProviderOption = {
  id: string;
  label: string;
  adapter: DatasourceFormValues['type'];
  defaultPort: string;
  defaultSSL: DatasourceFormValues['sslMode'];
  helperText: string;
  iconPath?: string;
  urlPattern?: RegExp;
};

const PROVIDER_PRESETS: ProviderOption[] = [
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

const ADAPTER_ICON_PATHS: Record<string, string> = {
  mssql: '/icons/adapters/MSSQL.png',
  postgres: '/icons/adapters/PostgreSQL.png',
};

const ADAPTER_DEFAULT_PORTS: Record<string, string> = {
  mssql: '1433',
  postgres: '5432',
};

const ADAPTER_DEFAULT_SSL: Record<string, DatasourceFormValues['sslMode']> = {
  mssql: 'require',
  postgres: 'require',
};

function adapterIconPath(adapter: DatasourceAdapterInfo): string | undefined {
  const candidates = [adapter.icon, adapter.type]
    .filter((value): value is string => Boolean(value))
    .map((value) => value.toLowerCase());
  for (const candidate of candidates) {
    if (candidate in ADAPTER_ICON_PATHS) {
      return ADAPTER_ICON_PATHS[candidate];
    }
  }
  return undefined;
}

export function buildProviderOptions(adapterTypes: DatasourceAdapterInfo[]): ProviderOption[] {
  const enabledAdapters = new Map(adapterTypes.map((adapter) => [adapter.type, adapter]));
  const options = PROVIDER_PRESETS.filter((provider) => enabledAdapters.has(provider.adapter));
  const existingIDs = new Set(options.map((provider) => provider.id));

  for (const adapter of adapterTypes) {
    if (existingIDs.has(adapter.type)) continue;
    const iconPath = adapterIconPath(adapter);
    options.push({
      id: adapter.type,
      label: adapter.displayName,
      adapter: adapter.type,
      defaultPort: ADAPTER_DEFAULT_PORTS[adapter.type] ?? '',
      defaultSSL: ADAPTER_DEFAULT_SSL[adapter.type] ?? 'require',
      helperText: adapter.description ?? `Connect to ${adapter.displayName}.`,
      ...(iconPath ? { iconPath } : {}),
    });
  }

  return options;
}

export function detectProviderFromUrl(url: string): ProviderOption | undefined {
  return PROVIDER_PRESETS.find((provider) => provider.urlPattern?.test(url));
}

export const QUERY_TEMPLATE = 'SELECT true AS connected';
