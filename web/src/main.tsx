import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';

import { App } from '@/App';
import { isRetryableQueryError } from '@/api/queryKeys';
import { copy } from '@/locale';
import '@/styles/tokens.css';
import '@/styles/global.css';

const routerBasename = import.meta.env.BASE_URL.replace(/\/$/, '') || '/';

document.title = copy.document.title;
document.documentElement.lang = copy.localeTag;
document
  .querySelector('meta[name="description"]')
  ?.setAttribute('content', copy.document.description);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Retry transient failures (network, 5xx) with exponential backoff; never a 4xx.
      retry: (failureCount, error) => isRetryableQueryError(failureCount, error),
      retryDelay: (attemptIndex) => Math.min(1_000 * 2 ** attemptIndex, 8_000),
      refetchOnReconnect: true,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
});

const rootElement = document.getElementById('root');
if (rootElement === null) {
  throw new Error('Root element not found');
}

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename={routerBasename}>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
