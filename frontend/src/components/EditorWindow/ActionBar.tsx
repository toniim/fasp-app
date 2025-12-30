import React, { useState, useEffect } from 'react';
import { SaveImage, OpenSaveDialog, GenerateFilename, CopyImageToClipboard, GetSettings } from '../../../wailsjs/go/main/App';
import { WindowHide } from '../../../wailsjs/runtime/runtime';
import { useEditorStore } from '../../store/editorStore';
import Toast from '../Toast/Toast';

interface ActionBarProps {
  stageRef: React.RefObject<any>;
  scaleRatio: number;
}

interface ToastState {
  message: string;
  type: 'success' | 'error' | 'info';
}

const ActionBar: React.FC<ActionBarProps> = ({ stageRef, scaleRatio }) => {
  const { image, cropRegion, applyCrop, setCropRegion } = useEditorStore();
  const [isSaving, setIsSaving] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  const handleSave = async () => {
    if (!stageRef.current || !image) return;

    try {
      setIsSaving(true);

      // Export stage to data URL
      const dataURL = stageRef.current.toDataURL({ pixelRatio: 2 });

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Generate default filename
      const defaultFilename = await GenerateFilename('png');

      // Open save dialog
      const savePath = await OpenSaveDialog(defaultFilename);

      if (!savePath) {
        setIsSaving(false);
        return;
      }

      // Convert base64 to bytes
      const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0));

      // Save image
      await SaveImage(
        {
          path: savePath,
          format: 'png',
          quality: 90,
        },
        Array.from(bytes)
      );

      setToast({ message: 'Screenshot saved successfully!', type: 'success' });

      // Close window after successful save
      setTimeout(() => {
        WindowHide();
      }, 1500); // Wait for toast to be visible
    } catch (error) {
      console.error('Failed to save screenshot:', error);
      setToast({ message: 'Failed to save screenshot', type: 'error' });
    } finally {
      setIsSaving(false);
    }
  };

  const handleCopy = async () => {
    if (!stageRef.current) return;

    try {
      // Export stage to data URL
      const dataURL = stageRef.current.toDataURL({ pixelRatio: 2 });

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Convert base64 to bytes
      const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0));

      // Copy to clipboard via backend
      await CopyImageToClipboard(Array.from(bytes));

      setToast({ message: 'Copied to clipboard!', type: 'success' });

      // Close window after successful copy
      setTimeout(() => {
        WindowHide();
      }, 1500); // Wait for toast to be visible
    } catch (error) {
      console.error('Failed to copy to clipboard:', error);
      setToast({ message: 'Failed to copy to clipboard', type: 'error' });
    }
  };

  const handleApplyCrop = () => {
    applyCrop(scaleRatio);
  };

  const handleCancelCrop = () => {
    setCropRegion(null);
  };

  const handleQuickSave = async () => {
    if (!stageRef.current || !image) return;

    try {
      setIsSaving(true);

      // Export stage to data URL
      const dataURL = stageRef.current.toDataURL({ pixelRatio: 2 });

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Get settings for default save path
      const settings = await GetSettings();
      let defaultPath = settings.default_save_path;

      // If no default path, use home directory
      if (!defaultPath) {
        defaultPath = '/Users/Shared/Grabix'; // Fallback path
      }

      // Generate filename
      const filename = await GenerateFilename('png');
      const savePath = `${defaultPath}/${filename}`;

      // Convert base64 to bytes
      const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0));

      // Save image
      await SaveImage(
        {
          path: savePath,
          format: 'png',
          quality: 90,
        },
        Array.from(bytes)
      );

      setToast({ message: `Saved to ${filename}`, type: 'success' });

      // Close window after successful quick save
      setTimeout(() => {
        WindowHide();
      }, 1500); // Wait for toast to be visible
    } catch (error) {
      console.error('Failed to quick save:', error);
      setToast({ message: 'Failed to quick save', type: 'error' });
    } finally {
      setIsSaving(false);
    }
  };

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const cmdOrCtrl = isMac ? e.metaKey : e.ctrlKey;

      if (!cmdOrCtrl) return;

      const key = e.key.toLowerCase();

      // Cmd+S / Ctrl+S - Quick Save
      if (key === 's' && !e.shiftKey) {
        e.preventDefault();
        if (!cropRegion) {
          handleQuickSave();
        }
        return;
      }

      // Cmd+Shift+S / Ctrl+Shift+S - Save As
      if (key === 's' && e.shiftKey) {
        e.preventDefault();
        if (!cropRegion) {
          handleSave();
        }
        return;
      }

      // Cmd+C / Ctrl+C - Copy
      if (key === 'c' && !e.shiftKey) {
        e.preventDefault();
        if (!cropRegion) {
          handleCopy();
        }
        return;
      }

      // Cmd+Shift+C / Ctrl+Shift+C - Cancel (crop or close window)
      if (key === 'c' && e.shiftKey) {
        e.preventDefault();
        if (cropRegion) {
          handleCancelCrop();
        } else {
          WindowHide();
        }
        return;
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [cropRegion, handleQuickSave, handleSave, handleCopy, handleCancelCrop]);

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modKey = isMac ? 'Cmd' : 'Ctrl';

  return (
    <>
      <div className="action-bar">
        {cropRegion ? (
          <>
            <button className="primary-button" onClick={handleApplyCrop} title="Apply Crop (Enter)">
              ✂️ Apply Crop
            </button>
            <button className="secondary-button" onClick={handleCancelCrop} title={`Cancel (${modKey}+Shift+C)`}>
              ❌ Cancel
            </button>
          </>
        ) : (
          <>
            <button className="primary-button" onClick={handleQuickSave} disabled={isSaving} title={`Quick Save (${modKey}+S)`}>
              {isSaving ? '...' : '⚡ Quick Save'}
            </button>
            <button className="secondary-button" onClick={handleSave} disabled={isSaving} title={`Save As (${modKey}+Shift+S)`}>
              {isSaving ? '...' : '💾 Save As'}
            </button>
            <button className="secondary-button" onClick={handleCopy} title={`Copy (${modKey}+C)`}>
              📋 Copy
            </button>
          </>
        )}
      </div>
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </>
  );
};

export default ActionBar;

