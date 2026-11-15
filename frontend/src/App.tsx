import { useState, useEffect, useRef } from 'react';
import './App.css';
import { CaptureActiveDisplay, CaptureZone, IsRecording, OpenImageDialog, ReadImageFile, ShowWindow } from '../wailsjs/go/main/App';
import { useEditorStore } from './store/editorStore';
import EditorWindow from './components/EditorWindow/EditorWindow';
import SettingsWindow from './components/SettingsWindow/SettingsWindow';
import PermissionWarning from './components/PermissionWarning/PermissionWarning';
import Toast from './components/Toast/Toast';
import RecordDialog from './components/Recording/RecordDialog';
import RecordingBar from './components/Recording/RecordingBar';
import RecordingDone, { RecordingResult } from './components/Recording/RecordingDone';
import { EventsOn, WindowHide } from '../wailsjs/runtime/runtime';

interface ToastState {
  message: string;
  type: 'success' | 'error' | 'info';
}

// SVG Icons - Clean, macOS-style
const Icons = {
  zone: (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 7V3h4M17 3h4v4M21 17v4h-4M7 21H3v-4"/>
      <rect x="8" y="8" width="8" height="8" rx="1"/>
    </svg>
  ),
  record: (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9"/>
      <circle cx="12" cy="12" r="4" fill="currentColor"/>
    </svg>
  ),
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
  const [showRecordDialog, setShowRecordDialog] = useState(false);
  const [recording, setRecording] = useState<{ mode: string; started_at: number } | null>(null);
  const [recordingDone, setRecordingDone] = useState<RecordingResult | null>(null);
  // Event handlers are registered once, so they read the recording state
  // through a ref. While recording, the window is the control bar: captures
  // would hide it and bring it back at editor size.
  const recordingRef = useRef(false);
  useEffect(() => {
    recordingRef.current = recording !== null;
  }, [recording]);

  // Listen for hotkey events
  useEffect(() => {
    const unsubscribeCapture = EventsOn('hotkey:capture', () => {
      handleCapture();
    });

    // Region capture uses the same flow: the capture opens in the crop tool,
    // so the user drags the part of the screen they want to keep.
    const unsubscribeRegion = EventsOn('hotkey:capture_region', () => {
      handleCapture();
    });

    const unsubscribeSettings = EventsOn('open:settings', () => {
      setShowSettings(true);
      ShowWindow();
    });

    const unsubscribeOpenImage = EventsOn('open:image', () => {
      handleOpenImage();
    });

    const unsubscribeZone = EventsOn('hotkey:capture_zone', () => {
      handleCaptureZone();
    });

    const unsubscribeOpenRecord = EventsOn('open:record', () => {
      setRecordingDone(null);
      setShowRecordDialog(true);
    });

    const unsubscribeRecStarted = EventsOn('recording:started', (status) => {
      setShowRecordDialog(false);
      setRecordingDone(null);
      setRecording(status);
    });

    const unsubscribeRecFinished = EventsOn('recording:finished', (result: RecordingResult) => {
      setRecording(null);
      setRecordingDone(result);
    });

    // A reload during a recording (e.g. dev hot reload) must keep the bar.
    IsRecording().then((on) => {
      if (on) setRecording((r) => r || { mode: '', started_at: Date.now() });
    });

    return () => {
      if (unsubscribeZone) unsubscribeZone();
      if (unsubscribeOpenRecord) unsubscribeOpenRecord();
      if (unsubscribeRecStarted) unsubscribeRecStarted();
      if (unsubscribeRecFinished) unsubscribeRecFinished();
      if (unsubscribeCapture) unsubscribeCapture();
      if (unsubscribeRegion) unsubscribeRegion();
      if (unsubscribeSettings) unsubscribeSettings();
      if (unsubscribeOpenImage) unsubscribeOpenImage();
    };
  }, []);

  const handleCapture = async () => {
    if (recordingRef.current) return;
    try {
      setIsCapturing(true);

      // Hide our own window first, otherwise the app ends up in its own
      // screenshot. Give the compositor a beat to actually take it off screen.
      WindowHide();
      await new Promise((resolve) => setTimeout(resolve, 200));

      const result = await CaptureActiveDisplay();

      // Show window with the captured image
      ShowWindow();

      setImage(result.data);
    } catch (error) {
      console.error('Failed to capture screenshot:', error);

      // Show window even if capture failed
      ShowWindow();

      setToast({ message: `Failed to capture screenshot: ${error}`, type: 'error' });
    } finally {
      setIsCapturing(false);
    }
  };

  // Freeze the screen, let the user drag a zone, and open only that zone.
  const handleCaptureZone = async () => {
    if (recordingRef.current) return;
    const hadImage = !!useEditorStore.getState().image;
    try {
      setIsCapturing(true);
      WindowHide();
      await new Promise((resolve) => setTimeout(resolve, 200));

      const result = await CaptureZone();
      if (!result) {
        // Esc: go back to where the user was.
        if (hadImage) ShowWindow();
        return;
      }
      ShowWindow();
      setImage(result.data, 'select');
    } catch (error) {
      console.error('Failed to capture zone:', error);
      ShowWindow();
      setToast({ message: `Failed to capture zone: ${error}`, type: 'error' });
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
      ShowWindow();

      // Set image in editor
      setImage(imageData);
    } catch (error) {
      console.error('Failed to open image:', error);
      setToast({ message: `Failed to open image: ${error}`, type: 'error' });
    }
  };

  if (recording) {
    return <RecordingBar startedAt={recording.started_at} mode={recording.mode} />;
  }

  return (
    <div id="App">
      <PermissionWarning />

      {image ? (
        <EditorWindow />
      ) : (
        <div className="welcome-screen">
          <h1>Fasp</h1>
          <p>Screenshot & Annotation Tool</p>
          <button className="capture-button" onClick={handleCapture} disabled={isCapturing}>
            {Icons.camera}
            <span>{isCapturing ? 'Capturing...' : 'Capture Screenshot'}</span>
          </button>
          <button className="capture-button" onClick={handleCaptureZone} disabled={isCapturing}>
            {Icons.zone}
            <span>Capture Zone</span>
          </button>
          <button className="capture-button" onClick={() => setShowRecordDialog(true)}>
            {Icons.record}
            <span>Record Screen</span>
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
      {showRecordDialog && <RecordDialog onClose={() => setShowRecordDialog(false)} />}
      {recordingDone && <RecordingDone result={recordingDone} onClose={() => setRecordingDone(null)} />}

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
