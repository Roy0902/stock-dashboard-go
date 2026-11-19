import { act, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getQuotes } from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import { useQuotes } from '@/features/quotes/useQuotes';
import { makeQuote } from '@/test/fixtures';
import { renderWithProviders } from '@/test/render';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    getQuotes: vi.fn(),
  };
});

const getQuotesMock = vi.mocked(getQuotes);

function QuotesProbe() {
  const quotes = useQuotes();
  return <span>{quotes.data?.quotes.length ?? 'pending'}</span>;
}

function setDocumentHidden(hidden: boolean): void {
  Object.defineProperty(document, 'hidden', {
    configurable: true,
    get: () => hidden,
  });
}

beforeEach(() => {
  getQuotesMock.mockReset();
  getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [makeQuote()] } });
});

afterEach(() => {
  vi.useRealTimers();
  Reflect.deleteProperty(document, 'hidden');
});

describe('useQuotes polling', () => {
  it('fetches once on mount and again after 60 seconds', async () => {
    vi.useFakeTimers();
    renderWithProviders(<QuotesProbe />);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);
    expect(screen.getByText('1')).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(59_000);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(2);
  });

  it('pauses while the document is hidden', async () => {
    vi.useFakeTimers();
    setDocumentHidden(true);
    renderWithProviders(<QuotesProbe />);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);
  });

  it('resumes the 60 second poll when the document becomes visible again', async () => {
    vi.useFakeTimers();
    setDocumentHidden(true);
    renderWithProviders(<QuotesProbe />);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      setDocumentHidden(false);
      document.dispatchEvent(new Event('visibilitychange'));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(getQuotesMock).toHaveBeenCalledTimes(2);
  });
});
