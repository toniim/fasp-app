import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Events } from '@wailsio/runtime';
import {
  CancelMediaJob,
  CopyFileToClipboard,
  EstimateGIF,
  GetMediaJob,
  QuickSaveGIF,
  ReleaseMediaJob,
  SaveGIFAs,
  StartGIFExport,
} from '../../../bindings/github.com/heytonyne/fasp/app';
import { formatTime } from './TrimBar';

interface VideoRef {
  path: string;
}

// The payload of the "media:job" event (see MediaJobEvent in app_mediajobs.go).
interface MediaJobEvent {
  id: string;
  kind: 'trim' | 'gif';
  state: 'running' | 'done' | 'error' | 'cancelled';
  done: number;
  total: number;
  path: string;
  size_bytes: number;
  error: string;
}

export const GIF_FPS = [8, 10, 12, 15];
export const GIF_WIDTHS: { value: number; label: string }[] = [
  { value: 0, label: 'Original (up to 960 px)' },
  { value: 640, label: '640 px' },
  { value: 480, label: '480 px' },
];

export const formatBytes = (bytes: number): string =>
  bytes >= 1e6 ? `${(bytes / 1e6).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1e3))} KB`;

interface MediaJobDialogProps {
  kind: 'gif' | 'trim';
  video: VideoRef;
  start: number; // seconds
  end: number; // seconds
  duration: number; // seconds
  // A job that is already running (a trim starts after its save dialog).
  jobId?: string;
  onNotify: (message: string, type: 'success' | 'error' | 'info') => void;
  onClose: () => void;
}

type Phase = 'options' | 'running' | 'done' | 'error';

const errText = (err: unknown): string => (err instanceof Error ? err.message : String(err ?? 'unknown error'));

/**
 * One dialog for the two long jobs of the player. A GIF export asks for the
 * frame rate and width first and shows the predicted size; both jobs then show
 * their progress, run in Go off the UI thread, and can be cancelled.
 */
const MediaJobDialog: React.FC<MediaJobDialogProps> = ({ kind, video, start, end, duration, jobId, onNotify, onClose }) => {
  const [phase, setPhase] = useState<Phase>(jobId ? 'running' : 'options');
  const [fps, setFps] = useState(10);
  const [width, setWidth] = useState(0);
  const [estimate, setEstimate] = useState<{ bytes?: number; error?: string } | null>(null);
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const [result, setResult] = useState<{ path: string; size: number } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false); // a save / copy of the result is in progress

  const phaseRef = useRef<Phase>(phase);
  const idRef = useRef<string | null>(jobId ?? null);
  const setPhaseBoth = (p: Phase) => {
    phaseRef.current = p;
    setPhase(p);
  };

  // The whole video is sent as "no end" so the range never overshoots by rounding.
  const startMs = Math.round(start * 1000);
  const endMs = end >= duration - 0.01 ? 0 : Math.round(end * 1000);

  // Apply one job event to the dialog.
  const apply = (data: MediaJobEvent) => {
    if (data.kind !== kind || phaseRef.current !== 'running') return;
    if (idRef.current === null) idRef.current = data.id;
    if (data.id !== idRef.current) return;
    switch (data.state) {
      case 'running':
        setProgress({ done: data.done, total: data.total });
        break;
      case 'done':
        if (kind === 'trim') {
          onNotify(`Saved to ${data.path}`, 'success');
          onClose();
        } else {
          setResult({ path: data.path, size: data.size_bytes });
          setPhaseBoth('done');
        }
        break;
      case 'error':
        setError(data.error || 'The job failed.');
        setPhaseBoth('error');
        break;
      case 'cancelled':
        if (kind === 'trim') {
          onNotify('Trim cancelled', 'info');
          onClose();
        } else {
          idRef.current = null;
          setProgress({ done: 0, total: 0 });
          setPhaseBoth('options');
        }
        break;
    }
  };

  useEffect(() => {
    const off = Events.On('media:job', ({ data }: { data: MediaJobEvent }) => apply(data));
    // A short job can end before this dialog was listening: ask how it stands.
    if (jobId) {
      GetMediaJob(jobId)
        .then((e) => e && apply(e as unknown as MediaJobEvent))
        .catch(() => undefined);
    }
    return () => {
      if (off) off();
    };
    // apply only reads refs and the parent's callbacks; it is set up once per job.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, jobId]);

  // Predict the GIF size for the current choices; late answers are dropped.
  useEffect(() => {
    if (kind !== 'gif' || phase !== 'options') return;
    let stale = false;
    setEstimate(null);
    const timer = window.setTimeout(() => {
      EstimateGIF(video.path, startMs, endMs, fps, width)
        .then((bytes) => !stale && setEstimate({ bytes }))
        .catch((err) => !stale && setEstimate({ error: errText(err) }));
    }, 250);
    return () => {
      stale = true;
      window.clearTimeout(timer);
    };
  }, [kind, phase, video.path, startMs, endMs, fps, width]);

  const cancel = useCallback(() => {
    if (idRef.current) CancelMediaJob(idRef.current);
  }, []);

  // Closing releases a finished GIF's temporary file; a running job is cancelled.
  const close = useCallback(() => {
    if (phaseRef.current === 'running') {
      cancel();
      if (kind === 'gif') return; // the "cancelled" event brings the options back
    }
    if (kind === 'gif' && idRef.current) ReleaseMediaJob(idRef.current);
    onClose();
  }, [cancel, kind, onClose]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      e.stopPropagation();
      if (phaseRef.current === 'running') cancel();
      else close();
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [cancel, close]);

  const startExport = async () => {
    setError('');
    setProgress({ done: 0, total: 0 });
    idRef.current = null;
    setPhaseBoth('running');
    try {
      const id = await StartGIFExport(video.path, startMs, endMs, fps, width);
      idRef.current = id;
    } catch (err) {
      setError(errText(err));
      setPhaseBoth('error');
    }
  };

  const act = async (work: () => Promise<string | void>, doneText: (r: string) => string) => {
    if (!result) return;
    setBusy(true);
    try {
      const r = await work();
      if (r !== '') onNotify(doneText(String(r ?? '')), 'success');
    } catch (err) {
      onNotify(errText(err), 'error');
    } finally {
      setBusy(false);
    }
  };

  const pct = progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0;
  const title = kind === 'gif' ? 'Export GIF' : 'Save Trimmed Video';

  return (
    <>
      <div className="job-backdrop" onClick={phase === 'running' ? undefined : close} />
      <div className="job-dialog" role="dialog" aria-label={title}>
        <h3>{title}</h3>
        <p className="job-range">
          {formatTime(start)} to {formatTime(end)} ({formatTime(end - start)})
        </p>

        {phase === 'options' && (
          <>
            <div className="job-row">
              <label htmlFor="gif-fps">Frame rate</label>
              <select id="gif-fps" value={fps} onChange={(e) => setFps(Number(e.target.value))}>
                {GIF_FPS.map((f) => (
                  <option key={f} value={f}>{f} fps</option>
                ))}
              </select>
            </div>
            <div className="job-row">
              <label htmlFor="gif-width">Width</label>
              <select id="gif-width" value={width} onChange={(e) => setWidth(Number(e.target.value))}>
                {GIF_WIDTHS.map((w) => (
                  <option key={w.value} value={w.value}>{w.label}</option>
                ))}
              </select>
            </div>
            <p className="job-estimate" data-testid="gif-estimate">
              {estimate === null && 'Estimating the size…'}
              {estimate?.bytes !== undefined && `Estimated size: about ${formatBytes(estimate.bytes)}`}
              {estimate?.error !== undefined && `Cannot estimate the size: ${estimate.error}`}
            </p>
            <div className="job-actions">
              <button className="secondary-button" onClick={close}>Cancel</button>
              <button className="primary-button" onClick={startExport} disabled={estimate?.error !== undefined}>
                Export
              </button>
            </div>
          </>
        )}

        {phase === 'running' && (
          <>
            <div
              className={`job-progress${progress.total === 0 ? ' indeterminate' : ''}`}
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={pct}
            >
              <div className="job-progress-bar" style={{ width: progress.total === 0 ? undefined : `${pct}%` }} />
            </div>
            <p className="job-estimate">
              {kind === 'gif'
                ? progress.total > 0
                  ? `Exporting… ${pct}% (frame ${progress.done} of ${progress.total})`
                  : 'Starting…'
                : 'Trimming the video…'}
            </p>
            <div className="job-actions">
              <button className="secondary-button" onClick={cancel}>Cancel</button>
            </div>
          </>
        )}

        {phase === 'done' && result && (
          <>
            <p className="job-estimate">GIF ready: {formatBytes(result.size)}</p>
            <div className="job-actions job-actions-wrap">
              <button
                className="primary-button"
                disabled={busy}
                onClick={() => act(() => SaveGIFAs(idRef.current!, result.path), (p) => `Saved to ${p}`)}
              >
                Save As…
              </button>
              <button
                className="secondary-button"
                disabled={busy}
                onClick={() => act(() => QuickSaveGIF(idRef.current!, result.path), (p) => `Saved to ${p}`)}
              >
                Quick Save
              </button>
              <button
                className="secondary-button"
                disabled={busy}
                onClick={() =>
                  act(async () => {
                    await CopyFileToClipboard(result.path);
                    return 'copied';
                  }, () => 'GIF copied. Paste it into a chat, an email or a folder.')
                }
              >
                Copy
              </button>
              <button className="secondary-button" onClick={close}>Done</button>
            </div>
          </>
        )}

        {phase === 'error' && (
          <>
            <p className="job-error">{error}</p>
            <div className="job-actions">
              <button className="secondary-button" onClick={close}>Close</button>
              {kind === 'gif' && (
                <button className="primary-button" onClick={() => setPhaseBoth('options')}>Back</button>
              )}
            </div>
          </>
        )}
      </div>
    </>
  );
};

export default MediaJobDialog;
