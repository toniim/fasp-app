import React, { useCallback, useState } from 'react';
import { useEditorStore } from '../../store/editorStore';
import { AnnotationTool } from '../../types';
import { CaptureActiveDisplay, OpenImageDialog, ReadImageFile, ShowWindow } from '../../../wailsjs/go/main/App';
import { WindowHide } from '../../../wailsjs/runtime/runtime';
import Toast from '../Toast/Toast';
import BackdropPanel from './BackdropPanel';
import { fontForSize, strokeForSize } from '../../utils/annotation-sizes';

// SVG Icons - Clean, macOS-style line icons
const Icons = {
  select: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 2L3 12L6.5 8.5L9 13L10.5 12.5L8 8L12 8L3 2Z" />
    </svg>
  ),
  crop: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 1V12H15" />
      <path d="M1 4H12V15" />
    </svg>
  ),
  rectangle: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="12" height="10" rx="1" />
    </svg>
  ),
  arrow: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 13L13 3" />
      <path d="M6 3H13V10" />
    </svg>
  ),
  numberedArrow: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 14L10 6" />
      <path d="M7 6H10V9" />
      <circle cx="12.5" cy="3.5" r="2.5" />
      <text x="12.5" y="5" fontSize="4" fill="currentColor" textAnchor="middle" stroke="none">1</text>
    </svg>
  ),
  arrowText: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 13L11 5" />
      <path d="M8 5H11V8" />
      <path d="M1 3H6" />
      <path d="M3.5 1V5" />
    </svg>
  ),
  text: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 3H13" />
      <path d="M8 3V14" />
      <path d="M5 14H11" />
    </svg>
  ),
  highlight: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="5" width="12" height="6" rx="1" fill="currentColor" fillOpacity="0.3" />
    </svg>
  ),
  blur: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="12" height="10" rx="1" />
      <path d="M5 6H11" strokeOpacity="0.5" />
      <path d="M5 8H11" strokeOpacity="0.5" />
      <path d="M5 10H11" strokeOpacity="0.5" />
    </svg>
  ),
  undo: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 6H10C12.2091 6 14 7.79086 14 10C14 12.2091 12.2091 14 10 14H6" />
      <path d="M6 3L3 6L6 9" />
    </svg>
  ),
  redo: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 6H6C3.79086 6 2 7.79086 2 10C2 12.2091 3.79086 14 6 14H10" />
      <path d="M10 3L13 6L10 9" />
    </svg>
  ),
  addImage: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="3" width="10" height="9" rx="1" />
      <path d="M2 10L5 7L8 10" />
      <path d="M13 9V15" />
      <path d="M10 12H16" />
    </svg>
  ),
  addShot: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13.5 10.5V12C13.5 12.55 13.05 13 12.5 13H2.5C1.95 13 1.5 12.55 1.5 12V5C1.5 4.45 1.95 4 2.5 4H4.5L5.5 2.5H8.5L9.5 4H11.5" />
      <circle cx="7.5" cy="8.5" r="2" />
      <path d="M13.5 1V7" />
      <path d="M10.5 4H16.5" />
    </svg>
  ),
  arrangeRow: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="4" width="3.5" height="8" rx="0.5" />
      <rect x="6.25" y="4" width="3.5" height="8" rx="0.5" />
      <rect x="11" y="4" width="3.5" height="8" rx="0.5" />
    </svg>
  ),
  arrangeColumn: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4" y="1.5" width="8" height="3.5" rx="0.5" />
      <rect x="4" y="6.25" width="8" height="3.5" rx="0.5" />
      <rect x="4" y="11" width="8" height="3.5" rx="0.5" />
    </svg>
  ),
  arrangeGrid: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="5" height="5" rx="0.5" />
      <rect x="9" y="2" width="5" height="5" rx="0.5" />
      <rect x="2" y="9" width="5" height="5" rx="0.5" />
      <rect x="9" y="9" width="5" height="5" rx="0.5" />
    </svg>
  ),
  bringForward: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="6" width="7" height="7" rx="1" strokeOpacity="0.5" />
      <rect x="6" y="2" width="8" height="8" rx="1" fill="currentColor" fillOpacity="0.3" />
    </svg>
  ),
  sendBackward: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="6" y="2" width="8" height="8" rx="1" strokeOpacity="0.5" />
      <rect x="2" y="6" width="7" height="7" rx="1" fill="currentColor" fillOpacity="0.3" />
    </svg>
  ),
  // A shot on a background with a margin.
  backdrop: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="2" width="13" height="12" rx="2" />
      <rect x="4.5" y="5" width="7" height="6" rx="1" fill="currentColor" fillOpacity="0.35" />
    </svg>
  ),
};

const Toolbar: React.FC = () => {
  const {
    selectedTool, setSelectedTool, selectedColor, setSelectedColor, selectedSize, setSelectedSize, undo, redo,
    images, selectedImageId, background, setBackground, addImage, bringForward, sendBackward,
    imageGap, arrangeImages, setImageGap,
    annotations, selectedAnnotationId, updateAnnotation, highlightOptions, setHighlightOptions,
    rectangleRadius, setRectangleRadius, backdrop,
  } = useEditorStore();
  const [backdropOpen, setBackdropOpen] = useState(false);
  const closeBackdrop = useCallback(() => setBackdropOpen(false), []);
  // A selected highlight or rectangle shows its tool's settings too, so it can
  // be tuned with the select tool after drawing.
  const selectedAnnotation = annotations.find((a) => a.id === selectedAnnotationId);
  const selectedShape =
    selectedAnnotation?.type === 'highlight' || selectedAnnotation?.type === 'rectangle' ? selectedAnnotation : null;
  const optionsTool: AnnotationTool = selectedShape ? selectedShape.type : selectedTool;

  // Corners slider of the highlight and rectangle tools: sets the radius new
  // boxes get and, when a box of that kind is selected, changes that box.
  const cornersSlider = (kind: 'highlight' | 'rectangle', radius: number, setRadius: (r: number) => void) => {
    const value = selectedShape ? selectedShape.cornerRadius ?? 0 : radius;
    return (
      <div className="toolbar-size">
        <label className="size-label" htmlFor={`${kind}-radius`}>Corners</label>
        <input
          id={`${kind}-radius`}
          type="range"
          min="0"
          max="40"
          step="1"
          value={value}
          onChange={(e) => {
            const r = Number(e.target.value);
            setRadius(r);
            if (selectedShape) updateAnnotation(selectedShape.id, { cornerRadius: r });
          }}
          className="size-slider"
          title={selectedShape ? `Corner radius of the selected ${kind}` : `Corner radius of new ${kind}s`}
        />
        <span className="size-value">{`${value}px`}</span>
      </div>
    );
  };
  const [toast, setToast] = useState<{ message: string; type: 'error' } | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [isCapturing, setIsCapturing] = useState(false);

  // Layer-order controls only apply to a selected image (images are only
  // selectable once there are two or more).
  const selectedIndex = selectedImageId ? images.findIndex((im) => im.id === selectedImageId) : -1;

  const handleAddImage = async () => {
    if (isAdding) return;
    setIsAdding(true);
    try {
      const path = await OpenImageDialog();
      if (!path) return; // cancelled
      const data = await ReadImageFile(path);
      await addImage(data);
    } catch (error) {
      console.error('Failed to add image:', error);
      setToast({ message: `Failed to add image: ${error instanceof Error ? error.message : String(error)}`, type: 'error' });
    } finally {
      setIsAdding(false);
    }
  };

  // Add shot: capture the screen without our own window in it, then put the
  // capture on the canvas and open it in the crop tool.
  const handleAddShot = async () => {
    if (isCapturing) return;
    setIsCapturing(true);
    try {
      WindowHide();
      await new Promise((resolve) => setTimeout(resolve, 200));
      const result = await CaptureActiveDisplay();
      ShowWindow();
      await addImage(result.data, { pendingCrop: true });
    } catch (error) {
      console.error('Failed to add screenshot:', error);
      // Show the window even if the capture failed.
      ShowWindow();
      setToast({ message: `Failed to add screenshot: ${error instanceof Error ? error.message : String(error)}`, type: 'error' });
    } finally {
      setIsCapturing(false);
    }
  };

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modKey = isMac ? 'Cmd' : 'Ctrl';

  const tools: { id: AnnotationTool; label: string; icon: React.ReactNode; shortcut: string }[] = [
    { id: 'select', label: 'Select', icon: Icons.select, shortcut: 'S' },
    { id: 'crop', label: 'Crop', icon: Icons.crop, shortcut: 'C' },
    { id: 'rectangle', label: 'Rectangle', icon: Icons.rectangle, shortcut: 'R' },
    { id: 'arrow', label: 'Arrow', icon: Icons.arrow, shortcut: 'A' },
    { id: 'numbered-arrow', label: 'Numbered Arrow', icon: Icons.numberedArrow, shortcut: 'N' },
    { id: 'arrow-text', label: 'Arrow with Text', icon: Icons.arrowText, shortcut: 'W' },
    { id: 'text', label: 'Text', icon: Icons.text, shortcut: 'T' },
    { id: 'highlight', label: 'Highlight', icon: Icons.highlight, shortcut: 'H' },
    { id: 'blur', label: 'Blur', icon: Icons.blur, shortcut: 'B' },
  ];

  const colors = [
    { value: '#ff0000', label: 'Red' },
    { value: '#00ff00', label: 'Green' },
    { value: '#0000ff', label: 'Blue' },
    { value: '#ffff00', label: 'Yellow' },
    { value: '#ff00ff', label: 'Magenta' },
    { value: '#00ffff', label: 'Cyan' },
    { value: '#000000', label: 'Black' },
    { value: '#ffffff', label: 'White' },
  ];

  return (
    <div className="toolbar">
      <div className="toolbar-left">
        <div className="toolbar-tools">
          {tools.map((tool) => (
            <button
              key={tool.id}
              className={`tool-button ${selectedTool === tool.id ? 'active' : ''}`}
              onClick={() => setSelectedTool(tool.id)}
              title={`${tool.label} (${tool.shortcut})`}
              aria-label={tool.label}
            >
              {tool.icon}
            </button>
          ))}
        </div>
        {/* Settings of the tool in use. Highlight has its own; the drawing
            tools share size and colour, and the rectangle adds corners. */}
        {optionsTool === 'highlight' && (
          <>
            <div className="toolbar-size">
              <label className="size-label" htmlFor="highlight-dim">Darkness</label>
              <input
                id="highlight-dim"
                type="range"
                min="0"
                max="90"
                step="5"
                value={Math.round(highlightOptions.dim * 100)}
                onChange={(e) => setHighlightOptions({ dim: Number(e.target.value) / 100 })}
                className="size-slider"
                title="How dark the picture gets outside the highlighted areas"
              />
              <span className="size-value">{`${Math.round(highlightOptions.dim * 100)}%`}</span>
            </div>
            {cornersSlider('highlight', highlightOptions.radius, (radius) => setHighlightOptions({ radius }))}
          </>
        )}
        {optionsTool !== 'highlight' && (
          <div className="toolbar-size">
            <label className="size-label">Size</label>
            <input
              type="range"
              min="1"
              max="5"
              value={selectedSize}
              onChange={(e) => setSelectedSize(Number(e.target.value))}
              className="size-slider"
              title={`Size: ${selectedSize} (${modKey}+1-5 or [ / ])`}
            />
            <span className="size-value">
              {selectedTool === 'text' || selectedAnnotation?.type === 'text'
                ? `${fontForSize(selectedSize)}px`
                : `${strokeForSize(selectedSize)}px`}
            </span>
          </div>
        )}
        {optionsTool === 'rectangle' && cornersSlider('rectangle', rectangleRadius, setRectangleRadius)}
      </div>
      {/* Highlight has no colour; the empty slot keeps the bar from shifting. */}
      <div className="toolbar-colors">
        {optionsTool !== 'highlight' && colors.map((color) => (
          <button
            key={color.value}
            className={`color-button ${selectedColor === color.value ? 'active' : ''}`}
            style={{
              backgroundColor: color.value,
            }}
            onClick={() => setSelectedColor(color.value)}
            title={color.label}
          />
        ))}
        {optionsTool !== 'highlight' && (
          <input
            type="color"
            value={selectedColor}
            onChange={(e) => setSelectedColor(e.target.value)}
            title="Custom color"
            style={{ marginLeft: '4px', width: '20px', height: '20px', cursor: 'pointer', borderRadius: '4px' }}
          />
        )}
      </div>
      <div className="toolbar-images">
        <button
          className={`action-button backdrop-toggle ${backdrop.enabled ? 'active' : ''}`}
          onClick={() => setBackdropOpen((open) => !open)}
          title="Backdrop: put the shot on a background for sharing"
          aria-label="Backdrop"
          aria-expanded={backdropOpen}
        >
          {Icons.backdrop}
        </button>
        {backdropOpen && <BackdropPanel onClose={closeBackdrop} />}
        <button
          className="action-button"
          onClick={handleAddImage}
          disabled={isAdding}
          title="Add image from file"
          aria-label="Add image"
        >
          {Icons.addImage}
        </button>
        <button
          className="action-button"
          onClick={handleAddShot}
          disabled={isCapturing}
          title="Add a screenshot of the current screen"
          aria-label="Add screenshot"
        >
          {Icons.addShot}
        </button>
        {selectedIndex >= 0 && (
          <>
            <button
              className="action-button"
              onClick={() => bringForward(selectedImageId!)}
              disabled={selectedIndex === images.length - 1}
              title="Bring image forward"
              aria-label="Bring image forward"
            >
              {Icons.bringForward}
            </button>
            <button
              className="action-button"
              onClick={() => sendBackward(selectedImageId!)}
              disabled={selectedIndex === 0}
              title="Send image backward"
              aria-label="Send image backward"
            >
              {Icons.sendBackward}
            </button>
          </>
        )}
        {images.length >= 2 && (
          <>
            <button
              className="action-button"
              onClick={() => arrangeImages('row')}
              title="Arrange in a row"
              aria-label="Arrange in a row"
            >
              {Icons.arrangeRow}
            </button>
            <button
              className="action-button"
              onClick={() => arrangeImages('column')}
              title="Arrange in a column"
              aria-label="Arrange in a column"
            >
              {Icons.arrangeColumn}
            </button>
            <button
              className="action-button"
              onClick={() => arrangeImages('grid')}
              title="Arrange in a grid"
              aria-label="Arrange in a grid"
            >
              {Icons.arrangeGrid}
            </button>
            <select
              className="gap-select"
              value={imageGap}
              onChange={(e) => setImageGap(Number(e.target.value))}
              title="Gap between images (re-arranges)"
              aria-label="Gap between images"
            >
              {[...new Set([0, 16, 24, 48, imageGap])].sort((a, b) => a - b).map((g) => (
                <option key={g} value={g}>{`${g}px`}</option>
              ))}
            </select>
          </>
        )}
        {images.length >= 2 && (
          <input
            type="color"
            className="background-swatch"
            value={background}
            onChange={(e) => setBackground(e.target.value)}
            title="Canvas background color"
            aria-label="Canvas background color"
          />
        )}
      </div>
      <div className="toolbar-actions">
        <button className="action-button" onClick={undo} title={`Undo (${modKey}+Z)`} aria-label="Undo">
          {Icons.undo}
        </button>
        <button className="action-button" onClick={redo} title={`Redo (${modKey}+Shift+Z)`} aria-label="Redo">
          {Icons.redo}
        </button>
      </div>
      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
};

export default Toolbar;

