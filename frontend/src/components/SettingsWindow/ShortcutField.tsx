import React, { useState } from 'react';
import { PauseHotkeys, ResumeHotkeys } from '../../../wailsjs/go/main/App';

interface ShortcutFieldProps {
  value: string;
  onChange: (value: string) => void;
}

// Special keys that can be used alone (without modifiers)
const SOLO_KEYS = [
  'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
  'F13', 'F14', 'F15', 'F16', 'F17', 'F18', 'F19', 'F20',
  'Help', 'Home', 'End', 'PageUp', 'PageDown', 'Delete',
];

// Combos browsers cannot record (PrintScreen never reaches the web view).
const PRESETS = [
  { value: 'PrintScreen', keys: ['PrtSc'] },
  { value: 'Ctrl+PrintScreen', keys: ['Ctrl', 'PrtSc'] },
  { value: 'Alt+PrintScreen', keys: ['Alt', 'PrtSc'] },
  { value: 'Shift+PrintScreen', keys: ['Shift', 'PrtSc'] },
  { value: 'Ctrl+Shift+S', keys: ['Ctrl', 'Shift', 'S'] },
  { value: 'Ctrl+Shift+4', keys: ['Ctrl', 'Shift', '4'] },
  { value: 'F13', keys: ['F13'] },
  { value: 'F14', keys: ['F14'] },
  { value: 'F15', keys: ['F15'] },
];

const KeyboardIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="2" y="4" width="20" height="16" rx="2" />
    <line x1="6" y1="8" x2="6" y2="8" />
    <line x1="10" y1="8" x2="10" y2="8" />
    <line x1="14" y1="8" x2="14" y2="8" />
    <line x1="18" y1="8" x2="18" y2="8" />
    <line x1="6" y1="12" x2="6" y2="12" />
    <line x1="10" y1="12" x2="10" y2="12" />
    <line x1="14" y1="12" x2="14" y2="12" />
    <line x1="18" y1="12" x2="18" y2="12" />
    <line x1="8" y1="16" x2="16" y2="16" />
  </svg>
);

/**
 * A global-hotkey input: click and press a combination, or pick a preset.
 * Global hotkeys are paused while recording so the combination being pressed
 * does not trigger a capture.
 */
const ShortcutField: React.FC<ShortcutFieldProps> = ({ value, onChange }) => {
  const [isRecording, setIsRecording] = useState(false);
  const [showKeyPicker, setShowKeyPicker] = useState(false);

  const startRecording = async () => {
    setIsRecording(true);
    try {
      await PauseHotkeys();
    } catch (error) {
      console.error('Failed to pause hotkeys:', error);
    }
  };

  const stopRecording = async () => {
    setIsRecording(false);
    try {
      await ResumeHotkeys();
    } catch (error) {
      console.error('Failed to resume hotkeys:', error);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isRecording) return;
    e.preventDefault();
    e.stopPropagation();

    const keys: string[] = [];
    if (e.metaKey) keys.push('Cmd');
    if (e.ctrlKey) keys.push('Ctrl');
    if (e.altKey) keys.push('Alt');
    if (e.shiftKey) keys.push('Shift');

    // While only modifiers are held (e.g. Ctrl+Shift), keep recording so the
    // user can still press the actual key. Do NOT commit yet.
    if (['Meta', 'Control', 'Alt', 'Shift'].includes(e.key)) return;
    keys.push(e.key === 'Print' ? 'PrintScreen' : e.key);

    const mainKey = keys[keys.length - 1];
    if (keys.length > 1 || SOLO_KEYS.includes(mainKey)) {
      onChange(keys.join('+'));
      stopRecording();
    }
  };

  return (
    <div className="setting-input">
      <div className="shortcut-input-group">
        <div className="shortcut-input-wrapper">
          <input
            type="text"
            className={`shortcut-input ${isRecording ? 'recording' : ''}`}
            value={value}
            readOnly
            placeholder="Click to record..."
            onClick={startRecording}
            onKeyDown={handleKeyDown}
            onBlur={() => {
              if (!showKeyPicker) stopRecording();
            }}
          />
          {isRecording && (
            <span className="recording-indicator">
              <span className="recording-dot"></span>
              Listening...
            </span>
          )}
        </div>
        <button
          type="button"
          className={`btn-keyboard ${showKeyPicker ? 'active' : ''}`}
          onClick={() => setShowKeyPicker(!showKeyPicker)}
          title="Choose from presets"
        >
          {KeyboardIcon}
        </button>
      </div>

      {showKeyPicker && (
        <div className="key-picker">
          <div className="key-picker-header">
            <span className="key-picker-title">Choose a shortcut</span>
            <span className="key-picker-subtitle">PrintScreen cannot be recorded in browser</span>
          </div>
          <div className="key-picker-grid">
            {PRESETS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                className={`key-combo ${value === opt.value ? 'selected' : ''}`}
                onClick={() => {
                  onChange(opt.value);
                  setShowKeyPicker(false);
                }}
              >
                <div className="key-combo-keys">
                  {opt.keys.map((key, idx) => (
                    <span key={idx}>
                      <kbd className="key-cap">{key}</kbd>
                      {idx < opt.keys.length - 1 && <span className="key-plus">+</span>}
                    </span>
                  ))}
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default ShortcutField;
