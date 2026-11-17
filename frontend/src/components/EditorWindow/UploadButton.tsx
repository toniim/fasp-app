import React, { useState, useEffect, useRef } from 'react';
import { uploadService, Visibility } from '../../services/upload-service';
import { useUpload } from '../../hooks/use-upload';
import { GetSettings } from '../../../bindings/github.com/heytonyne/fasp/app';
import { Clipboard } from '@wailsio/runtime';
import { AfterUploadAction } from '../../types';

export interface UploadFile {
  blob: Blob;
  filename: string;
  contentType: string;
}

interface UploadButtonProps {
  // Lazily produces the PNG data URL (image + latest annotations) at click time.
  getImageData?: () => string;
  // Or any file, e.g. a recording; takes precedence over getImageData.
  getFile?: () => Promise<UploadFile>;
  hasImage: boolean;
  // isPublic tells whether the file was uploaded as public or private.
  onComplete?: (publicUrl: string, directUrl: string, isPublic: boolean) => void;
  onError?: (error: string) => void;
}

const buttonBase: React.CSSProperties = {
  padding: '8px 16px',
  color: 'white',
  border: 'none',
  fontSize: '14px',
  display: 'flex',
  alignItems: 'center',
  gap: '8px',
};

export const UploadButton: React.FC<UploadButtonProps> = ({
  getImageData,
  getFile,
  hasImage,
  onComplete,
  onError,
}) => {
  const { isUploading, progress, error, result, upload } = useUpload();
  const [isConfigured, setIsConfigured] = useState(false);
  const [afterUploadAction, setAfterUploadAction] = useState<AfterUploadAction>('direct');
  // The "Upload as public" setting: what the main button does.
  const [defaultPublic, setDefaultPublic] = useState(true);
  const [menuOpen, setMenuOpen] = useState(false);
  // Whether the upload in flight is public; read when it completes.
  const usedPublic = useRef(true);
  const root = useRef<HTMLDivElement>(null);

  // Settings can change while the editor is open, so they are read again when
  // the menu opens and when an upload starts.
  const refreshSettings = async (): Promise<boolean> => {
    try {
      const settings = await GetSettings();
      if (settings?.after_upload_action) {
        setAfterUploadAction(settings.after_upload_action as AfterUploadAction);
      }
      const pub = settings?.upload_public ?? true;
      setDefaultPublic(pub);
      return pub;
    } catch (err) {
      console.error('Failed to read upload settings:', err);
      return defaultPublic;
    }
  };

  useEffect(() => {
    // Check if upload is configured
    uploadService.isConfigured().then(setIsConfigured);
    refreshSettings();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (result) {
      onComplete?.(result.public_url, result.direct_url, usedPublic.current);
      handleAfterUploadAction(result.public_url, result.direct_url);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result]);

  useEffect(() => {
    if (error) {
      onError?.(error);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [error]);

  // Close the menu on a click outside it or on Esc.
  useEffect(() => {
    if (!menuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpen(false);
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuOpen]);

  const handleAfterUploadAction = async (publicUrl: string, directUrl: string) => {
    switch (afterUploadAction) {
      case 'direct':
        await Clipboard.SetText(directUrl);
        break;
      case 'site':
        await Clipboard.SetText(publicUrl);
        break;
      case 'image':
        // Markdown image syntax only makes sense for images.
        await Clipboard.SetText(getFile ? directUrl : `![](${directUrl})`);
        break;
      case 'none':
      default:
        break;
    }
  };

  // visibility '' follows the setting.
  const handleUpload = async (visibility: Visibility = '') => {
    setMenuOpen(false);
    const settingPublic = await refreshSettings();
    usedPublic.current = visibility === '' ? settingPublic : visibility === 'public';

    if (getFile) {
      try {
        const f = await getFile();
        await upload(f.blob, f.filename, f.contentType, visibility);
      } catch (err) {
        console.error('Upload error:', err);
        onError?.(err instanceof Error ? err.message : 'Upload failed');
      }
      return;
    }
    if (!getImageData) return;
    // Export now so the upload includes any annotations added since load.
    const imageData = getImageData();
    if (!imageData) {
      onError?.('No image data to upload');
      return;
    }

    try {
      // Convert base64 to blob
      const base64Data = imageData.replace(/^data:image\/\w+;base64,/, '');
      const binaryString = window.atob(base64Data);
      const bytes = new Uint8Array(binaryString.length);
      for (let i = 0; i < binaryString.length; i++) {
        bytes[i] = binaryString.charCodeAt(i);
      }
      const blob = new Blob([bytes], { type: 'image/png' });

      // Generate filename
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const filename = `screenshot-${timestamp}.png`;

      // Upload
      await upload(blob, filename, 'image/png', visibility);
    } catch (err) {
      console.error('Upload error:', err);
      onError?.(err instanceof Error ? err.message : 'Upload failed');
    }
  };

  if (!isConfigured) {
    return null; // Don't show upload button if not configured
  }

  const disabled = isUploading || !hasImage;
  const background = isUploading ? '#666' : '#0066ff';
  const cursor = disabled ? 'not-allowed' : 'pointer';

  const menuItem = (label: string, hint: string, visibility: 'public' | 'private', isDefault: boolean) => (
    <button
      role="menuitem"
      onClick={() => handleUpload(visibility)}
      style={{
        display: 'block',
        width: '100%',
        textAlign: 'left',
        padding: '8px 14px',
        background: 'transparent',
        color: 'white',
        border: 'none',
        cursor: 'pointer',
        fontSize: '13px',
      }}
    >
      <div>{label}{isDefault ? ' (default)' : ''}</div>
      <div style={{ opacity: 0.6, fontSize: '11px' }}>{hint}</div>
    </button>
  );

  return (
    <div ref={root} style={{ position: 'relative', display: 'flex' }}>
      <button
        onClick={() => handleUpload('')}
        disabled={disabled}
        title={defaultPublic ? 'Upload (public link)' : 'Upload (private: only you can open it)'}
        style={{
          ...buttonBase,
          background,
          cursor,
          borderRadius: '4px 0 0 4px',
        }}
      >
        {isUploading ? (
          <>
            <span>Uploading... {progress}%</span>
          </>
        ) : (
          <>
            <span>☁️</span>
            <span>Upload</span>
          </>
        )}
      </button>
      <button
        onClick={() => {
          if (!menuOpen) refreshSettings();
          setMenuOpen(!menuOpen);
        }}
        disabled={disabled}
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        title="Upload as public or private"
        style={{
          ...buttonBase,
          padding: '8px 8px',
          background,
          cursor,
          borderRadius: '0 4px 4px 0',
          borderLeft: '1px solid rgba(255,255,255,0.35)',
        }}
      >
        <span>▾</span>
      </button>
      {menuOpen && (
        <div
          role="menu"
          style={{
            position: 'absolute',
            bottom: '100%',
            right: 0,
            marginBottom: '6px',
            minWidth: '220px',
            background: '#2b2b2f',
            border: '1px solid rgba(255,255,255,0.15)',
            borderRadius: '6px',
            boxShadow: '0 6px 18px rgba(0,0,0,0.4)',
            zIndex: 50,
            overflow: 'hidden',
          }}
        >
          {menuItem('Upload as Public', 'Anyone with the link can open it', 'public', defaultPublic)}
          {menuItem('Upload as Private', 'Only you can open it', 'private', !defaultPublic)}
        </div>
      )}
    </div>
  );
};
