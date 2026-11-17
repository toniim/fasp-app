import React, { useEffect, useState } from 'react';
import { StopRecording } from '../../../bindings/github.com/heytonyne/fasp/app';
import './Recording.css';

interface RecordingBarProps {
  startedAt: number; // Unix ms
  mode: string;
}

export const formatDuration = (seconds: number) => {
  const s = Math.max(0, Math.floor(seconds));
  const h = Math.floor(s / 3600);
  const mm = String(Math.floor((s % 3600) / 60)).padStart(2, '0');
  const ss = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
};

const MODE_LABEL: Record<string, string> = { display: 'Screen', window: 'Window', region: 'Region' };

/**
 * The whole window while recording: Fasp shrinks to this always-on-top bar,
 * which is excluded from the recording itself.
 */
const RecordingBar: React.FC<RecordingBarProps> = ({ startedAt, mode }) => {
  const [now, setNow] = useState(Date.now());
  const [stopping, setStopping] = useState(false);

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(id);
  }, []);

  const stop = () => {
    setStopping(true);
    StopRecording().catch(() => setStopping(false));
  };

  return (
    <div className="recording-bar">
      <span className="recording-bar-dot" />
      <span className="recording-bar-time">{formatDuration((now - startedAt) / 1000)}</span>
      <span className="recording-bar-mode">{MODE_LABEL[mode] || mode}</span>
      <button type="button" className="recording-bar-stop" onClick={stop} disabled={stopping}>
        <span className="recording-bar-stop-icon" />
        {stopping ? 'Saving…' : 'Stop'}
      </button>
    </div>
  );
};

export default RecordingBar;
