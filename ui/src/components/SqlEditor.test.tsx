import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { codeMirrorPropsSpy, mssqlDialect, postgresqlDialect, sqlSpy } = vi.hoisted(() => ({
  codeMirrorPropsSpy: vi.fn(),
  mssqlDialect: { name: 'mssql' },
  postgresqlDialect: { name: 'postgresql' },
  sqlSpy: vi.fn((config: unknown) => ({ kind: 'sql-extension', config })),
}));

vi.mock('@codemirror/lang-sql', () => ({
  MSSQL: mssqlDialect,
  PostgreSQL: postgresqlDialect,
  sql: sqlSpy,
}));

vi.mock('@codemirror/theme-one-dark', () => ({
  oneDark: { name: 'one-dark-theme' },
}));

vi.mock('@uiw/react-codemirror', () => ({
  default: (props: { onChange: (value: string) => void; value: string }) => {
    codeMirrorPropsSpy(props);

    return (
      <label>
        SQL editor
        <textarea
          aria-label="SQL editor"
          value={props.value}
          onChange={(event) => props.onChange(event.target.value)}
        />
      </label>
    );
  },
}));

import { SqlEditor } from './SqlEditor';

function SqlEditorHarness({
  dialect = 'PostgreSQL',
  validationError,
  validationStatus = 'idle',
}: {
  dialect?: 'PostgreSQL' | 'MSSQL';
  validationError?: string;
  validationStatus?: 'idle' | 'validating' | 'valid' | 'invalid';
}): JSX.Element {
  const [value, setValue] = useState('SELECT 1');

  return (
    <SqlEditor
      value={value}
      onChange={setValue}
      dialect={dialect}
      validationStatus={validationStatus}
      validationError={validationError}
    />
  );
}

describe('SqlEditor', () => {
  beforeEach(() => {
    codeMirrorPropsSpy.mockClear();
    sqlSpy.mockClear();
  });

  it('lets users edit SQL and configures the PostgreSQL dialect extension', async () => {
    const user = userEvent.setup();

    render(<SqlEditorHarness />);

    const editor = screen.getByRole('textbox', { name: /sql editor/i });

    expect(editor).toHaveValue('SELECT 1');
    expect(screen.queryByText(/sql is valid\./i)).not.toBeInTheDocument();
    expect(document.querySelector('.border-border-light')).toBeInTheDocument();

    await user.clear(editor);
    await user.type(editor, 'SELECT 2');

    expect(editor).toHaveValue('SELECT 2');
    expect(sqlSpy).toHaveBeenCalledWith({
      dialect: postgresqlDialect,
      upperCaseKeywords: true,
    });
  });

  it('switches dialect configuration when MSSQL is selected', () => {
    const onChange = vi.fn();
    const { rerender } = render(<SqlEditor value="SELECT 1" onChange={onChange} dialect="PostgreSQL" />);

    expect(sqlSpy).toHaveBeenLastCalledWith({
      dialect: postgresqlDialect,
      upperCaseKeywords: true,
    });

    rerender(<SqlEditor value="SELECT 1" onChange={onChange} dialect="MSSQL" />);

    expect(sqlSpy).toHaveBeenLastCalledWith({
      dialect: mssqlDialect,
      upperCaseKeywords: true,
    });
  });

  it('shows invalid feedback for users when validation fails', () => {
    render(
      <SqlEditor
        value="SELECT"
        onChange={vi.fn()}
        dialect="PostgreSQL"
        validationStatus="invalid"
        validationError="Syntax error near end of input."
      />,
    );

    expect(screen.getByText('Syntax error near end of input.')).toBeInTheDocument();
    expect(document.querySelector('.border-red-500')).toBeInTheDocument();
  });

  it('shows valid and validating states with visible status text', () => {
    const { rerender } = render(
      <SqlEditor value="SELECT 1" onChange={vi.fn()} dialect="PostgreSQL" validationStatus="valid" />,
    );

    expect(screen.getByText('SQL is valid.')).toBeInTheDocument();
    expect(document.querySelector('.border-emerald-500')).toBeInTheDocument();

    rerender(
      <SqlEditor value="SELECT 1" onChange={vi.fn()} dialect="PostgreSQL" validationStatus="validating" />,
    );

    expect(screen.getByText('Validating SQL…')).toBeInTheDocument();
    expect(document.querySelector('.border-amber-500')).toBeInTheDocument();
  });
});
