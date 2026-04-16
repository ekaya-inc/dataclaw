import { Plus, Trash2 } from 'lucide-react';

import type { ParameterType, QueryParameter } from '../types/query';

import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Label } from './ui/Label';

const PARAMETER_TYPES: ParameterType[] = ['string', 'integer', 'decimal', 'boolean', 'date', 'timestamp', 'uuid'];

export function ParameterEditor({ parameters, onChange }: { parameters: QueryParameter[]; onChange: (parameters: QueryParameter[]) => void }): JSX.Element {
  const updateParameter = (index: number, field: keyof QueryParameter, value: string | boolean | null): void => {
    onChange(
      parameters.map((parameter, currentIndex) =>
        currentIndex === index ? { ...parameter, [field]: value } : parameter,
      ),
    );
  };

  return (
    <div className="space-y-4 rounded-2xl border border-border-light p-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-text-primary">Parameters</h3>
          <p className="text-sm text-text-secondary">Optional placeholders for approved queries.</p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            onChange([
              ...parameters,
              { name: '', type: 'string', description: '', required: true, default: null },
            ])
          }
        >
          <Plus className="h-4 w-4" />
          Add parameter
        </Button>
      </div>
      {parameters.length === 0 ? (
        <p className="text-sm text-text-secondary">No parameters defined.</p>
      ) : (
        <div className="space-y-3">
          {parameters.map((parameter, index) => (
            <div key={`${parameter.name}-${index}`} className="grid gap-3 rounded-xl border border-border-light p-4 md:grid-cols-12">
              <div className="md:col-span-3">
                <Label htmlFor={`parameter-name-${index}`}>Name</Label>
                <Input
                  id={`parameter-name-${index}`}
                  value={parameter.name}
                  onChange={(event) => updateParameter(index, 'name', event.target.value)}
                  placeholder="customer_id"
                />
              </div>
              <div className="md:col-span-2">
                <Label htmlFor={`parameter-type-${index}`}>Type</Label>
                <select
                  id={`parameter-type-${index}`}
                  className="flex h-10 w-full rounded-lg border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary"
                  value={parameter.type}
                  onChange={(event) => updateParameter(index, 'type', event.target.value)}
                >
                  {PARAMETER_TYPES.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </select>
              </div>
              <div className="md:col-span-4">
                <Label htmlFor={`parameter-description-${index}`}>Description</Label>
                <Input
                  id={`parameter-description-${index}`}
                  value={parameter.description}
                  onChange={(event) => updateParameter(index, 'description', event.target.value)}
                  placeholder="Why this parameter exists"
                />
              </div>
              <div className="md:col-span-2">
                <Label htmlFor={`parameter-default-${index}`}>Default</Label>
                <Input
                  id={`parameter-default-${index}`}
                  value={parameter.default ?? ''}
                  onChange={(event) => updateParameter(index, 'default', event.target.value || null)}
                  placeholder="optional"
                />
              </div>
              <div className="flex items-end justify-between gap-3 md:col-span-1 md:flex-col md:justify-end">
                <label className="flex items-center gap-2 text-sm text-text-secondary">
                  <input
                    type="checkbox"
                    checked={parameter.required}
                    onChange={(event) => updateParameter(index, 'required', event.target.checked)}
                  />
                  Required
                </label>
                <Button type="button" variant="ghost" size="sm" onClick={() => onChange(parameters.filter((_, currentIndex) => currentIndex !== index))}>
                  <Trash2 className="h-4 w-4" />
                  Remove
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
