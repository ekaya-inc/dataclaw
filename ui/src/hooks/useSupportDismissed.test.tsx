import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useSupportDismissed } from './useSupportDismissed';

const STORAGE_KEY = 'dataclaw:support-dismissed';

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

describe('useSupportDismissed', () => {
  beforeEach(() => {
    installLocalStorageMock();
  });

  it('starts from persisted dismissal state and persists dismiss actions', () => {
    window.localStorage.setItem(STORAGE_KEY, '1');

    const persisted = renderHook(() => useSupportDismissed());

    expect(persisted.result.current[0]).toBe(true);

    window.localStorage.removeItem(STORAGE_KEY);

    const fresh = renderHook(() => useSupportDismissed());

    expect(fresh.result.current[0]).toBe(false);

    act(() => {
      fresh.result.current[1]();
    });

    expect(fresh.result.current[0]).toBe(true);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('1');
  });

  it('reacts to storage events from other tabs', () => {
    const { result } = renderHook(() => useSupportDismissed());

    expect(result.current[0]).toBe(false);

    act(() => {
      window.localStorage.setItem(STORAGE_KEY, '1');
      window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEY, newValue: '1' }));
    });

    expect(result.current[0]).toBe(true);

    act(() => {
      window.localStorage.removeItem(STORAGE_KEY);
      window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEY, newValue: null }));
    });

    expect(result.current[0]).toBe(false);
  });

  it('falls back safely when storage access fails', () => {
    const getItemSpy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('storage unavailable');
    });

    const { result } = renderHook(() => useSupportDismissed());

    expect(result.current[0]).toBe(false);

    getItemSpy.mockRestore();

    const setItemSpy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });

    act(() => {
      result.current[1]();
    });

    expect(result.current[0]).toBe(true);
    expect(setItemSpy).toHaveBeenCalledWith(STORAGE_KEY, '1');
  });
});
