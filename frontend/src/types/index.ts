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
  record_system_audio?: boolean;
  record_microphone?: boolean;
  record_mic_device?: string;
  record_fps?: number;
  // Editor tool settings kept across restarts (see utils/editor-prefs.ts).
  editor_prefs?: Record<string, unknown>;
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

export type AnnotationTool =
  | 'select' | 'rectangle' | 'ellipse' | 'line' | 'pen' | 'marker'
  | 'arrow' | 'numbered-arrow' | 'arrow-text' | 'text' | 'highlight' | 'blur' | 'crop';

// How a blur box hides what is under it. secure: an opaque pattern, nothing
// of the picture is left. blur / pixelate keep the look of the content.
export type BlurMode = 'secure' | 'blur' | 'pixelate';

export interface Annotation {
  id: string;
  type: AnnotationTool;
  x: number;
  y: number;
  width?: number;
  height?: number;
  // Blur boxes only; absent means secure.
  blurMode?: BlurMode;
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
  // For highlights and rectangles: corner radius in screen px at creation
  // (times unit gives doc px), so the rounding keeps its proportion to the
  // image. Absent means 0.
  cornerRadius?: number;
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

// A background picture behind the shot with a margin around it, for sharing.
// Part of the exported picture; a setting like colour and size (kept across
// shots, not part of undo).
// Shape of the exported picture with a backdrop: auto = the shot plus its
// padding, else that ratio (width:height) with the shot centred.
export type BackdropRatio = 'auto' | '16:9' | '1:1' | '4:5' | '3:2';

export interface BackdropSettings {
  enabled: boolean;
  presetId: string;
  ratio: BackdropRatio;
  padding: number; // margin, % of the shot's longer side
  radius: number; // corner radius of the shot (px of a 1200 px shot)
  shadow: boolean;
  // The user's own background image as a data URL, used when presetId is
  // 'custom'. Kept for the session only.
  customImage?: string;
}

export interface HighlightOptions {
  dim: number;
  radius: number;
}

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
  // Annotations selected together (marquee, Select all). Not part of undo.
  multiSelectedIds: string[];
  // Selection is not part of undo. An image and an annotation are never
  // selected together.
  selectedImageId: string | null;
  selectedColor: string;
  selectedSize: number; // Stroke width for shapes, font size for text
  // Settings of the highlight tool. Like colour and size they are tool
  // settings, not part of undo. dim (0..1) is how dark everything outside the
  // highlight boxes gets; one shade covers the picture, so it applies to every
  // box. radius is the corner radius new boxes get (screen px).
  highlightOptions: HighlightOptions;
  // Corner radius new rectangles get (screen px); a tool setting like above.
  rectangleRadius: number;
  // Mode new blur boxes get; a tool setting like above.
  blurMode: BlurMode;
  backdrop: BackdropSettings;
  cropRegion: CropRegion | null; // doc px
  // An image just added by Add shot that is waiting for its crop. Not part of
  // undo. step is the history step that added it, so Cancel can undo exactly
  // that add.
  pendingCrop: { imageId: string; step: number } | null;
  history: DocSnapshot[];
  historyStep: number;
}

