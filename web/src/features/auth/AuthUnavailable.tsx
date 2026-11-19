import { useAuth } from '@/features/auth/useAuth';
import { AuthPanel } from '@/features/auth/AuthPanel';
import { copy } from '@/locale';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import './AuthForms.css';

/**
 * Shown when the bootstrap status request failed for a transient reason. The user is not
 * signed out; the session state is unknown until a retry succeeds.
 */
export function AuthUnavailable() {
  const { retryAuth } = useAuth();

  return (
    <AuthPanel>
      <h1 className="auth-panel__heading">{copy.brand.name}</h1>
      <div className="auth-form__error">
        <Alert>{copy.errors.generic}</Alert>
      </div>
      <div className="auth-form">
        <div className="auth-form__actions">
          <Button onClick={retryAuth}>{copy.auth.actions.tryAgain}</Button>
        </div>
      </div>
    </AuthPanel>
  );
}
