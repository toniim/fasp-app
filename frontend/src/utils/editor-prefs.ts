// Editor tool settings kept across restarts: the backdrop, the highlight
// options, the rectangle radius and the blur mode. They live in the app
// settings under "editor_prefs" and are written a moment after the last change.

import { BackdropRatio, BackdropSettings, BlurMode, EditorState, HighlightOptions } from '../types';
import { useEditorStore } from '../store/editorStore';
import { BACKDROP_PRESETS, BACKDROP_RATIOS, CUSTOM_BACKDROP_ID, DEFAULT_BACKDROP } from './backdrops';
import { GetSettings, UpdateSetting } from '../../bindings/github.com/heytonyne/fasp/app';

const SAVE_DELAY_MS = 500;
const BLUR_MODES: BlurMode[] = ['secure', 'blur', 'pixelate'];

export interface EditorPrefs {
  // The user's own backdrop image is not kept: it can be many MB.
  backdrop: Omit<BackdropSettings, 'customImage'>;
  highlight: HighlightOptions;
  rectangleRadius: number;
  blurMode: BlurMode;
}

type PrefsSource = Pick<EditorState, 'backdrop' | 'highlightOptions' | 'rectangleRadius' | 'blurMode'>;

export function pickPrefs(state: PrefsSource): EditorPrefs {
  const { customImage: _customImage, ...backdrop } = state.backdrop;
  return {
    // Without the image, the custom preset would point at nothing.
    backdrop: backdrop.presetId === CUSTOM_BACKDROP_ID ? { ...backdrop, presetId: DEFAULT_BACKDROP.presetId } : backdrop,
    highlight: state.highlightOptions,
    rectangleRadius: state.rectangleRadius,
    blurMode: state.blurMode,
  };
}

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const num = (v: unknown, min: number, max: number): number | undefined =>
  typeof v === 'number' && Number.isFinite(v) ? Math.min(max, Math.max(min, v)) : undefined;

// Reads what the settings file holds. A missing, old or hand-edited value is
// ignored field by field, so the defaults stay for it.
export function sanitizePrefs(raw: unknown): Partial<PrefsSource> {
  const out: Partial<PrefsSource> = {};
  if (!isObject(raw)) return out;

  if (isObject(raw.backdrop)) {
    const b = raw.backdrop;
    const next: BackdropSettings = { ...DEFAULT_BACKDROP };
    if (typeof b.enabled === 'boolean') next.enabled = b.enabled;
    if (typeof b.presetId === 'string' && BACKDROP_PRESETS.some((p) => p.id === b.presetId)) next.presetId = b.presetId;
    if (typeof b.ratio === 'string' && BACKDROP_RATIOS.includes(b.ratio as BackdropRatio)) next.ratio = b.ratio as BackdropRatio;
    next.padding = num(b.padding, 2, 20) ?? next.padding;
    next.radius = num(b.radius, 0, 40) ?? next.radius;
    if (typeof b.shadow === 'boolean') next.shadow = b.shadow;
    if (typeof b.border === 'boolean') next.border = b.border;
    next.borderWidth = num(b.borderWidth, 1, 12) ?? next.borderWidth;
    if (typeof b.borderColor === 'string' && /^#[0-9a-fA-F]{6}$/.test(b.borderColor)) next.borderColor = b.borderColor;
    out.backdrop = next;
  }
  if (isObject(raw.highlight)) {
    out.highlightOptions = {
      dim: num(raw.highlight.dim, 0, 0.9) ?? 0.5,
      radius: num(raw.highlight.radius, 0, 40) ?? 8,
    };
  }
  const radius = num(raw.rectangleRadius, 0, 40);
  if (radius !== undefined) out.rectangleRadius = radius;
  if (typeof raw.blurMode === 'string' && BLUR_MODES.includes(raw.blurMode as BlurMode)) out.blurMode = raw.blurMode as BlurMode;
  return out;
}

let started = false;

// Loads the saved prefs into the store once per run, then saves every change.
// Saving starts only after the load, so the defaults never overwrite the file.
export function initEditorPrefs(): void {
  if (started) return;
  started = true;

  GetSettings()
    .then((settings) => {
      useEditorStore.setState(sanitizePrefs(settings?.editor_prefs));
    })
    .catch((error) => console.error('Failed to load editor settings:', error))
    .finally(watchPrefs);
}

function watchPrefs(): void {
  let saved = JSON.stringify(pickPrefs(useEditorStore.getState()));
  let timer: ReturnType<typeof setTimeout> | null = null;

  const flush = () => {
    timer = null;
    const prefs = pickPrefs(useEditorStore.getState());
    const json = JSON.stringify(prefs);
    if (json === saved) return;
    saved = json;
    UpdateSetting('editor_prefs', prefs).catch((error) => {
      // Try again on the next change.
      saved = '';
      console.error('Failed to save editor settings:', error);
    });
  };

  useEditorStore.subscribe((state, prev) => {
    if (
      state.backdrop === prev.backdrop &&
      state.highlightOptions === prev.highlightOptions &&
      state.rectangleRadius === prev.rectangleRadius &&
      state.blurMode === prev.blurMode
    ) {
      return;
    }
    if (timer) clearTimeout(timer);
    timer = setTimeout(flush, SAVE_DELAY_MS);
  });

  // A change made just before the window closes still reaches the file.
  window.addEventListener('beforeunload', () => {
    if (timer) {
      clearTimeout(timer);
      flush();
    }
  });
}
