import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Button } from '@/ui/Button';

describe('Button', () => {
  it('shows a spinner, the loading label and aria-busy while loading', () => {
    const { rerender } = render(
      <Button loading loadingLabel="Adding…">
        Add ticker
      </Button>,
    );

    const button = screen.getByRole('button');
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(within(button).getByText('Adding…')).toBeInTheDocument();
    expect(button.querySelector('.button__spinner')).toHaveAttribute('aria-hidden', 'true');

    rerender(<Button>Add ticker</Button>);

    expect(button).toBeEnabled();
    expect(button).not.toHaveAttribute('aria-busy');
    expect(button).toHaveTextContent('Add ticker');
  });

  it('keeps the label when no loading label is supplied', () => {
    render(<Button loading>Add ticker</Button>);

    const button = screen.getByRole('button');
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent('Add ticker');
  });
});
