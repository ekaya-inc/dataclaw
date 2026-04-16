export function StatusBanner({ tone = 'info', message }: { tone?: 'info' | 'success' | 'danger'; message: string }): JSX.Element {
  const classes = {
    info: 'border-blue-200 bg-blue-50 text-blue-800',
    success: 'border-emerald-200 bg-emerald-50 text-emerald-800',
    danger: 'border-red-200 bg-red-50 text-red-800',
  } as const;

  return <div className={`rounded-xl border px-4 py-3 text-sm ${classes[tone]}`}>{message}</div>;
}
