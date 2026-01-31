import React from 'react'
import {createRoot} from 'react-dom/client'
import { ModuleRegistry, AllCommunityModule } from 'ag-grid-community'
import './style.css'
import '@fortawesome/fontawesome-free/css/all.min.css'
import App from './App'

// Register all AG Grid Community features
ModuleRegistry.registerModules([AllCommunityModule])

// Disable file drag-and-drop navigation in the webview. If a user drags a file onto the
// window, the default browser behavior is to navigate to that file, which would replace
// our SPA UI. We prevent this only when the drag contains files, so normal UI drags work.
const preventFileDropNavigation = (e: DragEvent) => {
  const dt = e.dataTransfer;
  const types = dt?.types ? Array.from(dt.types) : [];
  if (types.includes('Files')) {
    e.preventDefault();
    e.stopPropagation();
    if (dt) dt.dropEffect = 'none';
  }
};
(['dragover', 'dragenter', 'drop'] as const).forEach((type) => {
  window.addEventListener(type, preventFileDropNavigation);
  document.addEventListener(type, preventFileDropNavigation);
});

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
