import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

function readCss(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8');
}

describe('responsive and colour regression guards', () => {
  it('collapses the seven-column row before it can overflow a 960 px viewport', () => {
    const row = readCss('../features/watchlist/QuoteRow.css');
    const list = readCss('../features/watchlist/WatchlistList.css');
    const skeleton = readCss('../features/watchlist/WatchlistSkeleton.css');

    expect(row).toContain('@media (max-width: 960px)');
    expect(list).toContain('@media (max-width: 960px)');
    expect(skeleton).toContain('@media (max-width: 960px)');
  });

  it('does not hide horizontal overflow on the body', () => {
    expect(readCss('./global.css')).not.toMatch(/overflow-x:\s*hidden/);
  });

  it('uses the direction tokens instead of literal hex values in the quote row', () => {
    const row = readCss('../features/watchlist/QuoteRow.css');

    expect(row).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(row).toContain('var(--color-up)');
    expect(row).toContain('var(--color-down)');
    expect(row).toContain('var(--color-neutral)');
  });

  it('overrides the direction tokens with AA light-mode values', () => {
    const tokens = readCss('./tokens.css');
    const lightBlock = tokens.slice(tokens.indexOf('@media (prefers-color-scheme: light)'));

    expect(lightBlock).toContain('--color-up: #15803d;');
    expect(lightBlock).toContain('--color-down: #b91c1c;');
    expect(lightBlock).toContain('--color-neutral: #4b5563;');
  });
});
