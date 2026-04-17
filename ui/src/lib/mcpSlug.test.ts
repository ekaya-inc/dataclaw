import { describe, expect, it } from 'vitest';

import { toMCPKey } from './mcpSlug';

describe('toMCPKey', () => {
  it('lowercases simple input', () => {
    expect(toMCPKey('MarketingBot')).toBe('marketingbot');
  });

  it('replaces spaces and punctuation with underscores', () => {
    expect(toMCPKey('My MCP Agent')).toBe('my_mcp_agent');
    expect(toMCPKey('Sales/Bot v2')).toBe('sales_bot_v2');
  });

  it('collapses runs of separators', () => {
    expect(toMCPKey('foo   bar---baz')).toBe('foo_bar_baz');
  });

  it('trims leading and trailing separators', () => {
    expect(toMCPKey('  My Agent  ')).toBe('my_agent');
    expect(toMCPKey('---Agent---')).toBe('agent');
  });

  it('falls back to "agent" for empty or all-non-alphanumeric input', () => {
    expect(toMCPKey('')).toBe('agent');
    expect(toMCPKey('!!!')).toBe('agent');
  });
});
