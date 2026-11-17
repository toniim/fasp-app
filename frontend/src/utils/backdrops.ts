// Backdrops: a background picture drawn behind the shot, with a margin around
// it, for sharing. Colour backdrops are painted from code (gradients and soft
// colour blobs) and stay sharp at any size. Photo backdrops are CC0 pictures
// from Wikimedia Commons shipped with the app (sources in
// assets/backdrops/CREDITS.md); the user can also pick an image of their own.

import { useEffect, useState } from 'react';
import { BackdropRatio, BackdropSettings, Frame } from '../types';
import { PHOTO_BACKDROPS } from './backdrop-photos';

// A soft colour spot: a radial gradient from color to transparent. x, y and r
// are fractions of the backdrop (r of its longer side).
interface Blob {
  x: number;
  y: number;
  r: number;
  color: string;
}

export interface BackdropPreset {
  id: string;
  name: string;
  // Base fill: one colour, or a linear gradient at angle degrees (0 = left to
  // right, 90 = top to bottom) through the colours. For a photo it is the
  // stand-in while the photo loads.
  colors: string[];
  angle: number;
  blobs?: Blob[];
  // A photo drawn over the whole backdrop (cover fit), and its small preview.
  image?: string;
  thumb?: string;
}

// Id of the backdrop made from the user's own image (settings.customImage).
export const CUSTOM_BACKDROP_ID = 'custom';

export const COLOR_BACKDROPS: BackdropPreset[] = [
  {
    id: 'hills', name: 'Hills', colors: ['#3a5bd9', '#6d3fd6'], angle: 90,
    blobs: [
      { x: 0.05, y: 0.15, r: 0.55, color: '#ff5a4e' },
      { x: 0.2, y: 1.0, r: 0.7, color: '#2e9e44' },
      { x: 0.95, y: 0.9, r: 0.5, color: '#7bc043' },
    ],
  },
  {
    id: 'aurora', name: 'Aurora', colors: ['#0b1026', '#111a3a'], angle: 90,
    blobs: [
      { x: 0.15, y: 0.2, r: 0.55, color: '#22d3ee' },
      { x: 0.85, y: 0.3, r: 0.55, color: '#a855f7' },
      { x: 0.5, y: 1.0, r: 0.6, color: '#10b981' },
    ],
  },
  {
    id: 'candy', name: 'Candy', colors: ['#ffe4ec', '#e8f0ff'], angle: 45,
    blobs: [
      { x: 0.1, y: 0.1, r: 0.5, color: '#f9a8d4' },
      { x: 0.9, y: 0.2, r: 0.5, color: '#a5b4fc' },
      { x: 0.6, y: 1.0, r: 0.55, color: '#7dd3fc' },
    ],
  },
  {
    id: 'sunset', name: 'Sunset', colors: ['#2b1055', '#7597de'], angle: 90,
    blobs: [
      { x: 0.5, y: 1.05, r: 0.65, color: '#ff7e5f' },
      { x: 0.1, y: 0.9, r: 0.4, color: '#feb47b' },
    ],
  },
  { id: 'ocean', name: 'Ocean', colors: ['#1e3c72', '#2a5298', '#38bdf8'], angle: 135 },
  { id: 'peach', name: 'Peach', colors: ['#ffecd2', '#fcb69f'], angle: 135 },
  { id: 'lavender', name: 'Lavender', colors: ['#a18cd1', '#fbc2eb'], angle: 135 },
  { id: 'mint', name: 'Mint', colors: ['#d4fc79', '#96e6a1'], angle: 135 },
  { id: 'sky', name: 'Sky', colors: ['#89f7fe', '#66a6ff'], angle: 135 },
  { id: 'fire', name: 'Fire', colors: ['#f12711', '#f5af19'], angle: 135 },
  { id: 'midnight', name: 'Midnight', colors: ['#232526', '#414345'], angle: 135 },
  { id: 'paper', name: 'Paper', colors: ['#f5f5f7'], angle: 0 },
];

export const BACKDROP_PRESETS: BackdropPreset[] = [...PHOTO_BACKDROPS, ...COLOR_BACKDROPS];

export const BACKDROP_RATIOS: BackdropRatio[] = ['auto', '16:9', '1:1', '4:5', '3:2'];

// Width / height of a ratio preset; null for auto.
const ratioValue = (ratio: BackdropRatio): number | null => {
  if (ratio === 'auto') return null;
  const [w, h] = ratio.split(':').map(Number);
  return w / h;
};

export const DEFAULT_BACKDROP: BackdropSettings = {
  enabled: false,
  presetId: 'hills',
  ratio: 'auto',
  padding: 8,
  radius: 12,
  shadow: true,
};

// The preset the settings select: the user's own image, a shipped one, or the
// first one when the id is unknown.
export const backdropPreset = (s: BackdropSettings): BackdropPreset => {
  if (s.presetId === CUSTOM_BACKDROP_ID && s.customImage) {
    return { id: CUSTOM_BACKDROP_ID, name: 'Your image', colors: ['#6b7280', '#374151'], angle: 135, image: s.customImage };
  }
  return BACKDROP_PRESETS.find((p) => p.id === s.presetId) ?? COLOR_BACKDROPS[0];
};

// Decoded photos by URL, kept for the session so switching back is instant.
const photoCache = new Map<string, HTMLImageElement>();

// useBackdropPhoto returns the decoded photo for src, or null while it loads
// (and when there is no src).
export function useBackdropPhoto(src: string | undefined): HTMLImageElement | null {
  const ready = (el: HTMLImageElement | undefined) => (el && el.complete && el.naturalWidth > 0 ? el : null);
  const [img, setImg] = useState<HTMLImageElement | null>(() => (src ? ready(photoCache.get(src)) : null));
  useEffect(() => {
    if (!src) {
      setImg(null);
      return;
    }
    let el = photoCache.get(src);
    if (ready(el)) {
      setImg(el!);
      return;
    }
    setImg(null);
    if (!el) {
      el = new Image();
      el.src = src;
      photoCache.set(src, el);
    }
    let cancelled = false;
    const target = el;
    target.decode().then(
      () => { if (!cancelled) setImg(target); },
      () => { photoCache.delete(src); },
    );
    return () => { cancelled = true; };
  }, [src]);
  return img;
}

const withAlpha = (hex: string, alpha: number): string => {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
};

// paintBackdrop fills the rectangle r with the preset: its photo when given
// (scaled to cover r, centred), else its colours.
export function paintBackdrop(ctx: CanvasRenderingContext2D, r: Frame, preset: BackdropPreset, photo?: HTMLImageElement | null): void {
  ctx.save();
  ctx.beginPath();
  ctx.rect(r.x, r.y, r.width, r.height);
  ctx.clip();
  if (photo) {
    const k = Math.max(r.width / photo.naturalWidth, r.height / photo.naturalHeight);
    const w = photo.naturalWidth * k;
    const h = photo.naturalHeight * k;
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(photo, r.x + (r.width - w) / 2, r.y + (r.height - h) / 2, w, h);
    ctx.restore();
    return;
  }
  if (preset.colors.length === 1) {
    ctx.fillStyle = preset.colors[0];
  } else {
    // Gradient line through the centre, long enough to cover the corners.
    const a = (preset.angle * Math.PI) / 180;
    const cx = r.x + r.width / 2;
    const cy = r.y + r.height / 2;
    const half = (Math.abs(r.width * Math.cos(a)) + Math.abs(r.height * Math.sin(a))) / 2;
    const g = ctx.createLinearGradient(cx - Math.cos(a) * half, cy - Math.sin(a) * half, cx + Math.cos(a) * half, cy + Math.sin(a) * half);
    preset.colors.forEach((c, i) => g.addColorStop(i / (preset.colors.length - 1), c));
    ctx.fillStyle = g;
  }
  ctx.fillRect(r.x, r.y, r.width, r.height);
  const side = Math.max(r.width, r.height);
  for (const b of preset.blobs ?? []) {
    const x = r.x + b.x * r.width;
    const y = r.y + b.y * r.height;
    const rad = b.r * side;
    const g = ctx.createRadialGradient(x, y, 0, x, y, rad);
    g.addColorStop(0, withAlpha(b.color, 0.9));
    g.addColorStop(0.5, withAlpha(b.color, 0.45));
    g.addColorStop(1, withAlpha(b.color, 0));
    ctx.fillStyle = g;
    ctx.fillRect(r.x, r.y, r.width, r.height);
  }
  ctx.restore();
}

export interface BackdropLayout {
  // The exported picture: the frame plus the margin, in doc px.
  outer: Frame;
  // Corner radius of the shot and its shadow, in doc px.
  radius: number;
  shadowBlur: number;
  shadowOffset: number;
}

// Sizes scale with the shot, so a Retina capture and a small one look alike:
// padding is a percentage of the longer side, and the radius is in px of a
// shot 1200 px on its longer side. A ratio preset grows the padded box on one
// axis until it has that ratio, so the padding is a minimum and the shot stays
// centred (to the pixel, the odd pixel goes to the right / bottom).
export function backdropLayout(frame: Frame, s: BackdropSettings): BackdropLayout {
  const side = Math.max(frame.width, frame.height);
  const pad = Math.round((side * s.padding) / 100);
  const unit = side / 1200;
  let width = frame.width + 2 * pad;
  let height = frame.height + 2 * pad;
  const ratio = ratioValue(s.ratio ?? 'auto');
  if (ratio !== null) {
    if (width / height < ratio) width = Math.round(height * ratio);
    else height = Math.round(width / ratio);
  }
  const left = Math.floor((width - frame.width) / 2);
  const top = Math.floor((height - frame.height) / 2);
  return {
    outer: { x: frame.x - left, y: frame.y - top, width, height },
    radius: Math.min(s.radius * unit, Math.min(frame.width, frame.height) / 2),
    shadowBlur: 40 * unit,
    shadowOffset: 14 * unit,
  };
}
