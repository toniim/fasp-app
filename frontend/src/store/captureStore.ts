import { create } from 'zustand';
import { CaptureResult, CropRegion } from '../types';

interface CaptureStore {
  captureResult: CaptureResult | null;
  cropRegion: CropRegion | null;
  isSelecting: boolean;
  setCaptureResult: (result: CaptureResult | null) => void;
  setCropRegion: (region: CropRegion | null) => void;
  setIsSelecting: (selecting: boolean) => void;
  reset: () => void;
}

export const useCaptureStore = create<CaptureStore>((set) => ({
  captureResult: null,
  cropRegion: null,
  isSelecting: false,

  setCaptureResult: (result: CaptureResult | null) => {
    set({ captureResult: result });
  },

  setCropRegion: (region: CropRegion | null) => {
    set({ cropRegion: region });
  },

  setIsSelecting: (selecting: boolean) => {
    set({ isSelecting: selecting });
  },

  reset: () => {
    set({ captureResult: null, cropRegion: null, isSelecting: false });
  },
}));

