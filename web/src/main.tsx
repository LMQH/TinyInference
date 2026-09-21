import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import '@carbon/charts-react/styles.css';
import './styles/index.scss';
import { App } from './app/App';
import { RealtimeProvider } from './realtime/RealtimeProvider';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000,
      gcTime: 5 * 60 * 1000,
      refetchOnWindowFocus: true,
      retry: false,
    },
    mutations: { retry: false },
  },
});

const root = document.getElementById('root');
if (!root) throw new Error('页面缺少应用挂载节点。');

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <RealtimeProvider>
          <App />
        </RealtimeProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
