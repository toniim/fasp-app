import { useState, useEffect, useRef } from 'react';
import './App.css';
import { CaptureActiveDisplay, CaptureLastZone, CaptureZone, IsRecording, OpenImageDialog, OpenRecordWindow, ReadImageFile, ShowWindow } from '../bindings/github.com/heytonyne/fasp/app';
import { useEditorStore, selectHasContent } from './store/editorStore';
import EditorWindow from './components/EditorWindow/EditorWindow';
import SettingsWindow from './components/SettingsWindow/SettingsWindow';
import PermissionWarning from './components/PermissionWarning/PermissionWarning';
import Toast from './components/Toast/Toast';
import RecordingBar from './components/Recording/RecordingBar';
import RecordingDone, { RecordingResult } from './components/Recording/RecordingDone';
import VideoViewer, { VideoInfo } from './components/VideoViewer/VideoViewer';
import { Events, Window } from '@wailsio/runtime';

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
  const setImage = useEditorStore((state) => state.setImage);
  const hasImage = useEditorStore(selectHasContent);
  const [isCapturing, setIsCapturing] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);
  const [recording, setRecording] = useState<{ mode: string; started_at: number } | null>(null);
  const [recordingDone, setRecordingDone] = useState<RecordingResult | null>(null);
  // A video shown in the in-app player; it takes the editor's place.
  const [video, setVideo] = useState<VideoInfo | null>(null);
  // Event handlers are registered once, so they read the recording state
  // through a ref. While recording, the window is the control bar: captures
  // would hide it and bring it back at editor size.
  const recordingRef = useRef(false);
  useEffect(() => {
    recordingRef.current = recording !== null;
  }, [recording]);

  // Listen for hotkey events
  useEffect(() => {
    // A hotkey (or the tray menu) captures the screen as it is, Fasp's own
    // window included, so Fasp itself can be shown in a shot.
    const unsubscribeCapture = Events.On('hotkey:capture', () => {
      handleCapture(false);
    });

    // Region capture uses the same flow: the capture opens in the crop tool,
    // so the user drags the part of the screen they want to keep.
    const unsubscribeRegion = Events.On('hotkey:capture_region', () => {
      handleCapture(false);
    });

    const unsubscribeSettings = Events.On('open:settings', () => {
      setShowSettings(true);
      ShowWindow();
    });

    const unsubscribeOpenImage = Events.On('open:image', () => {
      handleOpenImage();
    });

    const unsubscribeZone = Events.On('hotkey:capture_zone', () => {
      handleCaptureZone(false);
    });

    const unsubscribeRepeatZone = Events.On('hotkey:repeat_zone', () => {
      handleRepeatZone();
    });

    const unsubscribeRecStarted = Events.On('recording:started', ({ data: status }) => {
      setRecordingDone(null);
      setRecording(status);
    });

    const unsubscribeRecFinished = Events.On('recording:finished', ({ data: result }: { data: RecordingResult }) => {
      setRecording(null);
      if (result.url) {
        // Play it right away in the editor, like a fresh screenshot.
        setVideo({ path: result.path, url: result.url, size_bytes: result.size_bytes, warnings: result.warnings });
      } else {
        setRecordingDone(result); // nothing was saved: explain why
      }
    });

    const unsubscribeOpenVideo = Events.On('open:video', ({ data: v }: { data: VideoInfo }) => {
      setRecordingDone(null);
      setVideo(v);
    });

    // A message for work that started outside the editor (Capture Text hotkey).
    const unsubscribeToast = Events.On('toast', ({ data }: { data: ToastState }) => {
      setToast(data);
    });

    // A reload during a recording (e.g. dev hot reload) must keep the bar.
    IsRecording().then((on) => {
      if (on) setRecording((r) => r || { mode: '', started_at: Date.now() });
    });

    return () => {
      if (unsubscribeZone) unsubscribeZone();
      if (unsubscribeRepeatZone) unsubscribeRepeatZone();
      if (unsubscribeRecStarted) unsubscribeRecStarted();
      if (unsubscribeRecFinished) unsubscribeRecFinished();
      if (unsubscribeOpenVideo) unsubscribeOpenVideo();
      if (unsubscribeToast) unsubscribeToast();
      if (unsubscribeCapture) unsubscribeCapture();
      if (unsubscribeRegion) unsubscribeRegion();
      if (unsubscribeSettings) unsubscribeSettings();
      if (unsubscribeOpenImage) unsubscribeOpenImage();
    };
  }, []);

  // hideFasp: take Fasp's window off screen first. The buttons inside Fasp do,
  // or the shot would show only Fasp; a hotkey does not, so Fasp can be in it.
  const handleCapture = async (hideFasp = true) => {
    if (recordingRef.current) return;
    try {
      setIsCapturing(true);

      if (hideFasp) {
        // Give the compositor a beat to actually take it off screen.
        Window.Hide();
        await new Promise((resolve) => setTimeout(resolve, 200));
      }

      const result = await CaptureActiveDisplay();
      if (!result) throw new Error('the capture returned no image');

      // Show window with the captured image
      ShowWindow();

      setVideo(null);
      await setImage(result.data);
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
  const handleCaptureZone = async (hideFasp = true) => {
    if (recordingRef.current) return;
    // Whether the user was working in Fasp when the capture started. A hotkey
    // pressed from another app must not bring Fasp up on Esc.
    const wasInFasp = document.hasFocus() && selectHasContent(useEditorStore.getState());
    try {
      setIsCapturing(true);
      if (hideFasp) {
        Window.Hide();
        await new Promise((resolve) => setTimeout(resolve, 200));
      }

      const result = await CaptureZone();
      if (!result) {
        // Esc: leave the screen as it was. A window that stayed up is still
        // there; one hidden for the capture comes back only if the user was
        // working in it. The overlay hands the focus back to the app that
        // had it.
        if (hideFasp && wasInFasp) ShowWindow();
        return;
      }
      ShowWindow();
      setVideo(null);
      await setImage(result.data, 'select');
    } catch (error) {
      console.error('Failed to capture zone:', error);
      ShowWindow();
      setToast({ message: `Failed to capture zone: ${error}`, type: 'error' });
    } finally {
      setIsCapturing(false);
    }
  };

  // Capture the last zone again without the overlay. Like the hotkey capture it
  // leaves Fasp's window as it is. When there is no zone to repeat, or its
  // display is gone, the reason shows as a toast.
  const handleRepeatZone = async () => {
    if (recordingRef.current) return;
    try {
      setIsCapturing(true);
      const result = await CaptureLastZone();
      if (!result) throw new Error('the capture returned no image');
      ShowWindow();
      setVideo(null);
      await setImage(result.data, 'select');
    } catch (error) {
      console.error('Failed to repeat the last zone:', error);
      ShowWindow();
      setToast({ message: `Repeat Last Zone: ${error instanceof Error ? error.message : String(error)}`, type: 'error' });
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
      setVideo(null);
      await setImage(imageData);
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

      {video ? (
        <VideoViewer video={video} onClose={() => setVideo(null)} />
      ) : hasImage ? (
        <EditorWindow />
      ) : (
        <div className="welcome-screen">
          <h1>Fasp</h1>
          <p>Screenshot & Annotation Tool</p>
          <button className="capture-button" onClick={() => handleCapture(true)} disabled={isCapturing}>
            {Icons.camera}
            <span>{isCapturing ? 'Capturing...' : 'Capture Screenshot'}</span>
          </button>
          <button className="capture-button" onClick={() => handleCaptureZone(true)} disabled={isCapturing}>
            {Icons.zone}
            <span>Capture Zone</span>
          </button>
          <button className="capture-button" onClick={() => OpenRecordWindow()}>
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
