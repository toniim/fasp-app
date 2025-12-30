import { useState, useEffect } from 'react';
import './App.css';
import { CaptureActiveDisplay, OpenImageDialog, ReadImageFile } from '../wailsjs/go/main/App';
import { useEditorStore } from './store/editorStore';
import EditorWindow from './components/EditorWindow/EditorWindow';
import SettingsWindow from './components/SettingsWindow/SettingsWindow';
import PermissionWarning from './components/PermissionWarning/PermissionWarning';
import { EventsOn, WindowShow, WindowUnminimise, WindowHide } from '../wailsjs/runtime/runtime';

function App() {
  const { image, setImage } = useEditorStore();
  const [isCapturing, setIsCapturing] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

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
      alert('Failed to open image: ' + error);
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
          <button className="capture-button" onClick={handleOpenImage}>
            🖼️ Open Image
          </button>
          <button className="settings-button" onClick={() => setShowSettings(true)}>
            ⚙️ Settings
          </button>
        </div>
      )}

      {showSettings && <SettingsWindow onClose={() => setShowSettings(false)} />}
    </div>
  );
}

export default App;
