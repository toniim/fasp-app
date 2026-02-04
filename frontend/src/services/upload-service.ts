import { UploadInit, UploadComplete, IsUploadConfigured } from '../../wailsjs/go/main/App';

export interface InitResponse {
  file_id: string;
  upload_url: string;
}

export interface CompleteResponse {
  public_url: string;
  direct_url: string;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percentage: number;
}

class UploadService {
  /**
   * Initialize upload and get presigned upload URL
   */
  async init(filename: string, size: number, contentType: string): Promise<InitResponse> {
    try {
      const response = await UploadInit(filename, size, contentType);
      return response;
    } catch (error) {
      console.error('Failed to initialize upload:', error);
      throw error;
    }
  }

  /**
   * Upload file to presigned URL with progress tracking
   */
  async putFile(
    uploadURL: string,
    data: Blob,
    contentType: string,
    onProgress?: (progress: UploadProgress) => void
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();

      // Track upload progress
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress({
            loaded: e.loaded,
            total: e.total,
            percentage: Math.round((e.loaded / e.total) * 100)
          });
        }
      });

      // Handle completion
      xhr.addEventListener('load', () => {
        if (xhr.status === 200 || xhr.status === 204) {
          resolve();
        } else {
          reject(new Error(`Upload failed with status ${xhr.status}: ${xhr.responseText}`));
        }
      });

      // Handle errors
      xhr.addEventListener('error', () => {
        reject(new Error('Upload failed'));
      });

      xhr.addEventListener('abort', () => {
        reject(new Error('Upload aborted'));
      });

      // Send PUT request
      xhr.open('PUT', uploadURL);
      xhr.setRequestHeader('Content-Type', contentType);
      xhr.send(data);
    });
  }

  /**
   * Complete upload and get public URLs
   */
  async complete(fileID: string): Promise<CompleteResponse> {
    try {
      const response = await UploadComplete(fileID);
      return response;
    } catch (error) {
      console.error('Failed to complete upload:', error);
      throw error;
    }
  }

  /**
   * Check if upload is configured (user is authenticated and API host is set)
   */
  async isConfigured(): Promise<boolean> {
    try {
      return await IsUploadConfigured();
    } catch (error) {
      console.error('Failed to check upload configuration:', error);
      return false;
    }
  }

  /**
   * Full upload flow: init -> PUT -> complete
   */
  async upload(
    data: Blob,
    filename: string,
    contentType: string,
    onProgress?: (progress: UploadProgress) => void
  ): Promise<CompleteResponse> {
    // Step 1: Initialize upload
    const initResp = await this.init(filename, data.size, contentType);

    // Step 2: Upload file
    await this.putFile(initResp.upload_url, data, contentType, onProgress);

    // Step 3: Complete upload
    const completeResp = await this.complete(initResp.file_id);

    return completeResp;
  }
}

export const uploadService = new UploadService();
