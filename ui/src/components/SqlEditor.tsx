import { MSSQL, PostgreSQL, sql } from '@codemirror/lang-sql';
import { oneDark } from '@codemirror/theme-one-dark';
import CodeMirror from '@uiw/react-codemirror';
import { useMemo } from 'react';

import type { SqlDialect } from '../types/query';

type ValidationStatus = 'idle' | 'validating' | 'valid' | 'invalid';

function getDialect(dialect: SqlDialect) {
  return dialect === 'MSSQL' ? MSSQL : PostgreSQL;
}

function getBorderClass(status: ValidationStatus): string {
  switch (status) {
    case 'valid':
      return 'border-emerald-500';
    case 'invalid':
      return 'border-red-500';
    case 'validating':
      return 'border-amber-500';
    default:
      return 'border-border-light';
  }
}

export function SqlEditor({
  value,
  onChange,
  dialect,
  validationStatus = 'idle',
  validationError,
}: {
  value: string;
  onChange: (value: string) => void;
  dialect: SqlDialect;
  validationStatus?: ValidationStatus;
  validationError?: string | undefined;
}): JSX.Element {
  const extensions = useMemo(
    () => [sql({ dialect: getDialect(dialect), upperCaseKeywords: true })],
    [dialect],
  );

  return (
    <div className="space-y-2">
      <div className={`overflow-hidden rounded-xl border ${getBorderClass(validationStatus)}`}>
        <CodeMirror
          value={value}
          height="280px"
          extensions={extensions}
          theme={oneDark}
          onChange={onChange}
          basicSetup={{
            autocompletion: true,
            bracketMatching: true,
            lineNumbers: true,
            highlightActiveLine: true,
          }}
        />
      </div>
      {validationStatus === 'invalid' && validationError ? (
        <p className="text-sm text-red-600">{validationError}</p>
      ) : null}
      {validationStatus === 'valid' ? <p className="text-sm text-emerald-600">SQL is valid.</p> : null}
      {validationStatus === 'validating' ? <p className="text-sm text-amber-600">Validating SQL…</p> : null}
    </div>
  );
}
