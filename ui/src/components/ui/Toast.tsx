import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { CheckCircle2, AlertCircle, X } from 'lucide-react';

import { cn } from '../../utils/cn';

export type ToastVariant = 'success' | 'error' | 'info';

export interface ToastOptions {
  title: string;
  description?: string | undefined;
  variant?: ToastVariant | undefined;
  durationMs?: number | undefined;
}

interface Toast extends ToastOptions {
  id: string;
}

interface ToastContextValue {
  toast: (options: ToastOptions) => void;
  dismiss: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);
const DEFAULT_DURATION_MS = 4000;

export function ToastProvider({ children }: { children: ReactNode }): JSX.Element {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: string): void => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
  }, []);

  const toast = useCallback(
    (options: ToastOptions): void => {
      const id = crypto.randomUUID();
      const duration = options.durationMs ?? DEFAULT_DURATION_MS;
      setToasts((current) => [...current, { ...options, id }]);
      const timer = setTimeout(() => dismiss(id), duration);
      timers.current.set(id, timer);
    },
    [dismiss],
  );

  useEffect(() => {
    const currentTimers = timers.current;
    return () => {
      for (const timer of currentTimers.values()) {
        clearTimeout(timer);
      }
      currentTimers.clear();
    };
  }, []);

  const value = useMemo<ToastContextValue>(() => ({ toast, dismiss }), [toast, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        aria-live="polite"
        aria-atomic="true"
        className="pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col gap-2"
      >
        {toasts.map((entry) => (
          <ToastItem key={entry.id} toast={entry} onDismiss={() => dismiss(entry.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }): JSX.Element {
  const variant = toast.variant ?? 'info';
  const Icon = variant === 'success' ? CheckCircle2 : variant === 'error' ? AlertCircle : CheckCircle2;
  return (
    <div
      role="status"
      className={cn(
        'pointer-events-auto flex min-w-[280px] max-w-md items-start gap-3 rounded-xl border px-4 py-3 shadow-lg',
        variant === 'success' && 'border-emerald-200 bg-emerald-50 text-emerald-900',
        variant === 'error' && 'border-red-200 bg-red-50 text-red-900',
        variant === 'info' && 'border-slate-200 bg-white text-text-primary',
      )}
    >
      <Icon className="mt-0.5 h-5 w-5 shrink-0" aria-hidden />
      <div className="flex-1">
        <div className="text-sm font-semibold">{toast.title}</div>
        {toast.description ? <div className="mt-1 text-sm opacity-90">{toast.description}</div> : null}
      </div>
      <button
        type="button"
        onClick={onDismiss}
        className="text-current opacity-60 transition-opacity hover:opacity-100"
        aria-label="Dismiss notification"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

export function useToast(): ToastContextValue {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
