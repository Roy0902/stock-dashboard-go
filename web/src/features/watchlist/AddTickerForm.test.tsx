import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, addTicker } from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import type { ApiResult } from '@/api/client';
import type { Watchlist } from '@/api/types';
import { AddTickerForm } from '@/features/watchlist/AddTickerForm';
import { renderWithProviders } from '@/test/render';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    addTicker: vi.fn(),
  };
});

const addTickerMock = vi.mocked(addTicker);

function renderForm() {
  const onError = vi.fn();
  renderWithProviders(<AddTickerForm onError={onError} />);
  return {
    onError,
    input: screen.getByRole('textbox', { name: 'Ticker' }),
    button: screen.getByRole('button', { name: 'Add ticker' }),
  };
}

beforeEach(() => {
  addTickerMock.mockReset();
});

describe('AddTickerForm', () => {
  it('has a labelled input with the contracted placeholder and an add control', () => {
    renderForm();

    const input = screen.getByRole('textbox', { name: 'Ticker' });
    expect(input).toHaveAttribute('placeholder', 'e.g. AAPL');
    expect(screen.getByRole('button', { name: 'Add ticker' })).toBeEnabled();
  });

  it('disables the add control while the request is in flight', async () => {
    const user = userEvent.setup();
    let resolveAdd: (result: ApiResult<Watchlist>) => void = () => undefined;
    addTickerMock.mockImplementation(
      () =>
        new Promise<ApiResult<Watchlist>>((resolve) => {
          resolveAdd = resolve;
        }),
    );

    const { input, button } = renderForm();
    await user.type(input, 'aapl');
    await user.click(button);

    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(button).toHaveTextContent('Adding…');

    await act(async () => {
      resolveAdd({ ok: true, value: { tickers: ['AAPL'] } });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(button).toBeEnabled();
    });
    expect(button).toHaveTextContent('Add ticker');
    expect(input).toHaveValue('');
  });

  it('clears the input after a successful add', async () => {
    const user = userEvent.setup();
    addTickerMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL'] } });

    const { input, button, onError } = renderForm();
    await user.type(input, ' aapl ');
    await user.click(button);

    await waitFor(() => {
      expect(input).toHaveValue('');
    });
    expect(addTickerMock).toHaveBeenCalledWith('aapl');
    expect(onError).toHaveBeenLastCalledWith(null);
  });

  it('passes the server error copy to the shared alert slot and keeps the input', async () => {
    const user = userEvent.setup();
    addTickerMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Enter a valid ticker, e.g. AAPL.', 'INVALID_TICKER', 400),
    });

    const { input, button, onError } = renderForm();
    await user.type(input, '1AAPL');
    await user.click(button);

    await waitFor(() => {
      expect(onError).toHaveBeenLastCalledWith('Enter a valid ticker, e.g. AAPL.');
    });
    expect(input).toHaveValue('1AAPL');
  });

  it('leaves empty input validation to the API', async () => {
    const user = userEvent.setup();
    addTickerMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Enter a valid ticker, e.g. AAPL.', 'INVALID_TICKER', 400),
    });

    const { button } = renderForm();
    await user.click(button);

    expect(addTickerMock).toHaveBeenCalledWith('');
  });

  it('clears an older error when the user edits the field', async () => {
    const user = userEvent.setup();
    addTickerMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Enter a valid ticker, e.g. AAPL.', 'INVALID_TICKER', 400),
    });

    const { input, button, onError } = renderForm();
    await user.type(input, '1AAPL');
    await user.click(button);
    await waitFor(() => {
      expect(onError).toHaveBeenLastCalledWith('Enter a valid ticker, e.g. AAPL.');
    });

    await user.type(input, 'X');
    expect(onError).toHaveBeenLastCalledWith(null);
  });
});
