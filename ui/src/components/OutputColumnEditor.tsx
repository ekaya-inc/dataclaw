import { Plus, Trash2 } from 'lucide-react';

import type { OutputColumn } from '../types/query';

import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Label } from './ui/Label';

export function OutputColumnEditor({
  outputColumns,
  onChange,
}: {
  outputColumns: OutputColumn[];
  onChange: (outputColumns: OutputColumn[]) => void;
}): JSX.Element {
  const updateColumn = (index: number, field: keyof OutputColumn, value: string): void => {
    onChange(
      outputColumns.map((column, currentIndex) =>
        currentIndex === index ? { ...column, [field]: value } : column,
      ),
    );
  };

  return (
    <div className="space-y-4 rounded-2xl border border-border-light p-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-text-primary">Output columns</h3>
          <p className="text-sm text-text-secondary">
            Describe the columns this query returns. Documents the shape for agents—no effect on execution.
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onChange([...outputColumns, { name: '', type: '', description: '' }])}
        >
          <Plus className="h-4 w-4" />
          Add column
        </Button>
      </div>
      {outputColumns.length === 0 ? (
        <p className="text-sm text-text-secondary">No output columns described yet.</p>
      ) : (
        <div className="space-y-3">
          {outputColumns.map((column, index) => (
            <div key={`${column.name}-${index}`} className="grid gap-3 rounded-xl border border-border-light p-4 md:grid-cols-12">
              <div className="md:col-span-3">
                <Label htmlFor={`output-name-${index}`}>Name</Label>
                <Input
                  id={`output-name-${index}`}
                  value={column.name}
                  onChange={(event) => updateColumn(index, 'name', event.target.value)}
                  placeholder="customer_id"
                />
              </div>
              <div className="md:col-span-2">
                <Label htmlFor={`output-type-${index}`}>Type</Label>
                <Input
                  id={`output-type-${index}`}
                  value={column.type}
                  onChange={(event) => updateColumn(index, 'type', event.target.value)}
                  placeholder="uuid"
                />
              </div>
              <div className="md:col-span-6">
                <Label htmlFor={`output-description-${index}`}>Description</Label>
                <Input
                  id={`output-description-${index}`}
                  value={column.description}
                  onChange={(event) => updateColumn(index, 'description', event.target.value)}
                  placeholder="What this column represents"
                />
              </div>
              <div className="flex items-end justify-end md:col-span-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-9 w-9 p-0"
                  title="Remove"
                  aria-label="Remove column"
                  onClick={() => onChange(outputColumns.filter((_, currentIndex) => currentIndex !== index))}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
