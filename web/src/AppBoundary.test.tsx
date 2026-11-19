import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { AppBoundary } from '@/AppBoundary';

function Broken(): never {
  throw new Error('render failure');
}

describe('AppBoundary', () => {
  it('renders the contract copy and a reload action instead of a blank page', () => {
    const onReload = vi.fn();

    render(
      <AppBoundary onReload={onReload}>
        <Broken />
      </AppBoundary>,
    );

    expect(screen.getByText('Something went wrong. Try again shortly.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reload' }));
    expect(onReload).toHaveBeenCalledTimes(1);
  });
});
