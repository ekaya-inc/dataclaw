import { useMemo, useState } from 'react';

import type { QueryParameter } from '../types/query';

import { hasRequiredExecutionValues, ParameterInputForm, pruneUnknownParameterValues } from './ParameterInputForm';
import { Button } from './ui/Button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/Dialog';

interface ParameterInputDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  parameters: QueryParameter[];
  initialValues: Record<string, unknown>;
  title: string;
  description?: string;
  submitLabel?: string;
  submitting?: boolean;
  onSubmit: (values: Record<string, unknown>) => void;
}

interface ContentProps {
  parameters: QueryParameter[];
  initialValues: Record<string, unknown>;
  title: string;
  description?: string;
  submitLabel: string;
  submitting: boolean;
  onCancel: () => void;
  onSubmit: (values: Record<string, unknown>) => void;
}

function ParameterInputDialogContent({
  parameters,
  initialValues,
  title,
  description,
  submitLabel,
  submitting,
  onCancel,
  onSubmit,
}: ContentProps): JSX.Element {
  const [values, setValues] = useState<Record<string, unknown>>(() => pruneUnknownParameterValues(initialValues, parameters));
  const canSubmit = useMemo(() => hasRequiredExecutionValues(parameters, values), [parameters, values]);

  return (
    <DialogContent className="max-w-2xl">
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        {description ? <DialogDescription>{description}</DialogDescription> : null}
      </DialogHeader>
      <ParameterInputForm parameters={parameters} values={values} onChange={setValues} />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onCancel} disabled={submitting}>
          Cancel
        </Button>
        <Button type="button" onClick={() => onSubmit(values)} disabled={!canSubmit || submitting}>
          {submitLabel}
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}

export function ParameterInputDialog({
  open,
  onOpenChange,
  parameters,
  initialValues,
  title,
  description,
  submitLabel = 'Submit',
  submitting = false,
  onSubmit,
}: ParameterInputDialogProps): JSX.Element {
  const handleOpenChange = (next: boolean): void => {
    if (submitting) return;
    onOpenChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {open ? (
        <ParameterInputDialogContent
          parameters={parameters}
          initialValues={initialValues}
          title={title}
          {...(description ? { description } : {})}
          submitLabel={submitLabel}
          submitting={submitting}
          onCancel={() => handleOpenChange(false)}
          onSubmit={onSubmit}
        />
      ) : null}
    </Dialog>
  );
}
