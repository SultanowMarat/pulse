import { Component, type ErrorInfo, type ReactNode } from 'react';

interface AppErrorBoundaryProps {
  children: ReactNode;
  title?: string;
  message?: string;
}

interface AppErrorBoundaryState {
  hasError: boolean;
}

export default class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('[AppErrorBoundary] UI block crashed:', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (!this.state.hasError) {
      return this.props.children;
    }

    const title = this.props.title || '\u041E\u0448\u0438\u0431\u043A\u0430 \u043E\u0442\u043E\u0431\u0440\u0430\u0436\u0435\u043D\u0438\u044F';
    const message = this.props.message || '\u0411\u043B\u043E\u043A \u0432\u0440\u0435\u043C\u0435\u043D\u043D\u043E \u043D\u0435\u0434\u043E\u0441\u0442\u0443\u043F\u0435\u043D. \u041E\u0431\u043D\u043E\u0432\u0438\u0442\u0435 \u0441\u0442\u0440\u0430\u043D\u0438\u0446\u0443 \u0438\u043B\u0438 \u043F\u043E\u043F\u0440\u043E\u0431\u0443\u0439\u0442\u0435 \u0435\u0449\u0435 \u0440\u0430\u0437.';

    return (
      <div className="w-full h-full min-h-[180px] flex items-center justify-center p-4">
        <div className="w-full max-w-[420px] rounded-compass border border-danger/30 bg-danger/10 p-4 text-left">
          <p className="text-[15px] font-semibold text-danger">{title}</p>
          <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5] mt-1">{message}</p>
          <button
            type="button"
            onClick={this.handleReset}
            className="mt-3 inline-flex items-center justify-center px-3 py-2 rounded-compass bg-primary text-white text-[13px] font-medium hover:bg-primary-hover transition-colors"
          >
            {'\u041F\u043E\u043F\u0440\u043E\u0431\u043E\u0432\u0430\u0442\u044C \u0441\u043D\u043E\u0432\u0430'}
          </button>
        </div>
      </div>
    );
  }
}
