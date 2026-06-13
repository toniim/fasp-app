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
          console.error('[upload] R2 PUT rejected', {
            status: xhr.status,
            statusText: xhr.statusText,
            responseText: xhr.responseText,
            uploadURL,
          });
          reject(new Error(`R2 PUT failed (HTTP ${xhr.status} ${xhr.statusText}): ${xhr.responseText || 'no body'}`));
        }
      });

      // Handle errors — network/CORS failures give no status, so log what we can
      xhr.addEventListener('error', () => {
        console.error('[upload] R2 PUT network/CORS error', {
          status: xhr.status,
          statusText: xhr.statusText,
          uploadURL,
        });
        reject(new Error(
          `R2 PUT network error (likely CORS or unreachable host). status=${xhr.status} ${xhr.statusText}. URL host=${(() => { try { return new URL(uploadURL).host; } catch { return uploadURL; } })()}`
        ));
      });

      xhr.addEventListener('timeout', () => {
        console.error('[upload] R2 PUT timeout', { uploadURL });
        reject(new Error('R2 PUT timed out'));
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
    console.log('[upload] init', { filename, size: data.size, contentType });
    let initResp: InitResponse;
    try {
      initResp = await this.init(filename, data.size, contentType);
    } catch (error) {
      console.error('[upload] init step failed:', error);
      throw new Error(`Init failed: ${error instanceof Error ? error.message : String(error)}`);
    }
    console.log('[upload] init ok', { file_id: initResp.file_id });

    // Step 2: Upload file to presigned R2 URL
    try {
      await this.putFile(initResp.upload_url, data, contentType, onProgress);
    } catch (error) {
      console.error('[upload] PUT step failed:', error);
      throw error; // putFile already produces a descriptive message
    }
    console.log('[upload] PUT ok');

    // Step 3: Complete upload
    let completeResp: CompleteResponse;
    try {
      completeResp = await this.complete(initResp.file_id);
    } catch (error) {
      console.error('[upload] complete step failed:', error);
      throw new Error(`Complete failed: ${error instanceof Error ? error.message : String(error)}`);
    }
    console.log('[upload] complete ok', completeResp);

    return completeResp;
  }
}

export const uploadService = new UploadService();
