// This module is loaded via Module Federation by csd-core
// In standalone mode, it just displays a message

import React from 'react';
import ReactDOM from 'react-dom/client';

const App: React.FC = () => (
  <div style={{ padding: '2rem', fontFamily: 'system-ui, sans-serif' }}>
    <h1>CSD DevTrack Frontend</h1>
    <p>This application is designed to run within CSD-Core via Module Federation.</p>
    <p>Please access it through the CSD-Core dashboard at <a href="http://localhost:8080">http://localhost:8080</a></p>
  </div>
);

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
