import React from 'react';
import { createRoot } from 'react-dom/client';
import { Toaster } from 'sonner';
import './style.css';
import App from './App';

const container = document.getElementById('root');
const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <App />
    <Toaster richColors closeButton position="top-right" />
  </React.StrictMode>
);
