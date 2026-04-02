import { AlertTriangle, Loader2, RefreshCw } from 'lucide-react';

interface QueryStatusProps {
  isLoading: boolean;
  isError: boolean;
  error?: Error | null;
  refetch?: () => void;
  loadingText?: string;
  emptyText?: string;
  isEmpty?: boolean;
}

/** Shared loading / error / empty state for all views */
export default function QueryStatus({
  isLoading,
  isError,
  error,
  refetch,
  loadingText = 'Loading...',
  emptyText = 'No data available.',
  isEmpty = false,
}: QueryStatusProps) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center gap-3 py-16">
        <Loader2 size={20} className="animate-spin text-indigo-400" />
        <span className="text-sm text-[#8888a0]">{loadingText}</span>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-2xl border border-red-500/20 bg-red-500/5 p-8">
        <AlertTriangle size={28} className="text-red-400" />
        <div className="text-center">
          <h3 className="text-sm font-semibold text-red-300">Failed to load data</h3>
          <p className="mt-1 max-w-sm text-xs text-[#8888a0]">
            {error?.message ?? 'An unknown error occurred.'}
          </p>
        </div>
        {refetch && (
          <button
            type="button"
            onClick={() => refetch()}
            className="flex items-center gap-2 rounded-xl bg-red-500/10 px-4 py-2 text-sm text-red-300 transition hover:bg-red-500/20"
          >
            <RefreshCw size={14} />
            Retry
          </button>
        )}
      </div>
    );
  }

  if (isEmpty) {
    return (
      <div className="card flex items-center justify-center p-8 text-sm text-[#8888a0]">
        {emptyText}
      </div>
    );
  }

  return null;
}
