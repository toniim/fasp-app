import React, { useState, useEffect, useCallback } from 'react';
import { SaveImage, OpenSaveDialog, GenerateFilename, CopyImageToClipboard, GetSettings, HideWindow } from '../../../wailsjs/go/main/App';
import { useEditorStore } from '../../store/editorStore';
import Toast from '../Toast/Toast';
import { UploadButton } from './UploadButton';

// SVG Icons - Clean, macOS-style
const Icons = {
  quickSave: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 9V13H3V9" />
      <path d="M8 3V10" />
      <path d="M5 7L8 10L11 7" />
    </svg>
  ),
  saveAs: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 14H4C3.44772 14 3 13.5523 3 13V3C3 2.44772 3.44772 2 4 2H9L13 6V13C13 13.5523 12.5523 14 12 14Z" />
      <path d="M9 2V6H13" />
    </svg>
  ),
  copy: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="5" y="5" width="9" height="9" rx="1" />
      <path d="M11 5V3C11 2.44772 10.5523 2 10 2H3C2.44772 2 2 2.44772 2 3V10C2 10.5523 2.44772 11 3 11H5" />
    </svg>
  ),
  crop: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 1V12H15" />
      <path d="M1 4H12V15" />
    </svg>
  ),
  close: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4L12 12" />
      <path d="M12 4L4 12" />
    </svg>
  ),
};

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

  // The Konva stage is displayed downscaled by `scaleRatio` to fit the editor
  // viewport. Exporting at a fixed pixelRatio would bake in that downscale and
  // produce a sub-native (blurry) image. Re-rendering at 1/scaleRatio restores
  // the original capture resolution from the full-res source image.
  const exportPixelRatio = scaleRatio > 0 ? 1 / scaleRatio : 1;

  // Export the current stage (image + all annotations) to a PNG data URL at
  // native resolution. Called lazily at action time so the latest annotations
  // are always included.
  const exportDataURL = useCallback((): string => {
    if (!stageRef.current) return '';
    return stageRef.current.toDataURL({ pixelRatio: exportPixelRatio });
  }, [stageRef, exportPixelRatio]);

  const handleSave = async () => {
    if (!stageRef.current || !image) return;

    try {
      setIsSaving(true);

      // Export stage to data URL (image + latest annotations, native resolution)
      const dataURL = exportDataURL();

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
        HideWindow();
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
      // Export stage to data URL (image + latest annotations, native resolution)
      const dataURL = exportDataURL();

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Convert base64 to bytes
      const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0));

      // Copy to clipboard via backend
      await CopyImageToClipboard(Array.from(bytes));

      setToast({ message: 'Copied to clipboard!', type: 'success' });

      // Close window after successful copy
      setTimeout(() => {
        HideWindow();
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

  const handleUploadComplete = (publicUrl: string, directUrl: string) => {
    setToast({ message: 'Upload successful! URL copied to clipboard.', type: 'success' });

    // Close window after successful upload
    setTimeout(() => {
      HideWindow();
    }, 1500);
  };

  const handleUploadError = (error: string) => {
    setToast({ message: `Upload failed: ${error}`, type: 'error' });
  };

  const handleQuickSave = async () => {
    if (!stageRef.current || !image) return;

    try {
      setIsSaving(true);

      // Export stage to data URL (image + latest annotations, native resolution)
      const dataURL = exportDataURL();

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Get settings for default save path
      const settings = await GetSettings();
      let defaultPath = settings.default_save_path;

      // If no default path, use home directory
      if (!defaultPath) {
        defaultPath = '/Users/Shared/Fasp'; // Fallback path
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
        HideWindow();
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
          HideWindow();
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
              {Icons.crop}
              <span>Apply Crop</span>
            </button>
            <button className="secondary-button" onClick={handleCancelCrop} title={`Cancel (${modKey}+Shift+C)`}>
              {Icons.close}
              <span>Cancel</span>
            </button>
          </>
        ) : (
          <>
            <button className="primary-button" onClick={handleQuickSave} disabled={isSaving} title={`Quick Save (${modKey}+S)`}>
              {Icons.quickSave}
              <span>{isSaving ? 'Saving...' : 'Quick Save'}</span>
            </button>
            <button className="secondary-button" onClick={handleSave} disabled={isSaving} title={`Save As (${modKey}+Shift+S)`}>
              {Icons.saveAs}
              <span>{isSaving ? 'Saving...' : 'Save As'}</span>
            </button>
            <button className="secondary-button" onClick={handleCopy} title={`Copy (${modKey}+C)`}>
              {Icons.copy}
              <span>Copy</span>
            </button>
            <UploadButton
              getImageData={exportDataURL}
              hasImage={!!image}
              onComplete={handleUploadComplete}
              onError={handleUploadError}
            />
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

