import React, { useState, useEffect } from 'react';
import { uploadService } from '../../services/upload-service';
import { useUpload } from '../../hooks/use-upload';
import { GetSettings } from '../../../wailsjs/go/main/App';
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime';
import { AfterUploadAction } from '../../types';

interface UploadButtonProps {
  imageData: string; // Base64 image data
  onComplete?: (publicUrl: string, directUrl: string) => void;
  onError?: (error: string) => void;
}

export const UploadButton: React.FC<UploadButtonProps> = ({
  imageData,
  onComplete,
  onError,
}) => {
  const { isUploading, progress, error, result, upload } = useUpload();
  const [isConfigured, setIsConfigured] = useState(false);
  const [afterUploadAction, setAfterUploadAction] = useState<AfterUploadAction>('direct');

  useEffect(() => {
    // Check if upload is configured
    uploadService.isConfigured().then(setIsConfigured);

    // Load after upload action setting
    GetSettings().then(settings => {
      if (settings.after_upload_action) {
        setAfterUploadAction(settings.after_upload_action as AfterUploadAction);
      }
    });
  }, []);

  useEffect(() => {
    if (result) {
      onComplete?.(result.public_url, result.direct_url);
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

  const handleAfterUploadAction = async (publicUrl: string, directUrl: string) => {
    switch (afterUploadAction) {
      case 'direct':
        await ClipboardSetText(directUrl);
        break;
      case 'site':
        await ClipboardSetText(publicUrl);
        break;
      case 'image':
        await ClipboardSetText(`![](${directUrl})`);
        break;
      case 'none':
      default:
        break;
    }
  };

  const handleUpload = async () => {
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
      await upload(blob, filename, 'image/png');
    } catch (err) {
      console.error('Upload error:', err);
      onError?.(err instanceof Error ? err.message : 'Upload failed');
    }
  };

  if (!isConfigured) {
    return null; // Don't show upload button if not configured
  }

  return (
    <button
      onClick={handleUpload}
      disabled={isUploading || !imageData}
      style={{
        padding: '8px 16px',
        background: isUploading ? '#666' : '#0066ff',
        color: 'white',
        border: 'none',
        borderRadius: '4px',
        cursor: isUploading ? 'not-allowed' : 'pointer',
        fontSize: '14px',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
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
  );
};
