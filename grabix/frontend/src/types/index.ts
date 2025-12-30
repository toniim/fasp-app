// Types matching Go backend models

export interface DisplayInfo {
  id: number;
  name: string;
  width: number;
  height: number;
  x: number;
  y: number;
}

export interface CaptureResult {
  data: string; // Base64 encoded image
  width: number;
  height: number;
  timestamp: string;
}

export interface CropRegion {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface SaveOptions {
  path: string;
  format: string;
  quality: number;
}

export interface Settings {
  default_save_path?: string;
  default_format?: string;
  default_quality?: number;
  hotkeys?: Record<string, string>;
  upload_providers?: Record<string, UploadProvider>;
  active_provider?: string;
  defaultSaveLocation?: string; // Alias for compatibility
  defaultFormat?: string; // Alias for compatibility
}

export interface UploadProvider {
  name: string;
  enabled: boolean;
  endpoint?: string;
  headers?: Record<string, string>;
}

// Frontend-specific types

export type AnnotationTool = 'select' | 'rectangle' | 'arrow' | 'text' | 'highlight' | 'blur' | 'crop';

export interface Annotation {
  id: string;
  type: AnnotationTool;
  x: number;
  y: number;
  width?: number;
  height?: number;
  points?: number[];
  text?: string;
  fontSize?: number;
  fill?: string;
  stroke?: string;
  strokeWidth?: number;
  opacity?: number;
}

export interface EditorState {
  image: string | null;
  annotations: Annotation[];
  selectedTool: AnnotationTool;
  selectedAnnotationId: string | null;
  selectedColor: string;
  cropRegion: CropRegion | null;
  history: Annotation[][];
  historyStep: number;
  imageHistory: string[];
  imageHistoryStep: number;
}

