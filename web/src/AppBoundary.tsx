import { Component } from 'react';
import type { ReactNode } from 'react';

import { copy } from '@/locale';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import './AppBoundary.css';

type AppBoundaryProps = {
  children: ReactNode;
  /** Test seam; production reloads the document. */
  onReload?: () => void;
};

type AppBoundaryState = {
  failed: boolean;
};

/**
 * Turns a render exception into a recoverable panel instead of a blank `#root`.
 * The copy is the contract's INTERNAL_ERROR string.
 */
export class AppBoundary extends Component<AppBoundaryProps, AppBoundaryState> {
  state: AppBoundaryState = { failed: false };

  static getDerivedStateFromError(): AppBoundaryState {
    return { failed: true };
  }

  render() {
    if (this.state.failed) {
      return (
        <div className="app-boundary">
          <Alert>{copy.errors.generic}</Alert>
          <Button
            onClick={() => {
              (this.props.onReload ?? (() => window.location.reload()))();
            }}
          >
            {copy.app.reload}
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
