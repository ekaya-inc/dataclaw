import { detectProviderFromUrl } from '../constants';
import type { SSLMode } from '../types/datasource';

export interface ParsedConnectionString {
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
  sslMode: SSLMode;
  providerId: string | undefined;
}

const SSL_MODES: ReadonlyArray<SSLMode> = [
  'disable',
  'allow',
  'prefer',
  'require',
  'verify-ca',
  'verify-full',
];

function toSSLMode(value: string | null): SSLMode {
  if (!value) return 'require';
  const lower = value.toLowerCase();
  return (SSL_MODES as readonly string[]).includes(lower) ? (lower as SSLMode) : 'require';
}

// Accepts postgres://... and postgresql://... URIs. Values are URL-decoded so
// `user%40domain` becomes `user@domain`.
export function parsePostgresUrl(url: string): ParsedConnectionString | null {
  const match = url.match(
    /^postgres(?:ql)?:\/\/(?:([^:@]+)(?::([^@]*))?@)?([^:/]+)(?::(\d+))?(?:\/([^?]+))?(?:\?(.*))?$/,
  );
  if (!match) return null;
  const [, user, password, host, port, database, queryString] = match;
  const params = new URLSearchParams(queryString ?? '');
  const detectedProvider = detectProviderFromUrl(url);
  const defaultPort = detectedProvider ? Number(detectedProvider.defaultPort) : 5432;
  const explicitSSL = params.get('sslmode');
  const defaultSSL = detectedProvider?.defaultSSL ?? 'require';
  return {
    host: host ?? '',
    port: port ? Number(port) : defaultPort,
    user: decodeURIComponent(user ?? ''),
    password: decodeURIComponent(password ?? ''),
    database: database ?? '',
    sslMode: explicitSSL ? toSSLMode(explicitSSL) : defaultSSL,
    providerId: detectedProvider?.id,
  };
}
