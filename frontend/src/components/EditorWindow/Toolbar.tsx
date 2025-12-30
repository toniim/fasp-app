import React from 'react';
import { useEditorStore } from '../../store/editorStore';
import { AnnotationTool } from '../../types';

const Toolbar: React.FC = () => {
  const { selectedTool, setSelectedTool, selectedColor, setSelectedColor, selectedSize, setSelectedSize, undo, redo } = useEditorStore();

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modKey = isMac ? 'Cmd' : 'Ctrl';

  const tools: { id: AnnotationTool; label: string; icon: string; shortcut: string }[] = [
    { id: 'select', label: 'Select', icon: '↖', shortcut: 'S' },
    { id: 'crop', label: 'Crop', icon: '✂', shortcut: 'C' },
    { id: 'rectangle', label: 'Rectangle', icon: '▭', shortcut: 'R' },
    { id: 'arrow', label: 'Arrow', icon: '→', shortcut: 'A' },
    { id: 'numbered-arrow', label: 'Numbered Arrow', icon: '①', shortcut: 'N' },
    { id: 'arrow-text', label: 'Arrow with Text', icon: '➜', shortcut: 'W' },
    { id: 'text', label: 'Text', icon: 'T', shortcut: 'T' },
    { id: 'highlight', label: 'Highlight', icon: '◧', shortcut: 'H' },
    { id: 'blur', label: 'Blur', icon: '◎', shortcut: 'B' },
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
            >
              <span className="tool-icon">{tool.icon}</span>
              <span className="tool-label">{tool.label}</span>
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
        <button className="action-button" onClick={undo} title={`Undo (${modKey}+Z)`}>
          ◄
        </button>
        <button className="action-button" onClick={redo} title={`Redo (${modKey}+Shift+Z)`}>
          ►
        </button>
      </div>
    </div>
  );
};

export default Toolbar;

