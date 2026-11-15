import React from 'react';
import { OpenPath, RevealPath } from '../../../wailsjs/go/main/App';
import { formatDuration } from './RecordingBar';
import './Recording.css';

export interface RecordingResult {
  path: string;
  seconds: number;
  width: number;
  height: number;
  size_bytes: number;
  end_reason: string;
  warnings: string[] | null;
  error: string;
}

interface RecordingDoneProps {
  result: RecordingResult;
  onClose: () => void;
}

const formatSize = (bytes: number) =>
  bytes >= 1e9 ? `${(bytes / 1e9).toFixed(2)} GB` : `${(bytes / 1e6).toFixed(1)} MB`;

/** Shown after a recording ends: where it was saved, and quick actions. */
const RecordingDone: React.FC<RecordingDoneProps> = ({ result, onClose }) => {
  const saved = result.size_bytes > 0;
  const name = result.path.split(/[\\/]/).pop();

  return (
    <>
      <div className="settings-backdrop" onClick={onClose} />
      <div className="settings-window record-done">
        <div className="settings-header">
          <h2>{saved ? 'Recording saved' : 'Recording failed'}</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>
        <div className="settings-content">
          {saved && (
            <div className="record-done-file">
              <span className="record-done-name" title={result.path}>{name}</span>
              <span className="record-done-meta">
                {formatDuration(result.seconds)} · {result.width}×{result.height} · {formatSize(result.size_bytes)}
              </span>
            </div>
          )}
          {(result.warnings || []).map((w) => (
            <p key={w} className="record-warning">{w}</p>
          ))}
          {result.error && <p className="record-error">{result.error}</p>}
        </div>
        <div className="settings-footer">
          {saved && (
            <>
              <button className="btn-cancel" onClick={() => RevealPath(result.path)}>Show in Folder</button>
              <button className="btn-cancel" onClick={() => navigator.clipboard?.writeText(result.path)}>
                Copy Path
              </button>
              <button className="btn-save" onClick={() => OpenPath(result.path)}>Play</button>
            </>
          )}
          {!saved && <button className="btn-save" onClick={onClose}>Close</button>}
        </div>
      </div>
    </>
  );
};

export default RecordingDone;
