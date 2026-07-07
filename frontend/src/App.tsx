import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Sidebar from './components/Layout/Sidebar';
import ErrorBoundary from './components/common/ErrorBoundary';
import Dashboard from './pages/Dashboard';
import PullRequests from './pages/PullRequests';
import PullRequestDetail from './pages/PullRequestDetail';
import Repositories from './pages/Repositories';
import Reviews from './pages/Reviews';

// Lazy-loaded so the heavy mermaid dependency only loads when the Pipeline page is opened.
const Pipeline = lazy(() => import('./pages/Pipeline'));

export default function App() {
  return (
    <BrowserRouter>
      <div className="flex min-h-screen bg-[#0f172a]">
        <Sidebar />
        <main className="flex-1 ml-[260px] p-8 overflow-auto">
          <ErrorBoundary>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/repositories" element={<Repositories />} />
              <Route path="/pull-requests" element={<PullRequests />} />
              <Route path="/pull-requests/:id" element={<PullRequestDetail />} />
              <Route path="/reviews" element={<Reviews />} />
              <Route
                path="/pipeline"
                element={
                  <Suspense
                    fallback={<div className="text-sm text-[#94a3b8]">Loading pipeline…</div>}
                  >
                    <Pipeline />
                  </Suspense>
                }
              />
            </Routes>
          </ErrorBoundary>
        </main>
      </div>
    </BrowserRouter>
  );
}
