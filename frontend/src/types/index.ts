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
  // Document pixels per screen pixel at creation time (1 / scaleRatio). Fixed
  // visual sizes owned by the annotation (arrow head, numbered circle, label
  // gap, blur tile) are multiplied by this so they keep their proportion to
  // the image when the view scale changes later. Absent means 1.
  unit?: number;
}

// All positions and sizes below are in document pixels (doc px): one doc px is
// one native pixel of an image placed at its natural size. The view scale
// (zoom * scaleRatio) maps doc px to screen px; the document never changes
// with the window size.
export interface CanvasImage {
  id: string;
  src: string; // base64 PNG, no data: prefix
  naturalWidth: number;
  naturalHeight: number;
  x: number;
  y: number;
  width: number; // displayed size in doc px (natural size by default)
  height: number;
  crop?: CropRegion; // in natural px of src
}

// The exported / visible area of the document, in doc px (integers).
export interface Frame {
  x: number;
  y: number;
  width: number;
  height: number;
}

// One undo step: everything that undo/redo restores.
export interface DocSnapshot {
  images: CanvasImage[];
  annotations: Annotation[];
  frame: Frame;
  frameMode: FrameMode;
  background: string;
  imageGap: number;
}

export type ArrangeMode = 'row' | 'column' | 'grid';

// auto: the frame is the images' bounding box plus padding and follows every
// image change. manual: a canvas crop fixed it; it only grows when an image is
// added outside it.
export type FrameMode = 'auto' | 'manual';

export interface EditorState {
  images: CanvasImage[];
  frame: Frame;
  frameMode: FrameMode;
  background: string; // CSS color drawn under the images, inside the frame
  imageGap: number; // doc px between images when arranging (part of undo)
  // Layout the gap control re-applies. Not part of undo; null until the user
  // arranges.
  lastArrangeMode: ArrangeMode | null;
  annotations: Annotation[];
  selectedTool: AnnotationTool;
  selectedAnnotationId: string | null;
  // Selection is not part of undo. An image and an annotation are never
  // selected together.
  selectedImageId: string | null;
  selectedColor: string;
  selectedSize: number; // Stroke width for shapes, font size for text
  cropRegion: CropRegion | null; // doc px
  // An image just added by Add shot that is waiting for its crop. Not part of
  // undo. step is the history step that added it, so Cancel can undo exactly
  // that add.
  pendingCrop: { imageId: string; step: number } | null;
  history: DocSnapshot[];
  historyStep: number;
}

