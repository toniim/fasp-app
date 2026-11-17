import React, { useState, useEffect } from 'react';
import { GetSettings, UpdateSetting, TestUploadConnection, ListMicrophones } from '../../../bindings/github.com/heytonyne/fasp/app';
import ShortcutField from './ShortcutField';
import { Settings } from '../../types';
import './SettingsWindow.css';

type ConnectionStatus = 'idle' | 'testing' | 'success' | 'error';

interface SettingsWindowProps {
  onClose: () => void;
}

const SettingsWindow: React.FC<SettingsWindowProps> = ({ onClose }) => {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [hotkeys, setHotkeys] = useState<Record<string, string>>({});
  const [mics, setMics] = useState<{ id: string; name: string }[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('idle');
  const [connectionMessage, setConnectionMessage] = useState('');

  useEffect(() => {
    loadSettings();
  }, []);

  // Handle Escape key to close settings
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Esc while a shortcut field is listening belongs to that field.
      if (e.key === 'Escape' && !document.querySelector('.shortcut-input.recording')) {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const loadSettings = async () => {
    try {
      const data = await GetSettings();
      if (!data) throw new Error('the settings could not be read');
      setSettings(data as any);
      setHotkeys({
        capture_fullscreen: data.hotkeys?.capture_fullscreen || data.hotkeys?.capture || '',
        capture_zone: data.hotkeys?.capture_zone || '',
        repeat_zone: data.hotkeys?.repeat_zone || '',
        record: data.hotkeys?.record || '',
      });
      ListMicrophones().then((list) => setMics(list || [])).catch(() => setMics([]));
    } catch (error) {
      console.error('Failed to load settings:', error);
    }
  };

  const handleSave = async () => {
    if (!settings) return;

    // One combination can only trigger one action: the second registration
    // would silently fail.
    const combos = Object.values(hotkeys).filter(Boolean);
    const used = combos.map((k) => k.toLowerCase());
    const duplicate = combos.find((_, i) => used.indexOf(used[i]) !== i);
    if (duplicate) {
      alert(`The shortcut ${duplicate} is assigned to more than one action. Pick a different one.`);
      return;
    }

    setIsSaving(true);
    try {
      for (const key of ['capture_fullscreen', 'capture_zone', 'repeat_zone', 'record']) {
        await UpdateSetting(`hotkeys.${key}`, hotkeys[key] || '');
      }
      await UpdateSetting('record_system_audio', settings.record_system_audio ?? true);
      await UpdateSetting('record_microphone', settings.record_microphone ?? false);
      await UpdateSetting('record_mic_device', settings.record_mic_device || '');
      await UpdateSetting('record_fps', settings.record_fps || 30);

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

      await UpdateSetting('upload_public', settings.upload_public ?? true);

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
              <ShortcutField
                value={hotkeys.capture_fullscreen || ''}
                onChange={(v) => setHotkeys({ ...hotkeys, capture_fullscreen: v })}
              />
            </div>
            <div className="setting-row">
              <label className="setting-label">Capture Zone</label>
              <ShortcutField
                value={hotkeys.capture_zone || ''}
                onChange={(v) => setHotkeys({ ...hotkeys, capture_zone: v })}
              />
            </div>
            <div className="setting-row">
              <label className="setting-label">Repeat Last Zone</label>
              <ShortcutField
                value={hotkeys.repeat_zone || ''}
                onChange={(v) => setHotkeys({ ...hotkeys, repeat_zone: v })}
              />
            </div>
            <div className="setting-row">
              <label className="setting-label">Start / Stop Recording</label>
              <ShortcutField
                value={hotkeys.record || ''}
                onChange={(v) => setHotkeys({ ...hotkeys, record: v })}
              />
            </div>
            <p className="setting-hint">
              Capture Zone freezes the screen so you can drag the area to keep. Repeat Last Zone captures
              that same area again at once, without the overlay, and has no shortcut until you set one.
              Click an input to record a shortcut, or the keyboard icon for presets.
            </p>
          </div>
        </div>

        <div className="settings-section">
          <h3>Recording</h3>

          <div className="settings-table">
            <div className="setting-row">
              <label className="setting-label">Audio</label>
              <div className="setting-input-checkbox">
                <input
                  type="checkbox"
                  id="record-system-audio"
                  checked={settings.record_system_audio ?? true}
                  onChange={(e) => setSettings({ ...settings, record_system_audio: e.target.checked })}
                />
                <label htmlFor="record-system-audio" className="checkbox-label">
                  Computer sound
                </label>
                <input
                  type="checkbox"
                  id="record-microphone"
                  checked={settings.record_microphone ?? false}
                  onChange={(e) => setSettings({ ...settings, record_microphone: e.target.checked })}
                />
                <label htmlFor="record-microphone" className="checkbox-label">
                  Microphone
                </label>
              </div>
            </div>
            <div className="setting-row">
              <label className="setting-label">Microphone</label>
              <div className="setting-input">
                <select
                  value={settings.record_mic_device || ''}
                  onChange={(e) => setSettings({ ...settings, record_mic_device: e.target.value })}
                >
                  <option value="">System default</option>
                  {mics.map((m) => (
                    <option key={m.id} value={m.id}>{m.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="setting-row">
              <label className="setting-label">Frame Rate</label>
              <div className="setting-input">
                <select
                  value={settings.record_fps || 30}
                  onChange={(e) => setSettings({ ...settings, record_fps: Number(e.target.value) })}
                >
                  <option value={15}>15 fps</option>
                  <option value={30}>30 fps</option>
                  <option value={60}>60 fps</option>
                </select>
                <p className="setting-hint">Defaults for new recordings; you can change them each time you start one.</p>
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

            <div className="setting-row">
              <label className="setting-label">Visibility</label>
              <div className="setting-input-checkbox">
                <input
                  type="checkbox"
                  id="upload-public"
                  checked={settings.upload_public ?? true}
                  onChange={(e) => setSettings({ ...settings, upload_public: e.target.checked })}
                />
                <label htmlFor="upload-public" className="checkbox-label">
                  Upload as public
                </label>
              </div>
            </div>
            <p className="setting-hint">
              A public link opens for anyone who has it. A private link opens only for you, the owner.
              The Upload menu in the editor can choose either for a single upload. Password
              and expiry are not available.
            </p>
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

