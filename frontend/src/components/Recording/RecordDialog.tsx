import React, { useEffect, useState } from 'react';
import {
  GetSettings,
  ListMicrophones,
  ListWindows,
  SelectRecordingRegion,
  ShowWindow,
  StartRecording,
  UpdateSetting,
} from '../../../bindings/github.com/heytonyne/fasp/app';
import type { RecordingRequest } from '../../../bindings/github.com/heytonyne/fasp/models';
import type * as recorder from '../../../bindings/github.com/heytonyne/fasp/internal/service/recorder/models';
import { Window } from '@wailsio/runtime';
import './Recording.css';

type Mode = 'display' | 'window' | 'region';

interface RecordDialogProps {
  onClose: () => void;
}

const MODES: { id: Mode; label: string; hint: string }[] = [
  { id: 'display', label: 'Full screen', hint: 'The screen under the mouse pointer' },
  { id: 'window', label: 'Window', hint: 'One window, even when it is covered' },
  { id: 'region', label: 'Region', hint: 'Drag an area after pressing Start' },
];

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/**
 * Pick what to record and which audio to include. Audio choices start from the
 * Settings defaults and are remembered for next time.
 */
const RecordDialog: React.FC<RecordDialogProps> = ({ onClose }) => {
  const [mode, setMode] = useState<Mode>('display');
  const [windows, setWindows] = useState<recorder.Window[]>([]);
  const [windowHandle, setWindowHandle] = useState<number>(0);
  const [mics, setMics] = useState<recorder.AudioDevice[]>([]);
  const [systemAudio, setSystemAudio] = useState(true);
  const [microphone, setMicrophone] = useState(false);
  const [micDevice, setMicDevice] = useState('');
  const [fps, setFps] = useState(30);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    GetSettings()
      .then((s) => {
        setSystemAudio(s?.record_system_audio ?? true);
        setMicrophone(s?.record_microphone ?? false);
        setMicDevice(s?.record_mic_device || '');
        setFps(s?.record_fps || 30);
      })
      .catch(() => undefined);
    ListMicrophones().then((m) => setMics(m || [])).catch(() => setMics([]));
    refreshWindows();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busy) onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [busy, onClose]);

  const refreshWindows = () => {
    ListWindows()
      .then((ws) => {
        setWindows(ws || []);
        if (ws?.length && !ws.some((w) => w.handle === windowHandle)) setWindowHandle(ws[0].handle);
      })
      .catch(() => setWindows([]));
  };

  const start = async () => {
    if (mode === 'window' && !windowHandle) {
      setError('Pick a window to record.');
      return;
    }
    setBusy(true);
    setError('');
    try {
      // The choices made here become the defaults for the next recording.
      await UpdateSetting('record_system_audio', systemAudio);
      await UpdateSetting('record_microphone', microphone);
      await UpdateSetting('record_mic_device', micDevice);
      await UpdateSetting('record_fps', fps);

      let region = undefined;
      if (mode === 'region') {
        Window.Hide();
        await sleep(200); // let the window leave the screen before freezing it
        const picked = await SelectRecordingRegion();
        if (!picked) {
          ShowWindow();
          setBusy(false);
          return;
        }
        region = picked;
      }

      const request: RecordingRequest = {
        mode,
        window: mode === 'window' ? windowHandle : 0,
        region: region ?? null,
        system_audio: systemAudio,
        microphone,
        mic_device: micDevice,
        fps,
      };
      await StartRecording(request);
      onClose();
    } catch (err) {
      ShowWindow();
      setError(String(err));
      setBusy(false);
    }
  };

  return (
    <>
      <div className="settings-backdrop" onClick={() => !busy && onClose()} />
      <div className="settings-window record-dialog">
        <div className="settings-header">
          <h2>Record Screen</h2>
          <button className="close-btn" onClick={onClose} disabled={busy}>×</button>
        </div>

        <div className="settings-content">
          <div className="record-modes">
            {MODES.map((m) => (
              <button
                key={m.id}
                type="button"
                className={`record-mode ${mode === m.id ? 'selected' : ''}`}
                onClick={() => setMode(m.id)}
              >
                <span className="record-mode-label">{m.label}</span>
                <span className="record-mode-hint">{m.hint}</span>
              </button>
            ))}
          </div>

          {mode === 'window' && (
            <div className="record-window-list">
              <div className="record-window-list-header">
                <span>{windows.length} windows</span>
                <button type="button" className="record-link" onClick={refreshWindows}>Refresh</button>
              </div>
              {windows.length === 0 && <p className="setting-hint">No windows to record.</p>}
              {windows.map((w) => (
                <button
                  key={w.handle}
                  type="button"
                  className={`record-window ${windowHandle === w.handle ? 'selected' : ''}`}
                  onClick={() => setWindowHandle(w.handle)}
                  title={w.title}
                >
                  <span className="record-window-title">{w.title || w.app}</span>
                  <span className="record-window-app">
                    {w.app} · {w.width}×{w.height}
                  </span>
                </button>
              ))}
            </div>
          )}

          <div className="settings-section">
            <h3>Audio</h3>
            <div className="settings-table">
              <div className="setting-row">
                <label className="setting-label">Sources</label>
                <div className="setting-input-checkbox">
                  <input
                    type="checkbox"
                    id="rec-system-audio"
                    checked={systemAudio}
                    onChange={(e) => setSystemAudio(e.target.checked)}
                  />
                  <label htmlFor="rec-system-audio" className="checkbox-label">Computer sound</label>
                  <input
                    type="checkbox"
                    id="rec-microphone"
                    checked={microphone}
                    onChange={(e) => setMicrophone(e.target.checked)}
                  />
                  <label htmlFor="rec-microphone" className="checkbox-label">Microphone</label>
                </div>
              </div>
              {microphone && (
                <div className="setting-row">
                  <label className="setting-label">Microphone</label>
                  <div className="setting-input">
                    <select value={micDevice} onChange={(e) => setMicDevice(e.target.value)}>
                      <option value="">System default</option>
                      {mics.map((m) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                    {systemAudio && (
                      <p className="setting-hint">
                        With speakers, the microphone also picks up the computer sound. Use headphones to avoid an echo.
                      </p>
                    )}
                  </div>
                </div>
              )}
              <div className="setting-row">
                <label className="setting-label">Frame Rate</label>
                <div className="setting-input">
                  <select value={fps} onChange={(e) => setFps(Number(e.target.value))}>
                    <option value={15}>15 fps</option>
                    <option value={30}>30 fps</option>
                    <option value={60}>60 fps</option>
                  </select>
                </div>
              </div>
            </div>
          </div>

          {error && <p className="record-error">{error}</p>}
        </div>

        <div className="settings-footer">
          <button className="btn-cancel" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="btn-save record-start" onClick={start} disabled={busy}>
            {busy ? 'Starting…' : mode === 'region' ? 'Select Region' : 'Start Recording'}
          </button>
        </div>
      </div>
    </>
  );
};

export default RecordDialog;
