import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

// Without this, any throw during render unmounts the whole tree and the user
// gets a white page with no status code and nothing to report. Styling stays on
// plain utilities because this can render before ThemeProvider has put a
// light/dark class on <html>, which would leave the theme variables undefined.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error:', error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) {
      return this.props.children;
    }

    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="w-full max-w-md space-y-4 text-center">
          <h1 className="text-lg font-medium">Something went wrong</h1>
          <p className="text-sm opacity-70">
            Reloading usually fixes this. If it keeps happening, clearing this site's cookies and
            storage will reset the session.
          </p>
          <pre className="overflow-x-auto rounded-md border p-3 text-left text-xs opacity-70">
            {error.message}
          </pre>
          <button
            type="button"
            className="rounded-md border px-4 py-2 text-sm"
            onClick={() => window.location.reload()}
          >
            Reload
          </button>
        </div>
      </div>
    );
  }
}
