import { useId, useState } from 'react';

import { copy } from '@/locale';
import { Button } from '@/ui/Button';
import './AuthForms.css';

type RecoveryCodeListProps = {
  codes: string[];
};

export function RecoveryCodeList({ codes }: RecoveryCodeListProps) {
  const listId = useId();
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  const handleCopy = (code: string) => {
    if (typeof navigator.clipboard?.writeText !== 'function') return;
    void navigator.clipboard
      .writeText(code)
      .then(() => {
        setCopiedCode(code);
      })
      .catch(() => {
        setCopiedCode(null);
      });
  };

  return (
    <>
      <ul className="auth-recovery" aria-label={copy.auth.setup.recoveryCodesLabel}>
        {codes.map((code, index) => {
          const codeId = `${listId}-${index}`;
          return (
            <li className="auth-recovery__item" key={code}>
              <code className="auth-recovery__code" id={codeId}>
                {code}
              </code>
              <Button
                className="auth-recovery__copy"
                variant="subtle"
                aria-describedby={codeId}
                onClick={() => {
                  handleCopy(code);
                }}
              >
                {copiedCode === code ? copy.auth.actions.copied : copy.auth.actions.copy}
              </Button>
            </li>
          );
        })}
      </ul>
      <p className="sr-only" aria-live="polite">
        {copiedCode === null ? '' : copy.auth.actions.copied}
      </p>
    </>
  );
}
