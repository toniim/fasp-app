import React, { useRef } from 'react';

export const MIN_RANGE = 0.1; // seconds between the two handles

// m:ss.t, with tenths so a trim can be placed precisely.
export const formatTime = (seconds: number): string => {
  const total = Math.max(0, seconds);
  const m = Math.floor(total / 60);
  const s = total - m * 60;
  return `${m}:${s.toFixed(1).padStart(4, '0')}`;
};

interface TrimBarProps {
  duration: number;
  start: number;
  end: number;
  current: number; // playhead, seconds
  onChange: (start: number, end: number) => void;
  onSeek: (seconds: number) => void;
}

type Handle = 'start' | 'end';

/**
 * The trim range under the video: two handles on a timeline, the playhead, the
 * chosen times, and Set Start / Set End to take the current time of the video.
 */
const TrimBar: React.FC<TrimBarProps> = ({ duration, start, end, current, onChange, onSeek }) => {
  const trackRef = useRef<HTMLDivElement>(null);
  const pct = (t: number) => `${(Math.min(Math.max(t, 0), duration) / duration) * 100}%`;

  const timeAt = (clientX: number): number => {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect || rect.width === 0) return 0;
    return Math.min(Math.max((clientX - rect.left) / rect.width, 0), 1) * duration;
  };

  const move = (handle: Handle, t: number) => {
    if (handle === 'start') {
      const next = Math.min(Math.max(t, 0), end - MIN_RANGE);
      onChange(next, end);
      onSeek(next);
    } else {
      const next = Math.max(Math.min(t, duration), start + MIN_RANGE);
      onChange(start, next);
      onSeek(next);
    }
  };

  const onHandleDown = (handle: Handle) => (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const onHandleMove = (handle: Handle) => (e: React.PointerEvent<HTMLDivElement>) => {
    if (!e.currentTarget.hasPointerCapture(e.pointerId)) return;
    move(handle, timeAt(e.clientX));
  };

  const onHandleKey = (handle: Handle) => (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    e.stopPropagation();
    const step = (e.shiftKey ? 1 : 0.1) * (e.key === 'ArrowLeft' ? -1 : 1);
    move(handle, (handle === 'start' ? start : end) + step);
  };

  const setStart = () => onChange(Math.min(current, end - MIN_RANGE), end);
  const setEnd = () => onChange(start, Math.max(current, start + MIN_RANGE));
  const isFull = start <= 0 && end >= duration;

  return (
    <div className="trim-bar">
      <div className="trim-row">
        <div
          className="trim-track"
          ref={trackRef}
          onClick={(e) => onSeek(timeAt(e.clientX))}
          title="Click to jump to that time"
        >
          <div className="trim-range" style={{ left: pct(start), width: `calc(${pct(end)} - ${pct(start)})` }} />
          <div className="trim-playhead" style={{ left: pct(current) }} />
          {(['start', 'end'] as const).map((handle) => (
            <div
              key={handle}
              className={`trim-handle trim-handle-${handle}`}
              style={{ left: pct(handle === 'start' ? start : end) }}
              role="slider"
              tabIndex={0}
              aria-label={handle === 'start' ? 'Trim start' : 'Trim end'}
              aria-valuemin={0}
              aria-valuemax={duration}
              aria-valuenow={handle === 'start' ? start : end}
              aria-valuetext={formatTime(handle === 'start' ? start : end)}
              onClick={(e) => e.stopPropagation()}
              onPointerDown={onHandleDown(handle)}
              onPointerMove={onHandleMove(handle)}
              onKeyDown={onHandleKey(handle)}
            />
          ))}
        </div>
      </div>
      <div className="trim-row trim-controls">
        <button className="secondary-button trim-set" onClick={setStart} title="Start the trim at the current time">
          Set Start
        </button>
        <span className="trim-times">
          <span>Start {formatTime(start)}</span>
          <span>End {formatTime(end)}</span>
          <span>Length {formatTime(end - start)}</span>
        </span>
        <button className="secondary-button trim-set" onClick={setEnd} title="End the trim at the current time">
          Set End
        </button>
        <button
          className="secondary-button trim-set"
          onClick={() => onChange(0, duration)}
          disabled={isFull}
          title="Use the whole video"
        >
          Reset
        </button>
      </div>
    </div>
  );
};

export default TrimBar;
