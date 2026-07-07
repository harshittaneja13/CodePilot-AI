import { Component, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * ErrorBoundary contains render-time errors to a fallback UI instead of letting a
 * single component crash unmount the entire app (which shows a blank page). Wrap the
 * routed page content so the sidebar/shell survive a page-level error.
 */
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error) {
    console.error('[ErrorBoundary]', error);
  }

  handleReset = () => this.setState({ error: null });

  render() {
    if (this.state.error) {
      return (
        <div className="glass-card p-8 max-w-lg mx-auto mt-16 text-center">
          <h2 className="text-lg font-semibold text-[#f8fafc]">Something went wrong</h2>
          <p className="text-sm text-[#94a3b8] mt-2 break-words">
            {this.state.error.message}
          </p>
          <button
            onClick={this.handleReset}
            className="mt-5 px-4 py-2 rounded-lg text-sm font-medium bg-[#3b82f6] text-white hover:bg-[#2563eb] transition-colors"
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
