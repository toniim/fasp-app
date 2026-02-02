import React from 'react';
import { useEditorStore } from '../../store/editorStore';
import { AnnotationTool } from '../../types';

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
};

const Toolbar: React.FC = () => {
  const { selectedTool, setSelectedTool, selectedColor, setSelectedColor, selectedSize, setSelectedSize, undo, redo } = useEditorStore();

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
        {/* Size slider - always visible */}
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
            {selectedTool === 'text'
              ? `${selectedSize === 1 ? 12 : selectedSize === 2 ? 16 : selectedSize === 3 ? 20 : selectedSize === 4 ? 24 : 32}px`
              : `${selectedSize === 1 ? 1 : selectedSize === 2 ? 2 : selectedSize === 3 ? 4 : selectedSize === 4 ? 6 : 8}px`
            }
          </span>
        </div>
      </div>
      <div className="toolbar-colors">
        {colors.map((color) => (
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
        <input
          type="color"
          value={selectedColor}
          onChange={(e) => setSelectedColor(e.target.value)}
          title="Custom color"
          style={{ marginLeft: '4px', width: '20px', height: '20px', cursor: 'pointer', borderRadius: '4px' }}
        />
      </div>
      <div className="toolbar-actions">
        <button className="action-button" onClick={undo} title={`Undo (${modKey}+Z)`} aria-label="Undo">
          {Icons.undo}
        </button>
        <button className="action-button" onClick={redo} title={`Redo (${modKey}+Shift+Z)`} aria-label="Redo">
          {Icons.redo}
        </button>
      </div>
    </div>
  );
};

export default Toolbar;

