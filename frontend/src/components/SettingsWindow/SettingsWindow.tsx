import React, { useState, useEffect } from 'react';
import { GetSettings, UpdateSetting, PauseHotkeys, ResumeHotkeys, TestUploadConnection } from '../../../wailsjs/go/main/App';
import { Settings } from '../../types';
import './SettingsWindow.css';

type ConnectionStatus = 'idle' | 'testing' | 'success' | 'error';

interface SettingsWindowProps {
  onClose: () => void;
}

const SettingsWindow: React.FC<SettingsWindowProps> = ({ onClose }) => {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [captureShortcut, setCaptureShortcut] = useState('');
  const [isRecording, setIsRecording] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [showKeyPicker, setShowKeyPicker] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('idle');
  const [connectionMessage, setConnectionMessage] = useState('');

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
      setSettings(data as any);
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
    let hasMainKey = false;
    if (!['Meta', 'Control', 'Alt', 'Shift'].includes(e.key)) {
      // Normalize key name
      let keyName = e.key;

      // Handle special cases
      if (keyName === 'Print') keyName = 'PrintScreen';

      keys.push(keyName);
      hasMainKey = true;
    }

    // While only modifiers are held (e.g. Ctrl+Shift), keep recording so the
    // user can still press the actual key (e.g. Ctrl+Shift+D). Do NOT commit yet.
    if (!hasMainKey) return;

    // Accept if:
    // 1. Has at least one modifier + main key (keys.length > 1)
    // 2. OR is a special key that can be used alone
    const mainKey = keys[keys.length - 1];
    const isSpecialKey = specialKeys.includes(mainKey);

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

  const handleSpecialKeySelect = (key: string) => {
    setCaptureShortcut(key);
    setShowKeyPicker(false);
  };

  // Special keys that browsers can't capture (like PrintScreen)
  const specialKeyOptions = [
    { value: 'PrintScreen', keys: ['PrtSc'], category: 'Single Key' },
    { value: 'Ctrl+PrintScreen', keys: ['Ctrl', 'PrtSc'], category: 'With Modifier' },
    { value: 'Alt+PrintScreen', keys: ['Alt', 'PrtSc'], category: 'With Modifier' },
    { value: 'Shift+PrintScreen', keys: ['Shift', 'PrtSc'], category: 'With Modifier' },
    { value: 'Ctrl+Shift+S', keys: ['Ctrl', 'Shift', 'S'], category: 'Common' },
    { value: 'Ctrl+Shift+4', keys: ['Ctrl', 'Shift', '4'], category: 'Common' },
    { value: 'F13', keys: ['F13'], category: 'Function Keys' },
    { value: 'F14', keys: ['F14'], category: 'Function Keys' },
    { value: 'F15', keys: ['F15'], category: 'Function Keys' },
  ];

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

      // Update run_at_startup setting
      await UpdateSetting('run_at_startup', settings.run_at_startup || false);

      // Update after_upload_action setting
      if (settings.after_upload_action) {
        await UpdateSetting('after_upload_action', settings.after_upload_action);
      }

      // Update server URL + API key
      await UpdateSetting('server_url', settings.server_url || '');
      await UpdateSetting('api_key', settings.api_key || '');

      alert('Settings saved successfully!');
      onClose();
    } catch (error) {
      console.error('Failed to save settings:', error);
      alert('Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  const handleTestConnection = async () => {
    if (!settings) return;
    setConnectionStatus('testing');
    setConnectionMessage('');
    try {
      // Persist current values first — the backend reads them from settings.
      await UpdateSetting('server_url', settings.server_url || '');
      await UpdateSetting('api_key', settings.api_key || '');
      await TestUploadConnection();
      setConnectionStatus('success');
      setConnectionMessage('Connected — API key is valid.');
    } catch (err) {
      setConnectionStatus('error');
      setConnectionMessage(err instanceof Error ? err.message : String(err));
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
                <div className="shortcut-input-group">
                  <div className="shortcut-input-wrapper">
                    <input
                      type="text"
                      className={`shortcut-input ${isRecording ? 'recording' : ''}`}
                      value={captureShortcut}
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
                  </button>
                </div>

                {showKeyPicker && (
                  <div className="key-picker">
                    <div className="key-picker-header">
                      <span className="key-picker-title">Choose a shortcut</span>
                      <span className="key-picker-subtitle">PrintScreen cannot be recorded in browser</span>
                    </div>
                    <div className="key-picker-grid">
                      {specialKeyOptions.map(opt => (
                        <button
                          key={opt.value}
                          type="button"
                          className={`key-combo ${captureShortcut === opt.value ? 'selected' : ''}`}
                          onClick={() => handleSpecialKeySelect(opt.value)}
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
                <p className="setting-hint">Click input to record, or click keyboard icon for presets</p>
              </div>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h3>Account &amp; Server</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">Server URL</label>
              <div className="setting-input">
                <input
                  type="text"
                  value={settings.server_url ?? ''}
                  onChange={(e) => {
                    setSettings({ ...settings, server_url: e.target.value });
                    setConnectionStatus('idle');
                  }}
                  placeholder="https://api.fasp.me"
                />
              </div>
            </div>

            <div className="setting-row">
              <label className="setting-label">API Key</label>
              <div className="setting-input">
                <input
                  type="password"
                  value={settings.api_key ?? ''}
                  onChange={(e) => {
                    setSettings({ ...settings, api_key: e.target.value });
                    setConnectionStatus('idle');
                  }}
                  placeholder="fsk_live_..."
                  autoComplete="off"
                  spellCheck={false}
                />
                <p className="setting-hint">
                  Create an API key in fasp under Settings → API Keys (scope files:write).
                </p>
              </div>
            </div>

            <div className="setting-row">
              <label className="setting-label">Connection</label>
              <div className="setting-input">
                <button
                  type="button"
                  className="btn-test-connection"
                  onClick={handleTestConnection}
                  disabled={connectionStatus === 'testing' || !settings.api_key}
                >
                  {connectionStatus === 'testing' ? 'Testing…' : 'Test Connection'}
                </button>
                {connectionStatus === 'success' && (
                  <p className="setting-hint" style={{ color: '#22c55e' }}>{connectionMessage}</p>
                )}
                {connectionStatus === 'error' && (
                  <p className="setting-hint" style={{ color: '#ef4444' }}>{connectionMessage}</p>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h3>General</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">Run at Startup</label>
              <div className="setting-input-checkbox">
                <input
                  type="checkbox"
                  id="run-at-startup"
                  checked={settings.run_at_startup || false}
                  onChange={(e) => setSettings({ ...settings, run_at_startup: e.target.checked })}
                />
                <label htmlFor="run-at-startup" className="checkbox-label">
                  Start when login
                </label>
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

        <div className="settings-section">
          <h3>Upload Options</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">After Upload Action</label>
              <div className="setting-input">
                <select
                  value={settings.after_upload_action || 'direct'}
                  onChange={(e) => setSettings({ ...settings, after_upload_action: e.target.value as any })}
                >
                  <option value="none">Do Nothing</option>
                  <option value="direct">Copy Direct URL</option>
                  <option value="site">Copy Public URL</option>
                  <option value="image">Copy Image Markdown</option>
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

