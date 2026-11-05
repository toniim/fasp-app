import React from 'react';
import './ZoomBar.css';

interface ZoomBarProps {
  zoom: number;
  onZoomChange: (zoom: number) => void;
  // Base scale applied to fit the image into the viewport. One image pixel
  // equals one physical screen pixel when zoom = 1 / (scaleRatio * dpr).
  scaleRatio: number;
}

const ZoomBar: React.FC<ZoomBarProps> = ({ zoom, onZoomChange, scaleRatio }) => {
  // Screens are often scaled (e.g. Windows at 110%): a CSS pixel is not a
  // device pixel, so "1:1" has to account for devicePixelRatio or the preview
  // is silently upscaled ~10% and looks soft.
  const dpr = window.devicePixelRatio || 1;
  const realSizeZoom = scaleRatio > 0 ? 1 / (scaleRatio * dpr) : 1;
  // Allow the slider to reach real size even when it's beyond the default max.
  const maxZoom = Math.max(3, realSizeZoom);

  // What the user actually sees: share of the capture's native pixels on screen.
  const effectivePercent = Math.round(zoom * scaleRatio * dpr * 100);

  const handleZoomIn = () => {
    const newZoom = Math.min(zoom + 0.1, maxZoom);
    onZoomChange(newZoom);
  };

  const handleZoomOut = () => {
    const newZoom = Math.max(zoom - 0.1, 0.5);
    onZoomChange(newZoom);
  };

  const handleFit = () => {
    onZoomChange(1);
  };

  const handleRealSize = () => {
    onZoomChange(realSizeZoom);
  };

  const handleSliderChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onZoomChange(parseFloat(e.target.value));
  };

  return (
    <div className="zoom-bar">
      <button className="zoom-button" onClick={handleZoomOut} title="Zoom Out">
        −
      </button>
      <input
        type="range"
        min="0.5"
        max={maxZoom}
        step="0.1"
        value={zoom}
        onChange={handleSliderChange}
        className="zoom-slider"
      />
      <button className="zoom-button" onClick={handleZoomIn} title="Zoom In">
        +
      </button>
      {/* Display only: it used to be a "reset to fit" button, so clicking the
          "100%" label silently dropped the view back to the downscaled fit. */}
      <span className="zoom-button zoom-reset zoom-readout" title="Share of the capture's native pixels on screen">
        {effectivePercent}%
      </span>
      <div className="zoom-divider" />
      <button className="zoom-text-button" onClick={handleFit} title="Fit to window">
        Fit
      </button>
      <button
        className="zoom-text-button"
        onClick={handleRealSize}
        title="Real size: 1 image pixel = 1 screen pixel"
      >
        Real Size
      </button>
    </div>
  );
};

export default ZoomBar;
