import React, { useCallback, useEffect, useRef, useState } from 'react';
import { CopyFileToClipboard, OpenPath, RevealPath, SaveTrimmed, SaveVideoAs } from '../../../bindings/github.com/heytonyne/fasp/app';
import { UploadButton } from '../EditorWindow/UploadButton';
import TrimBar from './TrimBar';
import MediaJobDialog from './MediaJobDialog';
import Toast from '../Toast/Toast';
import { formatDuration } from '../Recording/RecordingBar';
import '../EditorWindow/EditorWindow.css';
import './VideoViewer.css';

export interface VideoInfo {
  path: string;
  url: string;
  size_bytes: number;
  warnings: string[] | null;
}

interface VideoViewerProps {
  video: VideoInfo;
  onClose: () => void;
}

interface ToastState {
  message: string;
  type: 'success' | 'error' | 'info';
}

const formatSize = (bytes: number) =>
  bytes >= 1e9 ? `${(bytes / 1e9).toFixed(2)} GB` : `${(bytes / 1e6).toFixed(1)} MB`;

const Icons = {
  copy: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="5" y="5" width="9" height="9" rx="1" />
      <path d="M11 5V3C11 2.44772 10.5523 2 10 2H3C2.44772 2 2 2.44772 2 3V10C2 10.5523 2.44772 11 3 11H5" />
    </svg>
  ),
  saveAs: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 14H4C3.44772 14 3 13.5523 3 13V3C3 2.44772 3.44772 2 4 2H9L13 6V13C13 13.5523 12.5523 14 12 14Z" />
      <path d="M9 2V6H13" />
    </svg>
  ),
  trim: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 2V14" />
      <path d="M11 2V14" />
      <path d="M5 8H11" />
    </svg>
  ),
  gif: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M6 6.5H5C4.5 6.5 4.5 9.5 5 9.5H6V8.5H5.2" />
      <path d="M8 6.5V9.5" />
      <path d="M12 6.5H10V9.5M10 8H11.5" />
    </svg>
  ),
  folder: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 4.5C2 3.67 2.67 3 3.5 3H6L7.5 4.5H12.5C13.33 4.5 14 5.17 14 6V11.5C14 12.33 13.33 13 12.5 13H3.5C2.67 13 2 12.33 2 11.5V4.5Z" />
    </svg>
  ),
  external: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 3H13V7" />
      <path d="M13 3L7 9" />
      <path d="M12 10V12.5C12 13.33 11.33 14 10.5 14H3.5C2.67 14 2 13.33 2 12.5V5.5C2 4.67 2.67 4 3.5 4H6" />
    </svg>
  ),
  close: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M4 4L12 12M12 4L4 12" />
    </svg>
  ),
};

/**
 * Plays a recording inside Fasp, in the same frame as the screenshot editor:
 * toolbar on top, the video in the middle, Copy / Save As / Upload below.
 */
const VideoViewer: React.FC<VideoViewerProps> = ({ video, onClose }) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [meta, setMeta] = useState<{ w: number; h: number; seconds: number } | null>(null);
  const [failed, setFailed] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);
  // The part of the video to trim or export as a GIF, and where the playhead is.
  const [range, setRange] = useState<{ start: number; end: number } | null>(null);
  const [playhead, setPlayhead] = useState(0);
  // The job dialog that is open, if any. A trim already has its job id: the
  // save dialog came first.
  const [job, setJob] = useState<{ kind: 'gif' | 'trim'; id?: string } | null>(null);
  const name = video.path.split(/[\\/]/).pop() || video.path;
  const folder = video.path.slice(0, video.path.length - name.length - 1);

  useEffect(() => {
    setMeta(null);
    setFailed(false);
    setRange(null);
    setPlayhead(0);
    setJob(null);
  }, [video.url]);

  const seek = useCallback((seconds: number) => {
    if (videoRef.current) videoRef.current.currentTime = seconds;
  }, []);

  // Save Trimmed: the destination is asked first, then Go trims in the background.
  const saveTrimmed = useCallback(async () => {
    if (!range) return;
    try {
      const id = await SaveTrimmed(video.path, Math.round(range.start * 1000), Math.round(range.end * 1000));
      if (id) setJob({ kind: 'trim', id });
    } catch (err) {
      setToast({ message: `Trim failed: ${err}`, type: 'error' });
    }
  }, [range, video.path]);

  const copy = useCallback(async () => {
    try {
      await CopyFileToClipboard(video.path);
      setToast({ message: 'Video copied. Paste it into a chat, an email or a folder.', type: 'success' });
    } catch (err) {
      setToast({ message: `Copy failed: ${err}`, type: 'error' });
    }
  }, [video.path]);

  const saveAs = useCallback(async () => {
    try {
      const dst = await SaveVideoAs(video.path);
      if (dst) setToast({ message: `Saved to ${dst}`, type: 'success' });
    } catch (err) {
      setToast({ message: `Save failed: ${err}`, type: 'error' });
    }
  }, [video.path]);

  const getFile = useCallback(async () => {
    const res = await fetch(video.url);
    if (!res.ok) throw new Error(`Cannot read the video (HTTP ${res.status})`);
    return { blob: await res.blob(), filename: name, contentType: 'video/mp4' };
  }, [video.url, name]);

  // Keyboard: the same shortcuts as the editor where they make sense, plus
  // player keys that work without focusing the video first.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (document.querySelector('.settings-window, .job-dialog')) return; // a dialog is open
      const mod = e.ctrlKey || e.metaKey;
      const v = videoRef.current;
      if (e.key === 'Escape') {
        onClose();
      } else if (mod && e.shiftKey && e.key.toLowerCase() === 's') {
        e.preventDefault();
        saveAs();
      } else if (mod && !e.shiftKey && e.key.toLowerCase() === 'c' && !window.getSelection()?.toString()) {
        e.preventDefault();
        copy();
      } else if (v && document.activeElement !== v && (e.key === ' ' || e.key.toLowerCase() === 'k')) {
        e.preventDefault();
        if (v.paused) v.play();
        else v.pause();
      } else if (v && document.activeElement !== v && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
        e.preventDefault();
        v.currentTime = Math.max(0, v.currentTime + (e.key === 'ArrowLeft' ? -5 : 5));
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [copy, saveAs, onClose]);

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modKey = isMac ? 'Cmd' : 'Ctrl';

  return (
    <div className="editor-window video-viewer">
      <div className="toolbar">
        <div className="video-title">
          <span className="video-name" title={video.path}>{name}</span>
          <span className="video-meta">
            {meta ? `${meta.w}×${meta.h} · ${formatDuration(meta.seconds)} · ` : ''}
            {formatSize(video.size_bytes)} · saved in {folder}
          </span>
        </div>
        <button className="video-close" onClick={onClose} title="Close (Esc)">
          {Icons.close}
        </button>
      </div>

      {(video.warnings || []).length > 0 && (
        <div className="video-warnings">
          {(video.warnings || []).map((w) => (
            <span key={w}>{w}</span>
          ))}
        </div>
      )}

      <div className="editor-canvas video-stage">
        {failed ? (
          <div className="video-failed">
            <p>This video cannot be played here.</p>
            <button className="secondary-button" onClick={() => OpenPath(video.path)}>
              {Icons.external}
              <span>Open in Player</span>
            </button>
          </div>
        ) : (
          <video
            ref={videoRef}
            key={video.url}
            className="video-player"
            src={video.url}
            controls
            autoPlay
            playsInline
            onLoadedMetadata={(e) => {
              const v = e.currentTarget;
              setMeta({ w: v.videoWidth, h: v.videoHeight, seconds: v.duration });
              if (Number.isFinite(v.duration) && v.duration > 0) setRange({ start: 0, end: v.duration });
            }}
            onTimeUpdate={(e) => setPlayhead(e.currentTarget.currentTime)}
            onError={() => setFailed(true)}
          />
        )}
      </div>

      {range && meta && !failed && (
        <TrimBar
          duration={meta.seconds}
          start={range.start}
          end={range.end}
          current={playhead}
          onChange={(start, end) => setRange({ start, end })}
          onSeek={seek}
        />
      )}

      <div className="action-bar">
        <button className="primary-button" onClick={copy} title={`Copy the video file (${modKey}+C)`}>
          {Icons.copy}
          <span>Copy</span>
        </button>
        <button className="secondary-button" onClick={saveAs} title={`Save As (${modKey}+Shift+S)`}>
          {Icons.saveAs}
          <span>Save As</span>
        </button>
        <button className="secondary-button" onClick={() => RevealPath(video.path)} title="Show in Folder">
          {Icons.folder}
          <span>Show in Folder</span>
        </button>
        <button
          className="secondary-button"
          onClick={saveTrimmed}
          disabled={!range || !meta || (range.start <= 0 && range.end >= meta.seconds)}
          title="Save the part between the two handles as a new MP4 (move a handle first)"
        >
          {Icons.trim}
          <span>Save Trimmed…</span>
        </button>
        <button
          className="secondary-button"
          onClick={() => setJob({ kind: 'gif' })}
          disabled={!range || !meta}
          title="Make a GIF of the part between the two handles"
        >
          {Icons.gif}
          <span>Export GIF…</span>
        </button>
        <UploadButton
          getFile={getFile}
          hasImage
          onComplete={(_publicUrl, _directUrl, isPublic) => setToast({
            message: isPublic
              ? 'Uploaded as public. The link is on the clipboard.'
              : 'Uploaded as private. The link is on the clipboard, but only you can open it.',
            type: 'success',
          })}
          onError={(err) => setToast({ message: `Upload failed: ${err}`, type: 'error' })}
        />
      </div>

      {job && range && meta && (
        <MediaJobDialog
          kind={job.kind}
          jobId={job.id}
          video={video}
          start={range.start}
          end={range.end}
          duration={meta.seconds}
          onNotify={(message, type) => setToast({ message, type })}
          onClose={() => setJob(null)}
        />
      )}

      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
};

export default VideoViewer;
