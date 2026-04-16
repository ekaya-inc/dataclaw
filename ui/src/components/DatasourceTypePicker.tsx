import { PROVIDERS } from '../constants';

export function DatasourceTypePicker({
  onSelect,
}: {
  onSelect: (providerId: string) => void;
}): JSX.Element {
  return (
    <div className="space-y-4">
      <p className="text-sm text-text-secondary">Choose a datasource type to connect.</p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {PROVIDERS.map((provider) => (
          <button
            key={provider.id}
            type="button"
            aria-label={provider.label}
            onClick={() => onSelect(provider.id)}
            className="group flex items-center gap-3 rounded-2xl border border-border-light bg-surface-primary p-5 text-left shadow-sm transition-all hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-purple"
          >
            <img
              src={provider.iconPath}
              alt=""
              className="h-11 w-11 shrink-0 object-contain"
              loading="lazy"
            />
            <div className="min-w-0">
              <h3 className="truncate text-sm font-semibold text-text-primary">{provider.label}</h3>
              <p className="mt-1 line-clamp-2 text-xs text-text-tertiary">{provider.helperText}</p>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
