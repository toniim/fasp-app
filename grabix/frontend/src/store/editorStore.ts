import { create } from 'zustand';
import { Annotation, AnnotationTool, EditorState, CropRegion } from '../types';

interface EditorStore extends EditorState {
  setImage: (image: string) => void;
  setSelectedTool: (tool: AnnotationTool) => void;
  setSelectedColor: (color: string) => void;
  setCropRegion: (region: CropRegion | null) => void;
  applyCrop: (scaleRatio: number) => void;
  addAnnotation: (annotation: Annotation) => void;
  updateAnnotation: (id: string, updates: Partial<Annotation>) => void;
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
  selectedTool: 'select',
  selectedAnnotationId: null,
  selectedColor: '#ff0000',
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
      history: [[]],
      historyStep: 0,
      imageHistory: newHistory,
      imageHistoryStep: newHistory.length - 1,
    });
  },

  setSelectedTool: (tool: AnnotationTool) => {
    set({ selectedTool: tool, selectedAnnotationId: null });
  },

  setSelectedColor: (color: string) => {
    set({ selectedColor: color });
  },

  setCropRegion: (region: CropRegion | null) => {
    set({ cropRegion: region });
  },

  applyCrop: (scaleRatio: number) => {
    const { image, cropRegion, imageHistory, imageHistoryStep } = get();
    if (!image || !cropRegion) return;

    // Create canvas to crop image
    const img = new Image();
    img.src = `data:image/png;base64,${image}`;
    img.onload = () => {
      // Convert crop region from stage coordinates to image coordinates
      const actualX = cropRegion.x / scaleRatio;
      const actualY = cropRegion.y / scaleRatio;
      const actualWidth = cropRegion.width / scaleRatio;
      const actualHeight = cropRegion.height / scaleRatio;

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

      // Update image and clear annotations
      set({
        image: croppedBase64,
        cropRegion: null,
        annotations: [],
        history: [[]],
        historyStep: 0,
        selectedTool: 'select',
        imageHistory: newHistory,
        imageHistoryStep: newHistory.length - 1,
      });
    };
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

  updateAnnotation: (id: string, updates: Partial<Annotation>) => {
    const { annotations } = get();
    const newAnnotations = annotations.map((ann) =>
      ann.id === id ? { ...ann, ...updates } : ann
    );
    set({ annotations: newAnnotations });
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

