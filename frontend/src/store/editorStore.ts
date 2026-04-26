import { create } from 'zustand';
import { Annotation, AnnotationTool, EditorState, CropRegion } from '../types';

interface EditorStore extends EditorState {
  setImage: (image: string) => void;
  setSelectedTool: (tool: AnnotationTool) => void;
  setSelectedColor: (color: string) => void;
  setSelectedSize: (size: number) => void;
  setCropRegion: (region: CropRegion | null) => void;
  applyCrop: (scaleRatio: number) => Promise<void>;
  addAnnotation: (annotation: Annotation) => void;
  updateAnnotation: (id: string, updates: Partial<Annotation>, commit?: boolean) => void;
  deleteAnnotation: (id: string) => void;
  setSelectedAnnotation: (id: string | null) => void;
  clearAnnotations: () => void;
  undo: () => void;
  redo: () => void;
  reset: () => void;
}

const initialState: EditorState = {
  image: null,
  annotations: [],
  selectedTool: 'crop', // Default to crop tool after screenshot
  selectedAnnotationId: null,
  selectedColor: '#ff0000',
  selectedSize: 2, // Default stroke width / font size multiplier
  cropRegion: null,
  history: [[]],
  historyStep: 0,
  imageHistory: [],
  imageHistoryStep: -1,
};

export const useEditorStore = create<EditorStore>((set, get) => ({
  ...initialState,

  setImage: (image: string) => {
    const { imageHistory, imageHistoryStep } = get();
    // Add to image history
    const newHistory = imageHistory.slice(0, imageHistoryStep + 1);
    newHistory.push(image);
    set({
      image,
      annotations: [],
      selectedAnnotationId: null,
      selectedTool: 'crop', // Default to crop tool after screenshot
      cropRegion: null,
      history: [[]],
      historyStep: 0,
      imageHistory: newHistory,
      imageHistoryStep: newHistory.length - 1,
    });
  },

  setSelectedTool: (tool: AnnotationTool) => {
    set({ selectedTool: tool, selectedAnnotationId: null });
  },

  // setImage above already resets selectedTool to 'crop' so newly opened
  // images go through the cropper before annotation editing.

  setSelectedColor: (color: string) => {
    set({ selectedColor: color });
  },

  setSelectedSize: (size: number) => {
    set({ selectedSize: size });
  },

  setCropRegion: (region: CropRegion | null) => {
    set({ cropRegion: region });
  },

  applyCrop: (scaleRatio: number) => {
    const { image, cropRegion, annotations, imageHistory, imageHistoryStep } = get();
    if (!image || !cropRegion) return Promise.resolve();

    // Normalize crop region (handle negative width/height)
    const normalizedCrop = {
      x: cropRegion.width < 0 ? cropRegion.x + cropRegion.width : cropRegion.x,
      y: cropRegion.height < 0 ? cropRegion.y + cropRegion.height : cropRegion.y,
      width: Math.abs(cropRegion.width),
      height: Math.abs(cropRegion.height),
    };

    // Create canvas to crop image
    return new Promise<void>((resolve, reject) => {
    const img = new Image();
    img.src = `data:image/png;base64,${image}`;
    img.onerror = () => reject(new Error('Failed to load image for cropping'));
    img.onload = () => {
      // Convert crop region from stage coordinates to image coordinates
      const actualX = normalizedCrop.x / scaleRatio;
      const actualY = normalizedCrop.y / scaleRatio;
      const actualWidth = normalizedCrop.width / scaleRatio;
      const actualHeight = normalizedCrop.height / scaleRatio;

      const canvas = document.createElement('canvas');
      canvas.width = actualWidth;
      canvas.height = actualHeight;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      // Draw cropped portion using actual image coordinates
      ctx.drawImage(
        img,
        actualX,
        actualY,
        actualWidth,
        actualHeight,
        0,
        0,
        actualWidth,
        actualHeight
      );

      // Convert to base64
      const croppedBase64 = canvas.toDataURL('image/png').replace(/^data:image\/\w+;base64,/, '');

      // Add to image history
      const newHistory = imageHistory.slice(0, imageHistoryStep + 1);
      newHistory.push(croppedBase64);

      // Adjust annotations to new crop coordinates
      // Keep annotations that are at least partially within the crop region
      const adjustedAnnotations = annotations.map(ann => {
        const adjusted = { ...ann };

        // Adjust position-based annotations (rectangle, highlight, blur, text)
        if (adjusted.x !== undefined) {
          adjusted.x = adjusted.x - normalizedCrop.x;
        }
        if (adjusted.y !== undefined) {
          adjusted.y = adjusted.y - normalizedCrop.y;
        }

        // Adjust point-based annotations (arrow, arrow-text, numbered-arrow)
        if (adjusted.points) {
          adjusted.points = adjusted.points.map((p, i) =>
            i % 2 === 0 ? p - normalizedCrop.x : p - normalizedCrop.y
          );
        }

        return adjusted;
      }).filter(ann => {
        // Filter out annotations that are completely outside the new cropped area
        // Check based on annotation type
        if (ann.points && ann.points.length >= 4) {
          // For arrows, check if any point is within bounds
          const xs = ann.points.filter((_, i) => i % 2 === 0);
          const ys = ann.points.filter((_, i) => i % 2 === 1);
          const minX = Math.min(...xs);
          const maxX = Math.max(...xs);
          const minY = Math.min(...ys);
          const maxY = Math.max(...ys);
          return maxX >= 0 && minX <= normalizedCrop.width &&
                 maxY >= 0 && minY <= normalizedCrop.height;
        } else if (ann.x !== undefined && ann.y !== undefined) {
          // For position-based annotations
          const annRight = ann.x + (ann.width || 0);
          const annBottom = ann.y + (ann.height || 0);
          return annRight >= 0 && ann.x <= normalizedCrop.width &&
                 annBottom >= 0 && ann.y <= normalizedCrop.height;
        }
        return true;
      });

      // Update image and keep adjusted annotations
      set({
        image: croppedBase64,
        cropRegion: null,
        annotations: adjustedAnnotations,
        history: [adjustedAnnotations],
        historyStep: 0,
        selectedTool: 'select',
        imageHistory: newHistory,
        imageHistoryStep: newHistory.length - 1,
      });
      resolve();
    };
    });
  },

  addAnnotation: (annotation: Annotation) => {
    const { annotations, history, historyStep } = get();
    const newAnnotations = [...annotations, annotation];
    const newHistory = history.slice(0, historyStep + 1);
    newHistory.push(newAnnotations);

    set({
      annotations: newAnnotations,
      history: newHistory,
      historyStep: historyStep + 1,
    });
  },

  // updateAnnotation pushes a history entry by default so drag/resize
  // operations are undoable. Pass `commit=false` for high-frequency intermediate
  // updates (e.g. while a drag is still in progress) and call once more with
  // `commit=true` (or omit) on the final update.
  updateAnnotation: (id: string, updates: Partial<Annotation>, commit: boolean = true) => {
    const { annotations, history, historyStep } = get();
    const newAnnotations = annotations.map((ann) =>
      ann.id === id ? { ...ann, ...updates } : ann
    );
    if (!commit) {
      set({ annotations: newAnnotations });
      return;
    }
    const newHistory = history.slice(0, historyStep + 1);
    newHistory.push(newAnnotations);
    set({
      annotations: newAnnotations,
      history: newHistory,
      historyStep: historyStep + 1,
    });
  },

  deleteAnnotation: (id: string) => {
    const { annotations, history, historyStep } = get();
    const newAnnotations = annotations.filter((ann) => ann.id !== id);
    const newHistory = history.slice(0, historyStep + 1);
    newHistory.push(newAnnotations);

    set({
      annotations: newAnnotations,
      history: newHistory,
      historyStep: historyStep + 1,
      selectedAnnotationId: null,
    });
  },

  setSelectedAnnotation: (id: string | null) => {
    set({ selectedAnnotationId: id });
  },

  clearAnnotations: () => {
    set({ annotations: [], history: [[]], historyStep: 0 });
  },

  undo: () => {
    const { history, historyStep, imageHistory, imageHistoryStep } = get();

    // Try to undo annotation first
    if (historyStep > 0) {
      const newStep = historyStep - 1;
      set({
        annotations: history[newStep],
        historyStep: newStep,
      });
    }
    // If no annotation to undo, try to undo image (crop)
    else if (imageHistoryStep > 0) {
      const newImageStep = imageHistoryStep - 1;
      set({
        image: imageHistory[newImageStep],
        imageHistoryStep: newImageStep,
        annotations: [],
        history: [[]],
        historyStep: 0,
      });
    }
  },

  redo: () => {
    const { history, historyStep, imageHistory, imageHistoryStep } = get();

    // Try to redo annotation first
    if (historyStep < history.length - 1) {
      const newStep = historyStep + 1;
      set({
        annotations: history[newStep],
        historyStep: newStep,
      });
    }
    // If no annotation to redo, try to redo image (crop)
    else if (imageHistoryStep < imageHistory.length - 1) {
      const newImageStep = imageHistoryStep + 1;
      set({
        image: imageHistory[newImageStep],
        imageHistoryStep: newImageStep,
        annotations: [],
        history: [[]],
        historyStep: 0,
      });
    }
  },

  reset: () => {
    set({
      ...initialState,
      imageHistory: [],
      imageHistoryStep: -1,
    });
  },
}));

