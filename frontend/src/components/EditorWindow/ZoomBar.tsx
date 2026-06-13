import React from 'react';
import './ZoomBar.css';

interface ZoomBarProps {
  zoom: number;
  onZoomChange: (zoom: number) => void;
  // Base scale applied to fit the image into the viewport. Real (1:1) pixel
  // size is reached when zoom = 1 / scaleRatio.
  scaleRatio: number;
}

const ZoomBar: React.FC<ZoomBarProps> = ({ zoom, onZoomChange, scaleRatio }) => {
  // Real-size zoom: cancel out the fit-scale so 1 image px === 1 screen px.
  const realSizeZoom = scaleRatio > 0 ? 1 / scaleRatio : 1;
  // Allow the slider to reach real size even when it's beyond the default max.
  const maxZoom = Math.max(3, realSizeZoom);

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
      <button className="zoom-button zoom-reset" onClick={handleFit} title="Reset Zoom">
        {Math.round(zoom * 100)}%
      </button>
      <div className="zoom-divider" />
      <button className="zoom-text-button" onClick={handleFit} title="Fit to window">
        Fit
      </button>
      <button className="zoom-text-button" onClick={handleRealSize} title="Real size (1:1)">
        Real Size
      </button>
    </div>
  );
};

export default ZoomBar;
