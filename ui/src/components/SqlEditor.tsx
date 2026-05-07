import { MSSQL, PostgreSQL, sql } from '@codemirror/lang-sql';
import { oneDark } from '@codemirror/theme-one-dark';
import CodeMirror from '@uiw/react-codemirror';
import { useMemo } from 'react';

import type { SqlDialect } from '../types/query';

function getDialect(dialect: SqlDialect) {
  return dialect === 'MSSQL' ? MSSQL : PostgreSQL;
}

export function SqlEditor({
  value,
  onChange,
  dialect,
}: {
  value: string;
  onChange: (value: string) => void;
  dialect: SqlDialect;
}): JSX.Element {
  const extensions = useMemo(
    () => [sql({ dialect: getDialect(dialect), upperCaseKeywords: true })],
    [dialect],
  );

  return (
    <div className="overflow-hidden rounded-xl border border-border-light">
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
  );
}
