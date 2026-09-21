import { Component, type ErrorInfo, type ReactNode } from 'react';
import { InlineNotification } from '@carbon/react';

export interface RouteErrorBoundaryProps { children: ReactNode }
interface RouteErrorBoundaryState { failed: boolean }

export class RouteErrorBoundary extends Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): RouteErrorBoundaryState {
    return { failed: true };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo): void {
    // Intentionally do not write response data or identifiers to the browser console.
  }

  render() {
    if (this.state.failed) {
      return (
        <section className="page">
          <header className="page-header"><h1>页面暂不可用</h1></header>
          <InlineNotification hideCloseButton kind="error" role="alert" title="无法显示此页面" subtitle="页面遇到兼容性错误。请重新加载；后端权威状态未被更改。" />
        </section>
      );
    }
    return this.props.children;
  }
}
