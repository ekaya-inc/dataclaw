import { useEffect, useRef, useState } from 'react';

import { validateQuery } from '../services/api';
import type { QueryParameter } from '../types/query';

export type ValidationStatus = 'idle' | 'validating' | 'valid' | 'invalid';

export interface SqlValidationState {
  status: ValidationStatus;
  error?: string | undefined;
  warnings?: string[] | undefined;
}

export interface UseSqlValidationOptions {
  sql: string;
  parameters: QueryParameter[];
  allowsModification: boolean;
  debounceMs?: number;
}

export function useSqlValidation({
  sql,
  parameters,
  allowsModification,
  debounceMs = 500,
}: UseSqlValidationOptions): SqlValidationState & { reset: () => void } {
  const [state, setState] = useState<SqlValidationState>({ status: 'idle' });
  const requestIdRef = useRef(0);

  useEffect(() => {
    const trimmed = sql.trim();
    if (!trimmed) {
      requestIdRef.current++;
      const queuedReset = setTimeout(() => setState({ status: 'idle' }), 0);
      return () => clearTimeout(queuedReset);
    }
    const requestId = ++requestIdRef.current;
    const timer = setTimeout(() => {
      setState({ status: 'validating' });
      void (async () => {
        try {
          const result = await validateQuery(trimmed, parameters, allowsModification);
          if (requestId !== requestIdRef.current) return;
          if (result.valid) {
            setState({ status: 'valid', warnings: result.warnings });
          } else {
            setState({ status: 'invalid', error: result.message ?? 'SQL validation failed.', warnings: result.warnings });
          }
        } catch (error) {
          if (requestId !== requestIdRef.current) return;
          setState({ status: 'invalid', error: error instanceof Error ? error.message : 'SQL validation failed.' });
        }
      })();
    }, debounceMs);

    return () => clearTimeout(timer);
  }, [sql, parameters, allowsModification, debounceMs]);

  const reset = (): void => {
    requestIdRef.current++;
    setState({ status: 'idle' });
  };

  return { ...state, reset };
}
