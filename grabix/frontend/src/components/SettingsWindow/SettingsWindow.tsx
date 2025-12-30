import React, { useState, useEffect } from 'react';
import { GetSettings, UpdateSetting, PauseHotkeys, ResumeHotkeys } from '../../../wailsjs/go/main/App';
import { Settings } from '../../types';
import './SettingsWindow.css';

interface SettingsWindowProps {
  onClose: () => void;
}

const SettingsWindow: React.FC<SettingsWindowProps> = ({ onClose }) => {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [captureShortcut, setCaptureShortcut] = useState('');
  const [isRecording, setIsRecording] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  // Handle Escape key to close settings
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !isRecording) {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isRecording, onClose]);

  const loadSettings = async () => {
    try {
      const data = await GetSettings();
      setSettings(data);
      setCaptureShortcut(data.hotkeys?.capture_fullscreen || data.hotkeys?.capture || 'Cmd+Shift+5');
    } catch (error) {
      console.error('Failed to load settings:', error);
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

    // Special keys that can be used alone (without modifiers)
    // Function keys + navigation keys
    const specialKeys = [
      'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
      'F13', 'F14', 'F15', 'F16', 'F17', 'F18', 'F19', 'F20',
      'Help', 'Home', 'End', 'PageUp', 'PageDown', 'Delete'
    ];

    // Add the main key (not modifier keys)
    if (!['Meta', 'Control', 'Alt', 'Shift'].includes(e.key)) {
      // Normalize key name
      let keyName = e.key;

      // Handle special cases
      if (keyName === 'Print') keyName = 'PrintScreen';

      keys.push(keyName);
    }

    // Accept if:
    // 1. Has modifiers + key (keys.length > 1)
    // 2. OR is a special key that can be used alone
    const mainKey = keys[keys.length - 1];
    const isSpecialKey = mainKey && specialKeys.includes(mainKey);

    if (keys.length > 1 || isSpecialKey) {
      setCaptureShortcut(keys.join('+'));
      stopRecording();
    }
  };

  const startRecording = async () => {
    setIsRecording(true);
    try {
      await PauseHotkeys();
      console.log('Hotkeys paused for recording');
    } catch (error) {
      console.error('Failed to pause hotkeys:', error);
    }
  };

  const stopRecording = async () => {
    setIsRecording(false);
    try {
      await ResumeHotkeys();
      console.log('Hotkeys resumed after recording');
    } catch (error) {
      console.error('Failed to resume hotkeys:', error);
    }
  };

  const handleSave = async () => {
    if (!settings) return;

    setIsSaving(true);
    try {
      await UpdateSetting('hotkeys.capture_fullscreen', captureShortcut);

      // Update other settings if needed
      if (settings.default_save_path) {
        await UpdateSetting('default_save_path', settings.default_save_path);
      }
      if (settings.default_format) {
        await UpdateSetting('default_format', settings.default_format);
      }

      alert('Settings saved successfully!');
      onClose();
    } catch (error) {
      console.error('Failed to save settings:', error);
      alert('Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  if (!settings) {
    return (
      <>
        <div className="settings-backdrop" onClick={onClose} />
        <div className="settings-window">
          <div className="settings-loading">Loading settings...</div>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="settings-backdrop" onClick={onClose} />
      <div className="settings-window">
      <div className="settings-header">
        <h2>Settings</h2>
        <button className="close-btn" onClick={onClose}>×</button>
      </div>

      <div className="settings-content">
        <div className="settings-section">
          <h3>Hotkeys</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">Capture Screenshot</label>
              <div className="setting-input">
                <div className="shortcut-input-wrapper">
                  <input
                    type="text"
                    className="shortcut-input"
                    value={captureShortcut}
                    readOnly
                    placeholder="Click to record shortcut"
                    onClick={startRecording}
                    onKeyDown={handleKeyDown}
                    onBlur={stopRecording}
                  />
                  {isRecording && (
                    <span className="recording-indicator">Press keys...</span>
                  )}
                </div>
                <p className="setting-hint">Click the input and press your desired key combination</p>
              </div>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h3>Save Options</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">Default Save Location</label>
              <div className="setting-input">
                <input
                  type="text"
                  value={settings.default_save_path || ''}
                  onChange={(e) => setSettings({ ...settings, default_save_path: e.target.value })}
                  placeholder="~/Pictures/Screenshots"
                />
              </div>
            </div>

            <div className="setting-row">
              <label className="setting-label">Default Format</label>
              <div className="setting-input">
                <select
                  value={settings.default_format || 'png'}
                  onChange={(e) => setSettings({ ...settings, default_format: e.target.value })}
                >
                  <option value="png">PNG</option>
                  <option value="jpg">JPEG</option>
                </select>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="settings-footer">
        <button className="btn-cancel" onClick={onClose}>Cancel</button>
        <button className="btn-save" onClick={handleSave} disabled={isSaving}>
          {isSaving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
    </>
  );
};

export default SettingsWindow;

