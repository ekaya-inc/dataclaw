import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useStoredParameterValues } from './useStoredParameterValues';

function installLocalStorageMock(): void {
  const store = new Map<string, string>();

  const localStorageMock = {
    clear: vi.fn(() => {
      store.clear();
    }),
    getItem: vi.fn((key: string) => store.get(key) ?? null),
    key: vi.fn((index: number) => Array.from(store.keys())[index] ?? null),
    removeItem: vi.fn((key: string) => {
      store.delete(key);
    }),
    setItem: vi.fn((key: string, value: string) => {
      store.set(key, value);
    }),
    get length() {
      return store.size;
    },
  } as Storage;

  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: localStorageMock,
  });
}

describe('useStoredParameterValues', () => {
  beforeEach(() => {
    installLocalStorageMock();
  });

  it('loads persisted values and writes updates back to storage', () => {
    window.localStorage.setItem('query-a', JSON.stringify({ limit: 10, enabled: true }));

    const { result } = renderHook(() => useStoredParameterValues('query-a'));

    expect(result.current[0]).toEqual({ limit: 10, enabled: true });

    act(() => {
      result.current[1]({ limit: 25, enabled: false });
    });

    expect(result.current[0]).toEqual({ limit: 25, enabled: false });
    expect(JSON.parse(window.localStorage.getItem('query-a') ?? 'null')).toEqual({
      limit: 25,
      enabled: false,
    });
  });

  it('re-reads when the key changes and removes storage for empty values', async () => {
    window.localStorage.setItem('query-a', JSON.stringify({ schema: 'public' }));
    window.localStorage.setItem('query-b', '["not-an-object"]');

    const { result, rerender } = renderHook(
      ({ storageKey }) => useStoredParameterValues(storageKey),
      { initialProps: { storageKey: 'query-a' } },
    );

    expect(result.current[0]).toEqual({ schema: 'public' });

    rerender({ storageKey: 'query-b' });

    await waitFor(() => {
      expect(result.current[0]).toEqual({});
    });

    act(() => {
      result.current[1]({});
    });

    expect(window.localStorage.getItem('query-b')).toBeNull();
  });

  it('falls back to in-memory state when storage reads or writes fail', () => {
    const getItemSpy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('storage unavailable');
    });

    const { result } = renderHook(() => useStoredParameterValues('query-c'));

    expect(result.current[0]).toEqual({});

    getItemSpy.mockRestore();

    const setItemSpy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });

    act(() => {
      result.current[1]({ draft: 'SELECT 1' });
    });

    expect(result.current[0]).toEqual({ draft: 'SELECT 1' });
    expect(setItemSpy).toHaveBeenCalledWith('query-c', JSON.stringify({ draft: 'SELECT 1' }));
  });
});
