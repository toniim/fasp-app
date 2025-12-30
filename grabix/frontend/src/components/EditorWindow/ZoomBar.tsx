import React from 'react';
import './ZoomBar.css';

interface ZoomBarProps {
  zoom: number;
  onZoomChange: (zoom: number) => void;
}

const ZoomBar: React.FC<ZoomBarProps> = ({ zoom, onZoomChange }) => {
  const handleZoomIn = () => {
    const newZoom = Math.min(zoom + 0.1, 3);
    onZoomChange(newZoom);
  };

  const handleZoomOut = () => {
    const newZoom = Math.max(zoom - 0.1, 0.5);
    onZoomChange(newZoom);
  };

  const handleReset = () => {
    onZoomChange(1);
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
        max="3"
        step="0.1"
        value={zoom}
        onChange={handleSliderChange}
        className="zoom-slider"
      />
      <button className="zoom-button" onClick={handleZoomIn} title="Zoom In">
        +
      </button>
      <button className="zoom-button zoom-reset" onClick={handleReset} title="Reset Zoom">
        {Math.round(zoom * 100)}%
      </button>
    </div>
  );
};

export default ZoomBar;

