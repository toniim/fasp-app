import React from 'react';
import { useEditorStore } from '../../store/editorStore';
import { AnnotationTool } from '../../types';

const Toolbar: React.FC = () => {
  const { selectedTool, setSelectedTool, selectedColor, setSelectedColor, undo, redo } = useEditorStore();

  const tools: { id: AnnotationTool; label: string; icon: string; shortcut: string }[] = [
    { id: 'select', label: 'Select', icon: '↖', shortcut: 'S' },
    { id: 'crop', label: 'Crop', icon: '✂', shortcut: 'C' },
    { id: 'rectangle', label: 'Rectangle', icon: '▭', shortcut: 'R' },
    { id: 'arrow', label: 'Arrow', icon: '→', shortcut: 'A' },
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
        <button className="action-button" onClick={undo} title="Undo (Cmd+Z)">
          ↶
        </button>
        <button className="action-button" onClick={redo} title="Redo (Cmd+Shift+Z)">
          ↷
        </button>
      </div>
    </div>
  );
};

export default Toolbar;

