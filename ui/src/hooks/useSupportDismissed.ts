import { useCallback, useEffect, useState } from 'react';

const STORAGE_KEY = 'dataclaw:support-dismissed';

function read(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

export function useSupportDismissed(): [boolean, () => void] {
  const [dismissed, setDismissed] = useState<boolean>(() => read());

  useEffect(() => {
    const onStorage = (event: StorageEvent): void => {
      if (event.key === STORAGE_KEY) setDismissed(read());
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const dismiss = useCallback((): void => {
    try {
      window.localStorage.setItem(STORAGE_KEY, '1');
    } catch {
      // ignore quota errors; in-memory state still updates
    }
    setDismissed(true);
  }, []);

  return [dismissed, dismiss];
}
