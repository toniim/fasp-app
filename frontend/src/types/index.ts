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

export type AfterUploadAction = 'none' | 'direct' | 'site' | 'image';

export interface Settings {
  default_save_path?: string;
  default_format?: string;
  default_quality?: number;
  hotkeys?: Record<string, string>;
  upload_providers?: Record<string, UploadProvider>;
  active_provider?: string;
  run_at_startup?: boolean;
  after_upload_action?: AfterUploadAction;
  server_url?: string;
  api_key?: string;
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

export type AnnotationTool = 'select' | 'rectangle' | 'arrow' | 'numbered-arrow' | 'arrow-text' | 'text' | 'highlight' | 'blur' | 'crop';

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
  rotation?: number; // degrees, applied to box-based annotations
  // For curved arrows
  curvature?: number; // 0 = straight, positive = curve right, negative = curve left
  // For numbered arrows
  number?: number; // The number to display in the circle
}

export interface EditorState {
  image: string | null;
  annotations: Annotation[];
  selectedTool: AnnotationTool;
  selectedAnnotationId: string | null;
  selectedColor: string;
  selectedSize: number; // Stroke width for shapes, font size for text
  cropRegion: CropRegion | null;
  history: Annotation[][];
  historyStep: number;
  imageHistory: string[];
  imageHistoryStep: number;
}

