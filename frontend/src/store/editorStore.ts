import { create } from 'zustand';
import { Annotation, AnnotationTool, ArrangeMode, BackdropSettings, CanvasImage, DocSnapshot, EditorState, CropRegion, Frame, HighlightOptions } from '../types';
import { DEFAULT_BACKDROP } from '../utils/backdrops';
import { fontForSize, strokeForSize } from '../utils/annotation-sizes';
import { IMAGE_GAP, autoFrame, boundsOf, growFrame } from '../utils/editor-geometry';
import { arrange } from '../utils/image-layout';

export type ImageBox = Partial<Pick<CanvasImage, 'x' | 'y' | 'width' | 'height'>>;

export interface AddImageOptions {
  at?: { x: number; y: number };
  pendingCrop?: boolean;
}

interface EditorStore extends EditorState {
  // Starts a new shot from a PNG. Resolves once the image is decoded and the
  // document is set; rejects when the data is not a decodable image. tool is
  // selected for the new shot (default crop).
  setImage: (image: string, tool?: AnnotationTool) => Promise<void>;
  // Adds a PNG to the open shot, to the right of the current content. Resolves
  // once decoded; rejects when the data is not a decodable image. A shot that
  // was replaced while decoding drops the image silently (resolves null).
  // at: top-left in doc px instead of the default placement. pendingCrop: open
  // the image in the crop tool, selected, and wait for Enter / Escape / Cancel.
  // Resolves with the placed image.
  addImage: (image: string, options?: AddImageOptions) => Promise<CanvasImage | null>;
  // Add shot: Escape. The image stays as captured and the crop tool is left.
  keepPendingCrop: () => void;
  // Add shot: Cancel. Undoes the add (and drops its redo) so no step is left.
  cancelPendingImage: () => void;
  // Commits a finished move/resize of one image (one undo step).
  updateImage: (id: string, box: ImageBox) => void;
  removeImage: (id: string) => void;
  bringForward: (id: string) => void;
  sendBackward: (id: string) => void;
  setBackground: (color: string) => void;
  // Row / Column / Grid layout of all images: one undo step, frame back to auto.
  arrangeImages: (mode: ArrangeMode) => void;
  // Sets the gap and re-arranges with the last used mode (row if none yet) in
  // one undo step, so the control always shows an effect.
  setImageGap: (gap: number) => void;
  setSelectedImage: (id: string | null) => void;
  setSelectedTool: (tool: AnnotationTool) => void;
  setSelectedColor: (color: string) => void;
  setMultiSelectedIds: (ids: string[]) => void;
  setSelectedSize: (size: number) => void;
  setHighlightOptions: (options: Partial<HighlightOptions>) => void;
  setRectangleRadius: (radius: number) => void;
  setBackdrop: (settings: Partial<BackdropSettings>) => void;
  setCropRegion: (region: CropRegion | null) => void;
  applyCrop: () => void;
  addAnnotation: (annotation: Annotation) => void;
  updateAnnotation: (id: string, updates: Partial<Annotation>) => void;
  deleteAnnotation: (id: string) => void;
  deleteAnnotations: (ids: string[]) => void;
  setSelectedAnnotation: (id: string | null) => void;
  clearAnnotations: () => void;
  undo: () => void;
  redo: () => void;
  reset: () => void;
}

const emptyFrame: Frame = { x: 0, y: 0, width: 0, height: 0 };
const DEFAULT_BACKGROUND = '#ffffff';

const initialState: EditorState = {
  images: [],
  frame: emptyFrame,
  frameMode: 'auto',
  background: DEFAULT_BACKGROUND,
  imageGap: IMAGE_GAP,
  lastArrangeMode: null,
  annotations: [],
  selectedTool: 'crop', // Default to crop tool after screenshot
  selectedAnnotationId: null,
  multiSelectedIds: [],
  selectedImageId: null,
  selectedColor: '#ff0000',
  selectedSize: 2, // Default stroke width / font size multiplier
  highlightOptions: { dim: 0.5, radius: 8 },
  rectangleRadius: 0,
  backdrop: DEFAULT_BACKDROP,
  cropRegion: null,
  pendingCrop: null,
  history: [
    { images: [], annotations: [], frame: emptyFrame, frameMode: 'auto', background: DEFAULT_BACKGROUND, imageGap: IMAGE_GAP },
  ],
  historyStep: 0,
};

// The fields that carry an annotation's colour, set to color; null when the
// annotation has no colour or already has this one.
const colorUpdates = (ann: Annotation, color: string): Partial<Annotation> | null => {
  switch (ann.type) {
    case 'rectangle':
    case 'arrow':
    case 'arrow-text':
      return ann.stroke === color ? null : { stroke: color };
    case 'numbered-arrow':
      return ann.stroke === color && ann.fill === color ? null : { stroke: color, fill: color };
    case 'text':
      return ann.fill === color ? null : { fill: color };
    default:
      return null;
  }
};

// The fields that carry an annotation's size for Size step size, in doc px
// through the annotation's creation unit; null when it has no size or already
// has this one. A text box grows or shrinks with its font so the text keeps
// fitting.
const sizeUpdates = (ann: Annotation, size: number): Partial<Annotation> | null => {
  const unit = ann.unit ?? 1;
  switch (ann.type) {
    case 'rectangle':
    case 'arrow':
    case 'arrow-text':
    case 'numbered-arrow': {
      const strokeWidth = strokeForSize(size) * unit;
      return ann.strokeWidth === strokeWidth ? null : { strokeWidth };
    }
    case 'text': {
      const fontSize = fontForSize(size) * unit;
      const old = ann.fontSize || 16 * unit;
      if (old === fontSize) return null;
      return ann.height ? { fontSize, height: (ann.height * fontSize) / old } : { fontSize };
    }
    default:
      return null;
  }
};

// True once at least one image is in the document.
export const selectHasContent = (state: EditorState): boolean => state.images.length > 0;

let imageSeq = 0;
// Only the latest setImage call may apply its result: a slower earlier decode
// must not replace a newer shot.
let latestSetImage = 0;
// Bumped whenever the document is replaced; an addImage that finishes decoding
// after that belongs to the old shot and is dropped.
let documentEpoch = 0;
// History step created by the last setBackground, while nothing else has been
// recorded since: further background changes edit that step instead of adding
// one per color-picker movement.
let backgroundStep = -1;
// Same for colour and size edits of the selection: dragging the colour picker
// or the Size slider is one undo step. Reset when the selection changes.
let styleStep: { kind: string; step: number } | null = null;

const snapshotOf = (
  state: Pick<EditorState, 'images' | 'annotations' | 'frame' | 'frameMode' | 'background' | 'imageGap'>,
): DocSnapshot => ({
  images: state.images,
  annotations: state.annotations,
  frame: state.frame,
  frameMode: state.frameMode,
  background: state.background,
  imageGap: state.imageGap,
});

// Decodes a base64 PNG for its natural size.
const decodeImage = (base64: string): Promise<{ width: number; height: number }> =>
  new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      if (img.naturalWidth === 0 || img.naturalHeight === 0) {
        reject(new Error('Image has no pixels'));
        return;
      }
      resolve({ width: img.naturalWidth, height: img.naturalHeight });
    };
    img.onerror = () => reject(new Error('Failed to decode image'));
    img.src = `data:image/png;base64,${base64}`;
  });

export const useEditorStore = create<EditorStore>((set, get) => {
  // Applies updates (null = leave as is) to every selected annotation as one
  // undo step. Repeated edits of the same kind on the same selection (a
  // slider or colour-picker drag) stay in that step.
  const editSelection = (kind: string, updatesFor: (ann: Annotation) => Partial<Annotation> | null) => {
    const { annotations, selectedAnnotationId, multiSelectedIds } = get();
    const ids = new Set(multiSelectedIds);
    if (selectedAnnotationId) ids.add(selectedAnnotationId);
    if (ids.size === 0) return;
    let changed = false;
    const next = annotations.map((ann) => {
      if (!ids.has(ann.id)) return ann;
      const updates = updatesFor(ann);
      if (!updates) return ann;
      changed = true;
      return { ...ann, ...updates };
    });
    if (!changed) return;
    const state = get();
    if (styleStep && styleStep.kind === kind && styleStep.step === state.historyStep && styleStep.step === state.history.length - 1) {
      const history = state.history.slice();
      history[styleStep.step] = { ...history[styleStep.step], annotations: next };
      set({ annotations: next, history });
      return;
    }
    pushHistory({ annotations: next });
    styleStep = { kind, step: get().historyStep };
  };

  // Records the current document as a new undo step, dropping any redo tail.
  const pushHistory = (next: Partial<EditorState>) => {
    backgroundStep = -1;
    styleStep = null;
    const state = get();
    const merged = { ...snapshotOf(state), ...next };
    const history = state.history.slice(0, state.historyStep + 1);
    history.push(snapshotOf(merged));
    // Any recorded change ends a pending crop (the image stays as it is);
    // addImage re-sets it through `next`.
    set({ pendingCrop: null, ...next, history, historyStep: history.length - 1 });
  };

  // New image list plus the frame that goes with it: auto mode follows the
  // images, manual mode keeps the frame.
  const withImages = (images: CanvasImage[]): Partial<EditorState> => {
    const { frameMode, frame } = get();
    return { images, frame: frameMode === 'auto' ? autoFrame(images) : frame };
  };

  // State patch for a layout: moved images, auto frame around them.
  const arranged = (images: CanvasImage[], mode: ArrangeMode, gap: number): Partial<EditorState> => {
    // The layout works on the visible source size: the crop, else the full image.
    const boxes = arrange(
      images.map((im) => ({
        ...im,
        naturalWidth: im.crop?.width ?? im.naturalWidth,
        naturalHeight: im.crop?.height ?? im.naturalHeight,
      })),
      mode,
      gap,
    );
    const next = images.map((im, i) => ({ ...im, ...boxes[i], id: im.id }));
    return { images: next, frame: autoFrame(next), frameMode: 'auto', lastArrangeMode: mode };
  };

  const restore = (step: number) => {
    backgroundStep = -1;
    styleStep = null;
    const snap = get().history[step];
    const { selectedImageId, pendingCrop } = get();
    set({
      pendingCrop: pendingCrop && snap.images.some((im) => im.id === pendingCrop.imageId) ? pendingCrop : null,
      images: snap.images,
      annotations: snap.annotations,
      frame: snap.frame,
      frameMode: snap.frameMode,
      background: snap.background,
      imageGap: snap.imageGap,
      selectedImageId:
        snap.images.length > 1 && snap.images.some((im) => im.id === selectedImageId) ? selectedImageId : null,
      historyStep: step,
    });
  };

  return {
    ...initialState,

    // tool is the tool selected for the new shot: crop after a full-screen
    // capture, select after a zone capture (the zone is already the crop).
    setImage: async (image: string, tool: AnnotationTool = 'crop') => {
      const token = ++latestSetImage;
      let size: { width: number; height: number };
      try {
        size = await decodeImage(image);
      } catch (err) {
        // A slower earlier shot that fails must not surface over a newer one.
        if (token !== latestSetImage) return;
        throw err;
      }
      if (token !== latestSetImage) return;

      documentEpoch++;
      backgroundStep = -1;
      styleStep = null;
      const canvasImage: CanvasImage = {
        id: `image-${++imageSeq}`,
        src: image,
        naturalWidth: size.width,
        naturalHeight: size.height,
        x: 0,
        y: 0,
        width: size.width,
        height: size.height,
      };
      const frame = autoFrame([canvasImage]);
      set({
        images: [canvasImage],
        frame,
        frameMode: 'auto',
        background: DEFAULT_BACKGROUND,
        imageGap: IMAGE_GAP,
        lastArrangeMode: null,
        annotations: [],
        selectedAnnotationId: null,
        multiSelectedIds: [],
        selectedImageId: null,
        selectedTool: tool,
        cropRegion: null,
        pendingCrop: null,
        history: [
          {
            images: [canvasImage],
            annotations: [],
            frame,
            frameMode: 'auto',
            background: DEFAULT_BACKGROUND,
            imageGap: IMAGE_GAP,
          },
        ],
        historyStep: 0,
      });
    },

    addImage: async (image: string, options?: AddImageOptions) => {
      const epoch = documentEpoch;
      const size = await decodeImage(image);
      if (epoch !== documentEpoch) return null;

      const { images, frameMode, frame } = get();
      const content = boundsOf(images);
      const at = options?.at;
      const added: CanvasImage = {
        id: `image-${++imageSeq}`,
        src: image,
        naturalWidth: size.width,
        naturalHeight: size.height,
        // Given position, else right of the current content, top aligned.
        x: at ? Math.round(at.x) : content ? Math.round(content.x + content.width + IMAGE_GAP) : 0,
        y: at ? Math.round(at.y) : content ? Math.round(content.y) : 0,
        width: size.width,
        height: size.height,
      };
      const next = [...images, added];
      const selectable = next.length > 1;
      const pending = !!options?.pendingCrop && selectable;
      pushHistory({
        images: next,
        frame: frameMode === 'auto' ? autoFrame(next) : growFrame(frame, added, next.length),
        selectedTool: pending ? 'crop' : 'select',
        selectedAnnotationId: null,
        selectedImageId: selectable ? added.id : null,
        cropRegion: null,
        // The index pushHistory is about to give the step that holds this image.
        pendingCrop: pending ? { imageId: added.id, step: get().historyStep + 1 } : null,
      });
      return added;
    },

    keepPendingCrop: () => {
      if (!get().pendingCrop) return;
      set({ pendingCrop: null, cropRegion: null, selectedTool: 'select' });
    },

    cancelPendingImage: () => {
      const { pendingCrop, historyStep, history } = get();
      if (!pendingCrop) return;
      if (historyStep === pendingCrop.step && pendingCrop.step > 0) {
        // Back to the step before the add and forget the add, so redo cannot
        // bring the image back and no extra step is left behind.
        restore(pendingCrop.step - 1);
        set({ history: history.slice(0, pendingCrop.step), cropRegion: null, selectedTool: 'select', pendingCrop: null });
        return;
      }
      // The document moved on; the add can no longer be undone on its own.
      set({ pendingCrop: null, cropRegion: null, selectedTool: 'select' });
    },

    updateImage: (id: string, box: ImageBox) => {
      const { images } = get();
      const current = images.find((im) => im.id === id);
      if (!current) return;
      // Whole doc px keep the image on the pixel grid (no resampling at 1:1).
      const updated: CanvasImage = {
        ...current,
        x: Math.round(box.x ?? current.x),
        y: Math.round(box.y ?? current.y),
        width: Math.max(1, Math.round(box.width ?? current.width)),
        height: Math.max(1, Math.round(box.height ?? current.height)),
      };
      if (
        updated.x === current.x &&
        updated.y === current.y &&
        updated.width === current.width &&
        updated.height === current.height
      ) {
        return; // a click without movement is not an undo step
      }
      pushHistory(withImages(images.map((im) => (im.id === id ? updated : im))));
    },

    // The last image stays: an empty document has nothing to show or export.
    removeImage: (id: string) => {
      const { images } = get();
      if (images.length <= 1 || !images.some((im) => im.id === id)) return;
      const next = images.filter((im) => im.id !== id);
      pushHistory({
        ...withImages(next),
        selectedImageId: null,
      });
    },

    // Z-order is the array order; the last image is drawn on top.
    bringForward: (id: string) => {
      const { images } = get();
      const i = images.findIndex((im) => im.id === id);
      if (i < 0 || i === images.length - 1) return;
      const next = images.slice();
      [next[i], next[i + 1]] = [next[i + 1], next[i]];
      pushHistory({ images: next });
    },

    sendBackward: (id: string) => {
      const { images } = get();
      const i = images.findIndex((im) => im.id === id);
      if (i <= 0) return;
      const next = images.slice();
      [next[i], next[i - 1]] = [next[i - 1], next[i]];
      pushHistory({ images: next });
    },

    setBackground: (color: string) => {
      const state = get();
      if (color === state.background) return;
      if (backgroundStep === state.historyStep && backgroundStep === state.history.length - 1) {
        const history = state.history.slice();
        history[backgroundStep] = { ...history[backgroundStep], background: color };
        set({ background: color, history });
        return;
      }
      pushHistory({ background: color });
      backgroundStep = get().historyStep;
    },

    // Annotations are not attached to images and stay where they are.
    arrangeImages: (mode: ArrangeMode) => {
      const { images, imageGap } = get();
      if (images.length < 2) return;
      pushHistory(arranged(images, mode, imageGap));
    },

    setImageGap: (gap: number) => {
      const { images, imageGap, lastArrangeMode } = get();
      if (images.length < 2 || gap === imageGap) return;
      pushHistory({ ...arranged(images, lastArrangeMode ?? 'row', gap), imageGap: gap });
    },

    setSelectedImage: (id: string | null) => {
      set(id ? { selectedImageId: id, selectedAnnotationId: null } : { selectedImageId: null });
    },

    setSelectedTool: (tool: AnnotationTool) => {
      // The crop tool keeps a selected image: that image is the crop target.
      // Leaving the crop tool ends a pending crop (the image stays as captured).
      set((state) => ({
        selectedTool: tool,
        selectedAnnotationId: null,
        selectedImageId: tool === 'crop' ? state.selectedImageId : null,
        pendingCrop: tool === 'crop' ? state.pendingCrop : null,
      }));
    },

    // Sets the colour for new annotations and recolours the selected ones.
    // Highlight and blur have no colour.
    setSelectedColor: (color: string) => {
      set({ selectedColor: color });
      editSelection('color', (ann) => colorUpdates(ann, color));
    },

    setMultiSelectedIds: (ids: string[]) => {
      styleStep = null;
      set({ multiSelectedIds: ids });
    },

    // Sets the size for new annotations and resizes the selected ones: the
    // stroke of shapes and arrows, the font of text. Highlight and blur have
    // no size.
    setSelectedSize: (size: number) => {
      set({ selectedSize: size });
      editSelection('size', (ann) => sizeUpdates(ann, size));
    },

    setHighlightOptions: (options: Partial<HighlightOptions>) => {
      set((state) => ({ highlightOptions: { ...state.highlightOptions, ...options } }));
    },

    setRectangleRadius: (radius: number) => {
      set({ rectangleRadius: radius });
    },

    setBackdrop: (settings: Partial<BackdropSettings>) => {
      set((state) => ({ backdrop: { ...state.backdrop, ...settings } }));
    },

    setCropRegion: (region: CropRegion | null) => {
      set({ cropRegion: region });
    },

    // With an image selected (two or more images) the crop tool crops that
    // image. Otherwise it is the canvas crop: the region is already in doc px,
    // so only the frame moves. Pixels are not re-encoded (the layer clips to the
    // frame) and annotations are kept as they are; the frame clips them on
    // screen and in the export. The frame is fixed from now on (manual mode).
    applyCrop: () => {
      const { cropRegion, frame, images, selectedImageId, selectedTool } = get();
      if (!cropRegion) return;

      // Normalize crop region (handle negative width/height)
      const x1 = Math.round(cropRegion.width < 0 ? cropRegion.x + cropRegion.width : cropRegion.x);
      const y1 = Math.round(cropRegion.height < 0 ? cropRegion.y + cropRegion.height : cropRegion.y);
      const x2 = Math.round(cropRegion.width < 0 ? cropRegion.x : cropRegion.x + cropRegion.width);
      const y2 = Math.round(cropRegion.height < 0 ? cropRegion.y : cropRegion.y + cropRegion.height);

      const target =
        selectedTool === 'crop' && images.length > 1 ? images.find((im) => im.id === selectedImageId) : undefined;
      if (target) {
        // Visible part of the source now, and doc px per source px of the box.
        const vis = target.crop ?? { x: 0, y: 0, width: target.naturalWidth, height: target.naturalHeight };
        const sx = target.width / vis.width;
        const sy = target.height / vis.height;

        // Clamp the region to the image's box.
        const left = Math.max(target.x, x1);
        const top = Math.max(target.y, y1);
        const right = Math.min(target.x + target.width, x2);
        const bottom = Math.min(target.y + target.height, y2);
        if (right - left < 1 || bottom - top < 1) {
          set({ cropRegion: null });
          return;
        }

        // Source px, whole numbers, inside the visible part.
        const cx = Math.min(vis.x + vis.width - 1, vis.x + Math.round((left - target.x) / sx));
        const cy = Math.min(vis.y + vis.height - 1, vis.y + Math.round((top - target.y) / sy));
        const cw = Math.max(1, Math.min(vis.x + vis.width, vis.x + Math.round((right - target.x) / sx)) - cx);
        const ch = Math.max(1, Math.min(vis.y + vis.height, vis.y + Math.round((bottom - target.y) / sy)) - cy);

        // The box keeps the doc-px size of the kept part and sits where those
        // pixels already were.
        const box = {
          x: Math.round(target.x + (cx - vis.x) * sx),
          y: Math.round(target.y + (cy - vis.y) * sy),
          width: Math.max(1, Math.round(cw * sx)),
          height: Math.max(1, Math.round(ch * sy)),
        };
        const unchanged =
          cx === vis.x && cy === vis.y && cw === vis.width && ch === vis.height;
        if (unchanged) {
          // Nothing cut off: no history step, just leave the crop.
          set({ cropRegion: null, selectedTool: 'select', pendingCrop: null });
          return;
        }

        pushHistory({
          ...withImages(
            images.map((im) => (im.id === target.id ? { ...im, ...box, crop: { x: cx, y: cy, width: cw, height: ch } } : im)),
          ),
          cropRegion: null,
          selectedTool: 'select',
        });
        return;
      }

      // The new frame may only shrink: intersect with the current one.
      const left = Math.max(frame.x, x1);
      const top = Math.max(frame.y, y1);
      const right = Math.min(frame.x + frame.width, x2);
      const bottom = Math.min(frame.y + frame.height, y2);

      if (right - left < 1 || bottom - top < 1) {
        // Nothing of the frame is inside the region: drop it instead of
        // producing an empty document.
        set({ cropRegion: null });
        return;
      }

      pushHistory({
        frame: { x: left, y: top, width: right - left, height: bottom - top },
        frameMode: 'manual',
        cropRegion: null,
        selectedTool: 'select',
        selectedImageId: null,
      });
    },

    addAnnotation: (annotation: Annotation) => {
      pushHistory({ annotations: [...get().annotations, annotation] });
    },

    // Does not push history: moves and edits of an existing annotation share
    // the step that created it.
    updateAnnotation: (id: string, updates: Partial<Annotation>) => {
      const { annotations } = get();
      const newAnnotations = annotations.map((ann) =>
        ann.id === id ? { ...ann, ...updates } : ann
      );
      set({ annotations: newAnnotations });
    },

    deleteAnnotation: (id: string) => {
      pushHistory({
        annotations: get().annotations.filter((ann) => ann.id !== id),
        selectedAnnotationId: null,
      });
    },

    deleteAnnotations: (ids: string[]) => {
      if (ids.length === 0) return;
      const idSet = new Set(ids);
      pushHistory({
        annotations: get().annotations.filter((ann) => !idSet.has(ann.id)),
        selectedAnnotationId: null,
      });
    },

    setSelectedAnnotation: (id: string | null) => {
      styleStep = null;
      set(id ? { selectedAnnotationId: id, selectedImageId: null } : { selectedAnnotationId: null });
    },

    clearAnnotations: () => {
      backgroundStep = -1;
      styleStep = null;
      const state = get();
      set({
        annotations: [],
        history: [snapshotOf({ ...state, annotations: [] })],
        historyStep: 0,
      });
    },

    undo: () => {
      const { historyStep } = get();
      if (historyStep > 0) restore(historyStep - 1);
    },

    redo: () => {
      const { history, historyStep } = get();
      if (historyStep < history.length - 1) restore(historyStep + 1);
    },

    reset: () => {
      documentEpoch++;
      backgroundStep = -1;
      styleStep = null;
      set({ ...initialState });
    },
  };
});
