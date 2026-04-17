import { describe, expect, it } from 'vitest';

import { parsePostgresUrl } from './connectionString';

describe('parsePostgresUrl', () => {
  it('uses CockroachDB defaults when sslmode is omitted', () => {
    const parsed = parsePostgresUrl('postgresql://roach:secret@cluster-name.us-east1.cockroachlabs.cloud/defaultdb');

    expect(parsed).toMatchObject({
      providerId: 'cockroachdb',
      port: 26257,
      sslMode: 'verify-full',
    });
  });

  it('keeps the standard PostgreSQL default sslmode when omitted', () => {
    const parsed = parsePostgresUrl('postgresql://postgres:secret@db.example.com/appdb');

    expect(parsed).toMatchObject({
      providerId: undefined,
      port: 5432,
      sslMode: 'require',
    });
  });

  it('lets an explicit sslmode override the detected provider default', () => {
    const parsed = parsePostgresUrl('postgresql://roach:secret@cluster-name.us-east1.cockroachlabs.cloud/defaultdb?sslmode=require');

    expect(parsed).toMatchObject({
      providerId: 'cockroachdb',
      sslMode: 'require',
    });
  });
});
