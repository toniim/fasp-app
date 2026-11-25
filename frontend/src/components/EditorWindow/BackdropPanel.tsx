import React, { useEffect, useRef, useState } from 'react';
import { useEditorStore } from '../../store/editorStore';
import { BACKDROP_RATIOS, BORDER_COLORS, COLOR_BACKDROPS, CUSTOM_BACKDROP_ID, BackdropPreset, paintBackdrop } from '../../utils/backdrops';
import { PHOTO_BACKDROPS } from '../../utils/backdrop-photos';
import { OpenImageDialog, ReadImageFile } from '../../../bindings/github.com/heytonyne/fasp/app';

// One preset: a photo shows its small preview, a colour preset is drawn with
// the same painter as the canvas, so the swatch looks like the result.
const Swatch: React.FC<{ preset: BackdropPreset }> = ({ preset }) => {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    if (preset.thumb) return;
    const canvas = ref.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx) return;
    const ratio = window.devicePixelRatio || 1;
    canvas.width = 56 * ratio;
    canvas.height = 36 * ratio;
    paintBackdrop(ctx, { x: 0, y: 0, width: canvas.width, height: canvas.height }, preset);
  }, [preset]);
  if (preset.thumb) return <img src={preset.thumb} alt="" className="backdrop-swatch-canvas" draggable={false} />;
  return <canvas ref={ref} className="backdrop-swatch-canvas" />;
};

interface BackdropPanelProps {
  onClose: () => void;
}

// Settings of the backdrop: on/off, the background, margin, corners and
// shadow. Opens under the toolbar's Backdrop button.
const BackdropPanel: React.FC<BackdropPanelProps> = ({ onClose }) => {
  const { backdrop, setBackdrop } = useEditorStore();
  const panelRef = useRef<HTMLDivElement>(null);
  const [choosing, setChoosing] = useState(false);
  const [chooseError, setChooseError] = useState<string | null>(null);

  const swatchButton = (p: BackdropPreset) => (
    <button
      key={p.id}
      className={`backdrop-swatch ${backdrop.presetId === p.id ? 'active' : ''}`}
      onClick={() => setBackdrop({ presetId: p.id, enabled: true })}
      title={p.name}
      aria-label={p.name}
    >
      <Swatch preset={p} />
    </button>
  );

  // The user's own picture as the background (kept for this session).
  const chooseImage = async () => {
    setChooseError(null);
    setChoosing(true);
    try {
      const path = await OpenImageDialog();
      if (!path) return;
      const data = await ReadImageFile(path);
      // The browser reads the real format from the bytes, whatever the type.
      setBackdrop({ customImage: `data:image/png;base64,${data}`, presetId: CUSTOM_BACKDROP_ID, enabled: true });
    } catch (error) {
      setChooseError(`Could not open the image: ${error}`);
    } finally {
      setChoosing(false);
    }
  };

  // A click outside the panel (except on its button, which toggles it) or Esc
  // closes it.
  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (panelRef.current?.contains(target)) return;
      if ((target as Element).closest?.('.backdrop-toggle')) return;
      onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey, true);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey, true);
    };
  }, [onClose]);

  return (
    <div className="backdrop-panel" ref={panelRef} role="dialog" aria-label="Backdrop">
      <label className="backdrop-row backdrop-enable">
        <input
          type="checkbox"
          checked={backdrop.enabled}
          onChange={(e) => setBackdrop({ enabled: e.target.checked })}
        />
        <span>Put the shot on a background</span>
      </label>
      <div className="backdrop-section">Photos</div>
      <div className="backdrop-grid">
        {PHOTO_BACKDROPS.map(swatchButton)}
      </div>
      <div className="backdrop-section">Colors</div>
      <div className="backdrop-grid">
        {COLOR_BACKDROPS.map(swatchButton)}
      </div>
      <div className="backdrop-row">
        <button className="backdrop-choose" onClick={chooseImage} disabled={choosing}>
          Choose image…
        </button>
        {backdrop.customImage && (
          <button
            className={`backdrop-swatch backdrop-custom ${backdrop.presetId === CUSTOM_BACKDROP_ID ? 'active' : ''}`}
            onClick={() => setBackdrop({ presetId: CUSTOM_BACKDROP_ID, enabled: true })}
            title="Your image"
            aria-label="Your image"
          >
            <img src={backdrop.customImage} alt="" className="backdrop-swatch-canvas" draggable={false} />
          </button>
        )}
        {chooseError && <span className="backdrop-error">{chooseError}</span>}
      </div>
      <div className="backdrop-row">
        <span className="size-label">Size</span>
        <div className="segmented" role="group" aria-label="Backdrop size">
          {BACKDROP_RATIOS.map((ratio) => (
            <button
              key={ratio}
              className={`segmented-button ${backdrop.ratio === ratio ? 'active' : ''}`}
              onClick={() => setBackdrop({ ratio, enabled: true })}
              title={ratio === 'auto' ? 'The shot plus its padding' : `Exports at ${ratio}; the shot stays centred`}
              aria-pressed={backdrop.ratio === ratio}
            >
              {ratio === 'auto' ? 'Auto' : ratio}
            </button>
          ))}
        </div>
      </div>
      <div className="backdrop-row">
        <label className="size-label" htmlFor="backdrop-padding">Padding</label>
        <input
          id="backdrop-padding"
          type="range"
          min="2"
          max="20"
          step="1"
          value={backdrop.padding}
          onChange={(e) => setBackdrop({ padding: Number(e.target.value), enabled: true })}
          className="size-slider"
        />
        <span className="size-value">{`${backdrop.padding}%`}</span>
      </div>
      <div className="backdrop-row">
        <label className="size-label" htmlFor="backdrop-radius">Corners</label>
        <input
          id="backdrop-radius"
          type="range"
          min="0"
          max="40"
          step="1"
          value={backdrop.radius}
          onChange={(e) => setBackdrop({ radius: Number(e.target.value), enabled: true })}
          className="size-slider"
        />
        <span className="size-value">{`${backdrop.radius}px`}</span>
      </div>
      <label className="backdrop-row">
        <input
          type="checkbox"
          checked={backdrop.shadow}
          onChange={(e) => setBackdrop({ shadow: e.target.checked, enabled: true })}
        />
        <span>Shadow</span>
      </label>
      <label className="backdrop-row">
        <input
          type="checkbox"
          checked={backdrop.border}
          onChange={(e) => setBackdrop({ border: e.target.checked, enabled: true })}
        />
        <span>Border</span>
      </label>
      {backdrop.border && (
        <>
          <div className="backdrop-row">
            <span className="size-label">Color</span>
            <div className="backdrop-border-colors" role="group" aria-label="Border color">
              {BORDER_COLORS.map(({ color, name }) => (
                <button
                  key={color}
                  type="button"
                  className={`backdrop-border-swatch ${backdrop.borderColor.toLowerCase() === color ? 'active' : ''}`}
                  style={{ backgroundColor: color }}
                  onClick={() => setBackdrop({ borderColor: color, enabled: true })}
                  title={name}
                  aria-label={name}
                  aria-pressed={backdrop.borderColor.toLowerCase() === color}
                />
              ))}
              <input
                type="color"
                className="backdrop-border-color-input"
                value={backdrop.borderColor}
                onChange={(e) => setBackdrop({ borderColor: e.target.value, enabled: true })}
                title="Custom border color"
                aria-label="Custom border color"
              />
            </div>
          </div>
          <div className="backdrop-row">
            <label className="size-label" htmlFor="backdrop-border-width">Width</label>
            <input
              id="backdrop-border-width"
              type="range"
              min="1"
              max="12"
              step="1"
              value={backdrop.borderWidth}
              onChange={(e) => setBackdrop({ borderWidth: Number(e.target.value), enabled: true })}
              className="size-slider"
            />
            <span className="size-value">{`${backdrop.borderWidth}px`}</span>
          </div>
        </>
      )}
    </div>
  );
};

export default BackdropPanel;
