import type { QueryParameter } from '../types/query';

import { Input } from './ui/Input';
import { Label } from './ui/Label';

function inputValue(value: unknown, fallback: unknown): string {
  const effective = value ?? fallback;
  if (Array.isArray(effective)) {
    return effective.map((item) => String(item)).join(', ');
  }
  if (effective === null || effective === undefined) {
    return '';
  }
  return String(effective);
}

function checkboxValue(value: unknown, fallback: unknown): boolean {
  const effective = value ?? fallback;
  return effective === true || effective === 'true' || effective === 1 || effective === '1';
}

export function ParameterInputForm({
  parameters,
  values,
  onChange,
}: {
  parameters: QueryParameter[];
  values: Record<string, unknown>;
  onChange: (values: Record<string, unknown>) => void;
}): JSX.Element | null {
  if (parameters.length === 0) {
    return null;
  }

  const requiredParameters = parameters.filter((parameter) => parameter.required);
  const optionalParameters = parameters.filter((parameter) => !parameter.required);

  const updateValue = (name: string, value: unknown): void => {
    onChange({ ...values, [name]: value });
  };

  const renderParameter = (parameter: QueryParameter): JSX.Element => (
    <div key={parameter.name} className="space-y-2">
      <Label htmlFor={`execute-parameter-${parameter.name}`}>
        {parameter.name}
        {parameter.required ? <span className="ml-1 text-red-500">*</span> : null}
        <span className="ml-2 text-xs font-normal text-text-tertiary">({parameter.type})</span>
      </Label>
      {parameter.description ? <p className="text-xs text-text-secondary">{parameter.description}</p> : null}
      {parameter.type === 'boolean' ? (
        <label className="flex items-center gap-2 text-sm text-text-secondary">
          <input
            id={`execute-parameter-${parameter.name}`}
            type="checkbox"
            checked={checkboxValue(values[parameter.name], parameter.default)}
            onChange={(event) => updateValue(parameter.name, event.target.checked)}
          />
          <span>{checkboxValue(values[parameter.name], parameter.default) ? 'true' : 'false'}</span>
        </label>
      ) : (
        <Input
          id={`execute-parameter-${parameter.name}`}
          type={parameter.type === 'integer' || parameter.type === 'decimal' ? 'number' : parameter.type === 'date' ? 'date' : 'text'}
          step={parameter.type === 'decimal' ? '0.01' : parameter.type === 'integer' ? '1' : undefined}
          value={inputValue(values[parameter.name], parameter.default)}
          onChange={(event) => updateValue(parameter.name, event.target.value)}
          placeholder={
            parameter.type === 'timestamp'
              ? 'RFC3339 timestamp, e.g. 2026-04-16T12:30:00Z'
              : parameter.type === 'string[]'
                ? 'Comma-separated values, e.g. pending,active'
                : parameter.type === 'integer[]'
                  ? 'Comma-separated integers, e.g. 1,2,3'
                  : parameter.default !== undefined && parameter.default !== null
                    ? `Default: ${inputValue(undefined, parameter.default)}`
                    : parameter.required
                      ? 'Required'
                      : 'Optional'
          }
        />
      )}
    </div>
  );

  return (
    <div className="space-y-4 rounded-2xl border border-border-light bg-surface-secondary p-4">
      <div>
        <h3 className="text-sm font-semibold text-text-primary">Execution parameters</h3>
        <p className="text-sm text-text-secondary">Enter values for the saved query before it runs. Defaults are used when present.</p>
      </div>
      {requiredParameters.length > 0 ? <div className="space-y-4">{requiredParameters.map(renderParameter)}</div> : null}
      {optionalParameters.length > 0 ? <div className="space-y-4">{optionalParameters.map(renderParameter)}</div> : null}
    </div>
  );
}
