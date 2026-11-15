import React, { useState, useEffect, useCallback } from 'react';
import { SaveImage, QuickSaveImage, OpenSaveDialog, GenerateFilename, CopyImageToClipboard, HideWindow } from '../../../wailsjs/go/main/App';
import { useEditorStore, selectHasContent } from '../../store/editorStore';
import { Frame } from '../../types';
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
  keep: (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 8.5L6.5 12L13 4.5" />
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
  // The exported area in document pixels. The stage itself is the viewport,
  // so exporting has to crop to this box (and drop zoom/pan first).
  frame: Frame;
}

interface ToastState {
  message: string;
  type: 'success' | 'error' | 'info';
}

// Wails rejects with the Go error string rather than an Error instance, so
// unwrap both shapes instead of swallowing the reason.
const errorText = (error: unknown): string => {
  if (error instanceof Error) return error.message;
  const text = String(error ?? '').trim();
  return text === '' ? 'unknown error' : text;
};

const ActionBar: React.FC<ActionBarProps> = ({ stageRef, frame }) => {
  const { cropRegion, applyCrop, setCropRegion, pendingCrop, keepPendingCrop, cancelPendingImage } = useEditorStore();
  // A crop in progress: a region being drawn, or an Add shot waiting for its crop.
  const cropping = cropRegion !== null || pendingCrop !== null;
  const hasImage = useEditorStore(selectHasContent);
  const [isSaving, setIsSaving] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  // Export the image + annotations at native resolution. The document is in
  // doc px (one native image pixel each), so with the stage at scale 1 and
  // position 0 the layer offset puts the frame at the origin and a pixelRatio
  // of 1 renders it without any resampling. The stage is the viewport (it may
  // be zoomed and panned), so reset the transform, crop to the frame and
  // restore afterwards.
  const exportDataURL = useCallback((): string => {
    const stage = stageRef.current;
    if (!stage) return '';

    const prevScale = { x: stage.scaleX(), y: stage.scaleY() };
    const prevPos = { x: stage.x(), y: stage.y() };

    // Editor chrome (selection handles, hover box) is not part of the picture.
    const chrome: any[] = stage.find('.editor-chrome');
    chrome.forEach((layer) => layer.visible(false));

    stage.scale({ x: 1, y: 1 });
    stage.position({ x: 0, y: 0 });

    try {
      return stage.toDataURL({
        x: 0,
        y: 0,
        width: frame.width,
        height: frame.height,
        pixelRatio: 1,
      });
    } finally {
      chrome.forEach((layer) => layer.visible(true));
      stage.scale(prevScale);
      stage.position(prevPos);
      stage.batchDraw();
    }
  }, [stageRef, frame]);

  const handleSave = async () => {
    if (!stageRef.current || !hasImage) return;

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
      setToast({ message: `Save failed: ${errorText(error)}`, type: 'error' });
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
      setToast({ message: `Copy failed: ${errorText(error)}`, type: 'error' });
    }
  };

  const handleApplyCrop = () => {
    applyCrop();
  };

  // Cancel: during an Add shot it removes the new image (undoing its add);
  // otherwise it drops the region being drawn.
  const handleCancelCrop = () => {
    if (pendingCrop) {
      cancelPendingImage();
    } else {
      setCropRegion(null);
    }
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
    if (!stageRef.current || !hasImage) return;

    try {
      setIsSaving(true);

      // Export stage to data URL (image + latest annotations, native resolution)
      const dataURL = exportDataURL();

      // Remove data URL prefix to get base64
      const base64Data = dataURL.replace(/^data:image\/\w+;base64,/, '');

      // Convert base64 to bytes
      const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0));

      // Go resolves the folder (configured default, else platform default),
      // builds the filename and saves — no platform path guessing here.
      const savedPath = await QuickSaveImage(Array.from(bytes));

      setToast({ message: `Saved to ${savedPath}`, type: 'success' });

      // Close window after successful quick save
      setTimeout(() => {
        HideWindow();
      }, 1500); // Wait for toast to be visible
    } catch (error) {
      console.error('Failed to quick save:', error);
      setToast({ message: `Quick save failed: ${errorText(error)}`, type: 'error' });
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
        if (!cropping) {
          handleQuickSave();
        }
        return;
      }

      // Cmd+Shift+S / Ctrl+Shift+S - Save As
      if (key === 's' && e.shiftKey) {
        e.preventDefault();
        if (!cropping) {
          handleSave();
        }
        return;
      }

      // Cmd+C / Ctrl+C - Copy
      if (key === 'c' && !e.shiftKey) {
        e.preventDefault();
        if (!cropping) {
          handleCopy();
        }
        return;
      }

      // Cmd+Shift+C / Ctrl+Shift+C - Cancel (crop or close window)
      if (key === 'c' && e.shiftKey) {
        e.preventDefault();
        if (cropping) {
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
  }, [cropping, handleQuickSave, handleSave, handleCopy, handleCancelCrop]);

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modKey = isMac ? 'Cmd' : 'Ctrl';

  return (
    <>
      <div className="action-bar">
        {cropping ? (
          <>
            <button
              className="primary-button"
              onClick={handleApplyCrop}
              disabled={!cropRegion}
              title={cropRegion ? 'Apply Crop (Enter)' : 'Drag on the image to choose the part to keep'}
            >
              {Icons.crop}
              <span>Apply Crop</span>
            </button>
            {pendingCrop && (
              <button className="secondary-button" onClick={keepPendingCrop} title="Keep the whole screenshot (Esc)">
                {Icons.keep}
                <span>Keep Full</span>
              </button>
            )}
            <button
              className="secondary-button"
              onClick={handleCancelCrop}
              title={pendingCrop ? `Remove this screenshot (${modKey}+Shift+C)` : `Cancel (${modKey}+Shift+C)`}
            >
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
              hasImage={hasImage}
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

