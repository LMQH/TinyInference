import { lazy, Suspense } from 'react';
import { Route, Routes, useLocation } from 'react-router-dom';
import { Loading } from '@carbon/react';
import { AppShell } from './AppShell';
import { RouteErrorBoundary } from './RouteErrorBoundary';

const OverviewRoute = lazy(() => import('../routes/overview/OverviewRoute'));
const RequestsRoute = lazy(() => import('../routes/requests/RequestsRoute'));
const MetricsRoute = lazy(() => import('../routes/metrics/MetricsRoute'));
const AlertsRoute = lazy(() => import('../routes/alerts/AlertsRoute'));
const DataRoute = lazy(() => import('../routes/data/DataRoute'));

export function App() {
  const location = useLocation();
  return (
    <AppShell>
      <RouteErrorBoundary key={location.pathname}>
      <Suspense fallback={<div className="page" aria-busy="true"><Loading withOverlay={false} description="正在加载页面" /></div>}>
        <Routes>
          <Route path="/" element={<OverviewRoute />} />
          <Route path="/requests" element={<RequestsRoute />} />
          <Route path="/metrics" element={<MetricsRoute />} />
          <Route path="/alerts" element={<AlertsRoute />} />
          <Route path="/data" element={<DataRoute />} />
          <Route path="*" element={<section className="page"><header className="page-header"><h1>页面不存在</h1><p>此控制台仅提供五个已批准的运维页面。</p></header></section>} />
        </Routes>
      </Suspense>
      </RouteErrorBoundary>
    </AppShell>
  );
}
