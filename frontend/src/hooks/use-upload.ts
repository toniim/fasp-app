import { useState, useCallback } from 'react';
import { uploadService, UploadProgress, CompleteResponse } from '../services/upload-service';

export interface UploadState {
  isUploading: boolean;
  progress: number;
  error: string | null;
  result: CompleteResponse | null;
}

export const useUpload = () => {
  const [state, setState] = useState<UploadState>({
    isUploading: false,
    progress: 0,
    error: null,
    result: null,
  });

  const upload = useCallback(async (
    data: Blob,
    filename: string,
    contentType: string = 'image/png'
  ): Promise<CompleteResponse | null> => {
    setState({
      isUploading: true,
      progress: 0,
      error: null,
      result: null,
    });

    try {
      const result = await uploadService.upload(
        data,
        filename,
        contentType,
        (progress: UploadProgress) => {
          setState(prev => ({
            ...prev,
            progress: progress.percentage,
          }));
        }
      );

      setState({
        isUploading: false,
        progress: 100,
        error: null,
        result,
      });

      return result;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Upload failed';
      setState({
        isUploading: false,
        progress: 0,
        error: errorMessage,
        result: null,
      });
      return null;
    }
  }, []);

  const reset = useCallback(() => {
    setState({
      isUploading: false,
      progress: 0,
      error: null,
      result: null,
    });
  }, []);

  return {
    ...state,
    upload,
    reset,
  };
};
