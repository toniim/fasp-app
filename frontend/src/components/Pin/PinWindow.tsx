import React, { useCallback, useEffect, useRef, useState } from 'react';
import { ClosePin, CopyPin, GetPinImage, ResetPinZoom, SavePinAs, ZoomPin } from '../../../bindings/github.com/heytonyne/fasp/app';
import './PinWindow.css';

interface PinWindowProps {
  id: string;
}

const OPACITIES = [1, 0.75, 0.5, 0.25];

const Icons = {
  copy: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="5" y="5" width="9" height="9" rx="1" />
      <path d="M11 5V3C11 2.44772 10.5523 2 10 2H3C2.44772 2 2 2.44772 2 3V10C2 10.5523 2.44772 11 3 11H5" />
    </svg>
  ),
  save: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 14H4C3.44772 14 3 13.5523 3 13V3C3 2.44772 3.44772 2 4 2H9L13 6V13C13 13.5523 12.5523 14 12 14Z" />
      <path d="M9 2V6H13" />
    </svg>
  ),
  close: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M4 4L12 12M12 4L4 12" />
    </svg>
  ),
};

// A pinned screenshot. Go owns the window (size and position); this page draws
// the image, turns wheel and pinch input into zoom requests, and offers a small
// toolbar. The window is borderless, so dragging the picture moves it.
const PinWindow: React.FC<PinWindowProps> = ({ id }) => {
  const [src, setSrc] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [opacity, setOpacity] = useState(1);
  const [note, setNote] = useState('');
  const frameRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    document.documentElement.classList.add('pin-page');
    GetPinImage(id)
      .then((data) => (data ? setSrc(`data:image/png;base64,${data}`) : setFailed(true)))
      .catch(() => setFailed(true));
  }, [id]);

  // Show a short result next to the toolbar buttons, then clear it.
  const noteTimer = useRef<number>();
  const flash = useCallback((text: string) => {
    setNote(text);
    window.clearTimeout(noteTimer.current);
    noteTimer.current = window.setTimeout(() => setNote(''), 2000);
  }, []);

  const close = useCallback(() => {
    ClosePin(id).catch((err) => console.error('Failed to close the pin:', err));
  }, [id]);

  const copy = async () => {
    try {
      await CopyPin(id);
      flash('Copied');
    } catch (err) {
      flash(`Copy failed: ${err}`);
    }
  };

  const save = async () => {
    try {
      const path = await SavePinAs(id);
      if (path) flash('Saved');
    } catch (err) {
      flash(`Save failed: ${err}`);
    }
  };

  const cycleOpacity = () => {
    const next = OPACITIES[(OPACITIES.indexOf(opacity) + 1) % OPACITIES.length];
    setOpacity(next);
  };

  // Zoom requests are sent one at a time: wheel events arrive faster than the
  // window can resize, so those that come in meanwhile are merged into one.
  const pending = useRef({ factor: 1, x: 0, y: 0 });
  const busy = useRef(false);
  const requestZoom = useCallback(
    async (factor: number, x: number, y: number) => {
      pending.current = { factor: pending.current.factor * factor, x, y };
      if (busy.current) return;
      busy.current = true;
      try {
        while (pending.current.factor !== 1) {
          const { factor: f, x: px, y: py } = pending.current;
          pending.current = { factor: 1, x: px, y: py };
          await ZoomPin(id, f, px, py);
        }
      } catch (err) {
        console.error('Failed to zoom the pin:', err);
        pending.current = { factor: 1, x: 0, y: 0 };
      } finally {
        busy.current = false;
      }
    },
    [id],
  );

  useEffect(() => {
    // Mouse wheel and a trackpad pinch on Windows (which arrives as Ctrl+wheel).
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const lines = e.deltaMode === 1 ? 16 : 1;
      const speed = e.ctrlKey ? 0.01 : 0.0015;
      requestZoom(Math.exp(-e.deltaY * lines * speed), e.clientX, e.clientY);
    };
    // A trackpad pinch in the macOS web view arrives as WebKit gesture events.
    let lastScale = 1;
    const onGestureStart = (e: Event) => {
      e.preventDefault();
      lastScale = 1;
    };
    const onGestureChange = (e: Event) => {
      e.preventDefault();
      const g = e as unknown as { scale: number; clientX: number; clientY: number };
      requestZoom(g.scale / lastScale, g.clientX, g.clientY);
      lastScale = g.scale;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    window.addEventListener('wheel', onWheel, { passive: false });
    window.addEventListener('gesturestart', onGestureStart);
    window.addEventListener('gesturechange', onGestureChange);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('wheel', onWheel);
      window.removeEventListener('gesturestart', onGestureStart);
      window.removeEventListener('gesturechange', onGestureChange);
      window.removeEventListener('keydown', onKey);
    };
  }, [requestZoom, close]);

  // Double-click resets to 100%. The runtime treats a double-click on a drag
  // area as a title bar double-click on macOS (it would zoom the window), so
  // the area stops being a drag area until that click has passed.
  const onMouseDown = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.detail !== 2 || e.button !== 0) return;
    const frame = e.currentTarget;
    frame.style.setProperty('--wails-draggable', 'no-drag');
    window.setTimeout(() => frame.style.removeProperty('--wails-draggable'), 500);
    ResetPinZoom(id, e.clientX, e.clientY).catch((err) => console.error('Failed to reset the zoom:', err));
  };

  if (failed) {
    return (
      <div className="pin-frame pin-failed">
        <span>This pin is no longer available.</span>
        <button className="pin-button" onClick={close} title="Close (Esc)">
          {Icons.close}
        </button>
      </div>
    );
  }

  return (
    <div
      ref={frameRef}
      className="pin-frame"
      onMouseDown={onMouseDown}
      onContextMenu={(e) => e.preventDefault()}
    >
      {src && <img className="pin-image" src={src} style={{ opacity }} alt="Pinned screenshot" draggable={false} />}
      <div className="pin-toolbar" role="toolbar">
        <button className="pin-button pin-opacity" onClick={cycleOpacity} title="Opacity: click to change (100, 75, 50, 25 %)">
          {Math.round(opacity * 100)}%
        </button>
        <button className="pin-button" onClick={copy} title="Copy the image">
          {Icons.copy}
        </button>
        <button className="pin-button" onClick={save} title="Save As…">
          {Icons.save}
        </button>
        <button className="pin-button" onClick={close} title="Close (Esc)">
          {Icons.close}
        </button>
        {note && <span className="pin-note">{note}</span>}
      </div>
    </div>
  );
};

export default PinWindow;
