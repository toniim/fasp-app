import { useState, useEffect } from 'react';
import './App.css';
import { CaptureActiveDisplay, AutoScreenshot } from '../wailsjs/go/main/App';
import { useEditorStore } from './store/editorStore';
import EditorWindow from './components/EditorWindow/EditorWindow';
import SettingsWindow from './components/SettingsWindow/SettingsWindow';
import PermissionWarning from './components/PermissionWarning/PermissionWarning';
import Toast from './components/Toast/Toast';
import { EventsOn, WindowShow, WindowUnminimise, WindowHide } from '../wailsjs/runtime/runtime';

function App() {
  const { image, setImage } = useEditorStore();
  const [isCapturing, setIsCapturing] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'info' } | null>(null);

  // Listen for hotkey events
  useEffect(() => {
    const unsubscribeCapture = EventsOn('hotkey:capture', () => {
      handleCapture();
    });

    const unsubscribeAutoScreenshot = EventsOn('hotkey:auto_screenshot', () => {
      handleAutoScreenshot();
    });

    const unsubscribeSettings = EventsOn('open:settings', () => {
      setShowSettings(true);
      WindowShow();
      WindowUnminimise();
    });

    return () => {
      if (unsubscribeCapture) unsubscribeCapture();
      if (unsubscribeAutoScreenshot) unsubscribeAutoScreenshot();
      if (unsubscribeSettings) unsubscribeSettings();
    };
  }, []);

  const handleCapture = async () => {
    try {
      setIsCapturing(true);

      // Capture screenshot immediately without hiding window
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

      alert('Failed to capture screenshot: ' + error);
    } finally {
      setIsCapturing(false);
    }
  };

  const handleAutoScreenshot = async () => {
    try {
      console.log('[INFO] Auto screenshot triggered from frontend');

      // Call backend AutoScreenshot method
      const filename = await AutoScreenshot();

      console.log('[INFO] Auto screenshot saved:', filename);

      // Show success toast notification
      setToast({ message: `Saved: ${filename}`, type: 'success' });
    } catch (error) {
      console.error('[ERROR] Auto screenshot failed:', error);
      setToast({ message: `Failed to save screenshot: ${error}`, type: 'error' });
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
            {isCapturing ? 'Capturing...' : '📸 Capture Screenshot'}
          </button>
          <button className="settings-button" onClick={() => setShowSettings(true)}>
            ⚙️ Settings
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
