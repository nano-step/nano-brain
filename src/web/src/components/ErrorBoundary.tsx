import { Component, type ErrorInfo, type PropsWithChildren } from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<PropsWithChildren<{ fallbackTitle?: string }>, ErrorBoundaryState> {
  constructor(props: PropsWithChildren<{ fallbackTitle?: string }>) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center gap-4 rounded-2xl border border-red-500/20 bg-red-500/5 p-8">
          <AlertTriangle size={32} className="text-red-400" />
          <div className="text-center">
            <h2 className="text-lg font-semibold text-red-300">
              {this.props.fallbackTitle ?? 'Something went wrong'}
            </h2>
            <p className="mt-2 max-w-md text-sm text-[#8888a0]">
              {this.state.error?.message ?? 'An unexpected error occurred while rendering this view.'}
            </p>
          </div>
          <button
            type="button"
            onClick={() => this.setState({ hasError: false, error: null })}
            className="flex items-center gap-2 rounded-xl bg-red-500/10 px-4 py-2 text-sm text-red-300 transition hover:bg-red-500/20"
          >
            <RefreshCw size={14} />
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
