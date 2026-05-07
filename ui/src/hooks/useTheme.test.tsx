import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { THEME_STORAGE_KEY, useTheme } from './useTheme';

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

describe('useTheme', () => {
  beforeEach(() => {
    installLocalStorageMock();
    document.documentElement.removeAttribute('data-theme');
  });

  it('defaults to system when no stored value exists', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current[0]).toBe('system');
    expect(document.documentElement.getAttribute('data-theme')).toBe('system');
  });

  it('reads a persisted mode and applies it to the document root', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'dark');

    const { result } = renderHook(() => useTheme());

    expect(result.current[0]).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('persists updates and reflects them on the document root', () => {
    const { result } = renderHook(() => useTheme());

    act(() => {
      result.current[1]('light');
    });

    expect(result.current[0]).toBe('light');
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('falls back to system for unknown stored values', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'turquoise');

    const { result } = renderHook(() => useTheme());

    expect(result.current[0]).toBe('system');
  });

  it('reacts to storage events from other tabs', () => {
    const { result } = renderHook(() => useTheme());

    act(() => {
      window.dispatchEvent(
        new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'dark' }),
      );
    });

    expect(result.current[0]).toBe('dark');

    act(() => {
      window.dispatchEvent(
        new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: null }),
      );
    });

    expect(result.current[0]).toBe('system');
  });
});
