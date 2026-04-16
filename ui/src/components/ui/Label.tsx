import type { LabelHTMLAttributes } from 'react';

import { cn } from '../../utils/cn';

export function Label({ className, ...props }: LabelHTMLAttributes<HTMLLabelElement>): JSX.Element {
  return <label className={cn('text-sm font-medium text-text-primary', className)} {...props} />;
}
