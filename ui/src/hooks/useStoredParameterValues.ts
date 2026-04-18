import { useCallback, useEffect, useState } from 'react';

function readFromStorage(key: string): Record<string, unknown> {
  if (typeof window === 'undefined') return {};
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return {};
  } catch {
    return {};
  }
}

function writeToStorage(key: string, values: Record<string, unknown>): void {
  if (typeof window === 'undefined') return;
  try {
    if (Object.keys(values).length === 0) {
      window.localStorage.removeItem(key);
      return;
    }
    window.localStorage.setItem(key, JSON.stringify(values));
  } catch {
    // Ignore quota/serialization errors — the form still works in-memory.
  }
}

export function useStoredParameterValues(
  key: string,
): [Record<string, unknown>, (values: Record<string, unknown>) => void] {
  const [values, setValues] = useState<Record<string, unknown>>(() => readFromStorage(key));

  useEffect(() => {
    setValues(readFromStorage(key));
  }, [key]);

  const updateValues = useCallback(
    (next: Record<string, unknown>) => {
      setValues(next);
      writeToStorage(key, next);
    },
    [key],
  );

  return [values, updateValues];
}
