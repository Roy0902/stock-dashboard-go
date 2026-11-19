import { writeFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';

import react from '@vitejs/plugin-react';
import type { Plugin } from 'vite';
import { defineConfig as defineVitestConfig } from 'vitest/config';

// Keeps `//go:embed all:dist` compilable before the first client build and after
// `emptyOutDir` wipes the directory.
function embedPlaceholder(): Plugin {
  return {
    name: 'embed-placeholder',
    closeBundle() {
      const target = fileURLToPath(
        new URL('../server/internal/webui/dist/.gitkeep', import.meta.url),
      );
      writeFileSync(target, '');
    },
  };
}

const apiTarget = `http://127.0.0.1:${process.env.API_PORT ?? '3000'}`;

export default defineVitestConfig({
  plugins: [react(), embedPlaceholder()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      // changeOrigin stays false so the Go Origin/Host CSRF guard sees the browser's
      // real Host (the Vite port) instead of the API port.
      '/api': { target: apiTarget, changeOrigin: false },
      '/healthz': { target: apiTarget, changeOrigin: false },
    },
  },
  build: {
    outDir: '../server/internal/webui/dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: true,
    restoreMocks: true,
    clearMocks: true,
  },
});
