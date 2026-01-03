/**
 * Routes exposed to csd-core via Module Federation.
 *
 * IMPORTANT: This file should NOT wrap routes in Router or Layout components.
 * csd-core provides the Router and Layout - we just provide the route mapping.
 */
import React, { Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ServiceConfigProvider } from './ServiceConfigContext';
import { TranslationProvider } from './translations/TranslationContext';
import type { ServiceConfig } from './types';

// Lazy load pages for better performance
const DashboardPage = React.lazy(() => import('./modules/devtrack/dashboard/DashboardPage'));
const ProjectsPage = React.lazy(() => import('./modules/devtrack/projects/ProjectsPage'));
const ProjectDetailPage = React.lazy(() => import('./modules/devtrack/projects/ProjectDetailPage'));
const SessionsPage = React.lazy(() => import('./modules/devtrack/sessions/SessionsPage'));
const SessionDetailPage = React.lazy(() => import('./modules/devtrack/sessions/SessionDetailPage'));

// Loading component
const Loading: React.FC = () => (
  <div style={{ padding: '2rem', textAlign: 'center' }}>Loading...</div>
);

// Route configuration
const ROUTES: Record<string, React.ComponentType> = {
  '/devtrack/dashboard': DashboardPage,
  '/devtrack/projects': ProjectsPage,
  '/devtrack/projects/:id': ProjectDetailPage,
  '/devtrack/sessions': SessionsPage,
  '/devtrack/sessions/:id': SessionDetailPage,
};

interface DevTrackRoutesProps {
  config?: ServiceConfig;
}

/**
 * Main routes component exposed via Module Federation.
 * Receives ServiceConfig from csd-core host.
 */
const DevTrackRoutes: React.FC<DevTrackRoutesProps> = ({ config }) => {
  return (
    <ServiceConfigProvider config={config}>
      <TranslationProvider>
        <Suspense fallback={<Loading />}>
          <Routes>
            {/* Dashboard */}
            <Route path="/devtrack/dashboard" element={<DashboardPage />} />

            {/* Projects */}
            <Route path="/devtrack/projects" element={<ProjectsPage />} />
            <Route path="/devtrack/projects/:id" element={<ProjectDetailPage />} />

            {/* Sessions */}
            <Route path="/devtrack/sessions" element={<SessionsPage />} />
            <Route path="/devtrack/sessions/:id" element={<SessionDetailPage />} />

            {/* Default redirect */}
            <Route path="/devtrack" element={<Navigate to="/devtrack/dashboard" replace />} />
            <Route path="/devtrack/*" element={<Navigate to="/devtrack/dashboard" replace />} />
          </Routes>
        </Suspense>
      </TranslationProvider>
    </ServiceConfigProvider>
  );
};

export default DevTrackRoutes;
