import { useState, useEffect } from 'react';
import './App.css';
import { CaptureActiveDisplay, OpenImageDialog, ReadImageFile } from '../wailsjs/go/main/App';
import { useEditorStore } from './store/editorStore';
import EditorWindow from './components/EditorWindow/EditorWindow';
import SettingsWindow from './components/SettingsWindow/SettingsWindow';
import PermissionWarning from './components/PermissionWarning/PermissionWarning';
import Toast from './components/Toast/Toast';
import { EventsOn, WindowShow, WindowUnminimise, WindowHide } from '../wailsjs/runtime/runtime';

interface ToastState {
  message: string;
  type: 'success' | 'error' | 'info';
}

// SVG Icons - Clean, macOS-style
const Icons = {
  camera: (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
      <circle cx="12" cy="13" r="4"/>
    </svg>
  ),
  image: (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
      <circle cx="8.5" cy="8.5" r="1.5"/>
      <polyline points="21 15 16 10 5 21"/>
    </svg>
  ),
  settings: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3"/>
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>
  ),
};

function App() {
  const { image, setImage } = useEditorStore();
  const [isCapturing, setIsCapturing] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  // Listen for hotkey events
  useEffect(() => {
    const unsubscribeCapture = EventsOn('hotkey:capture', () => {
      handleCapture();
    });

    const unsubscribeSettings = EventsOn('open:settings', () => {
      setShowSettings(true);
      WindowShow();
      WindowUnminimise();
    });

    const unsubscribeOpenImage = EventsOn('open:image', () => {
      handleOpenImage();
    });

    return () => {
      if (unsubscribeCapture) unsubscribeCapture();
      if (unsubscribeSettings) unsubscribeSettings();
      if (unsubscribeOpenImage) unsubscribeOpenImage();
    };
  }, []);

  const handleCapture = async () => {
    try {
      setIsCapturing(true);

      // Hide the Grabix window first so it doesn't appear in the screenshot
      WindowHide();
      // Give the OS a tick to actually hide the window before capturing
      await new Promise((resolve) => setTimeout(resolve, 200));

      const result = await CaptureActiveDisplay();

      // Show window with the captured image
      WindowUnminimise();
      WindowShow();

      setImage(result.data);
    } catch (error) {
      console.error('Failed to capture screenshot:', error);

      // Show window even if capture failed
      WindowUnminimise();
      WindowShow();

      setToast({ message: `Failed to capture screenshot: ${error}`, type: 'error' });
    } finally {
      setIsCapturing(false);
    }
  };

  const handleOpenImage = async () => {
    try {
      // Open file dialog
      const path = await OpenImageDialog();

      if (!path) {
        // User cancelled
        return;
      }

      // Read image file
      const imageData = await ReadImageFile(path);

      // Show window with the image
      WindowUnminimise();
      WindowShow();

      // Set image in editor
      setImage(imageData);
    } catch (error) {
      console.error('Failed to open image:', error);
      setToast({ message: `Failed to open image: ${error}`, type: 'error' });
    }
  };

  return (
    <div id="App">
      <PermissionWarning />

      {image ? (
        <EditorWindow />
      ) : (
        <div className="welcome-screen">
          <h1>Grabix</h1>
          <p>Screenshot & Annotation Tool</p>
          <button className="capture-button" onClick={handleCapture} disabled={isCapturing}>
            {Icons.camera}
            <span>{isCapturing ? 'Capturing...' : 'Capture Screenshot'}</span>
          </button>
          <button className="capture-button" onClick={handleOpenImage}>
            {Icons.image}
            <span>Open Image</span>
          </button>
          <button className="settings-button" onClick={() => setShowSettings(true)}>
            {Icons.settings}
            <span>Settings</span>
          </button>
        </div>
      )}

      {showSettings && <SettingsWindow onClose={() => setShowSettings(false)} />}

      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
}

export default App;
