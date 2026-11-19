import { useId, useRef, useState } from 'react';
import type { FormEvent } from 'react';

import { copy } from '@/locale';
import { Button } from '@/ui/Button';
import { useAddTicker } from '@/features/watchlist/useAddTicker';
import './AddTickerForm.css';

type AddTickerFormProps = {
  onError: (message: string | null) => void;
  onAdded?: (ticker: string) => void;
};

export function AddTickerForm({ onError, onAdded }: AddTickerFormProps) {
  const [ticker, setTicker] = useState('');
  const inputId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const mutation = useAddTicker();

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mutation.isPending) {
      return;
    }
    onError(null);
    const submitted = ticker.trim();
    mutation.mutate(submitted, {
      onSuccess: () => {
        setTicker((current) => (current.trim() === submitted ? '' : current));
        onError(null);
        onAdded?.(submitted.toUpperCase());
        inputRef.current?.focus();
      },
      onError: (error) => {
        onError(error.message);
      },
    });
  };

  const handleChange = (value: string) => {
    setTicker(value);
    onError(null);
  };

  return (
    <form className="add-ticker" onSubmit={handleSubmit} aria-busy={mutation.isPending}>
      <div className="add-ticker__field">
        <label className="add-ticker__label" htmlFor={inputId}>
          {copy.addTicker.label}
        </label>
        <input
          ref={inputRef}
          id={inputId}
          className="add-ticker__input"
          name="ticker"
          type="text"
          value={ticker}
          placeholder={copy.addTicker.placeholder}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          enterKeyHint="done"
          onChange={(event) => {
            handleChange(event.target.value);
          }}
        />
      </div>
      <Button
        className="add-ticker__submit"
        type="submit"
        loading={mutation.isPending}
        loadingLabel={copy.addTicker.submitPending}
      >
        {copy.addTicker.submit}
      </Button>
    </form>
  );
}
