// Pure layout math for Row / Column / Grid. No React or store imports.

import type { ArrangeMode } from '../types';

export interface LayoutImage {
  id: string;
  x: number;
  y: number;
  width: number; // current displayed box, doc px
  height: number;
  naturalWidth: number;
  naturalHeight: number;
}

export interface LayoutBox {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

// naturalWidth/naturalHeight are the size of the visible source: the full
// image, or its crop once images can be cropped. Boxes keep that aspect (resize
// is aspect locked), so the natural size is both the cap and the aspect source;
// using the rounded current box instead drifts a pixel per arrange.
const maxWidthOf = (im: LayoutImage): number => im.naturalWidth;
const maxHeightOf = (im: LayoutImage): number => im.naturalHeight;

// Common height for a row: the smallest natural-size height among the images.
// It comes from the natural sizes, never the current boxes, so arranging again
// (or switching modes) gives the same sizes instead of shrinking every time.
function commonHeight(images: LayoutImage[]): number {
  return Math.max(1, Math.floor(Math.min(...images.map(maxHeightOf))));
}

function commonWidth(images: LayoutImage[]): number {
  return Math.max(1, Math.floor(Math.min(...images.map(maxWidthOf))));
}

// Reading order for the grid: two images share a row when their vertical
// ranges overlap by at least half the smaller height (top-aligned shots of very
// different heights, or a row the grid centered); same row goes left to right,
// otherwise top first. Raw y would flip neighbours the grid centered.
function readingOrder(a: LayoutImage, b: LayoutImage): number {
  const overlap = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y);
  if (overlap >= Math.min(a.height, b.height) / 2) return a.x - b.x;
  return a.y - b.y;
}

const widthAtHeight =(im: LayoutImage, h: number) => Math.max(1, Math.round((h * im.naturalWidth) / im.naturalHeight));
const heightAtWidth = (im: LayoutImage, w: number) => Math.max(1, Math.round((w * im.naturalHeight) / im.naturalWidth));

// Lays the images out starting at (0, 0); the frame adds its padding later.
// Returns one box per input image, in the input order (z-order is untouched).
// All modes keep the images' current reading order (see readingOrder), so
// switching between modes never reshuffles them.
//  row:    same height, left to right, top aligned.
//  column: same width, top to bottom, left aligned.
//  grid:   ceil(sqrt(n)) columns in reading order (y, then x); every image has
//          the same width, each row is as tall as its tallest image, images
//          are centered vertically in their row.
// Images never grow past their natural size: the shared size is the smallest
// natural limit among them. Arranging is idempotent: same input set, same boxes.
export function arrange(images: LayoutImage[], mode: ArrangeMode, gap: number): LayoutBox[] {
  if (images.length === 0) return [];
  const g = Math.max(0, Math.round(gap));
  const boxes = new Map<string, LayoutBox>();

  if (mode === 'row') {
    const h = commonHeight(images);
    let x = 0;
    for (const im of [...images].sort(readingOrder)) {
      const w = widthAtHeight(im, h);
      boxes.set(im.id, { id: im.id, x, y: 0, width: w, height: h });
      x += w + g;
    }
  } else if (mode === 'column') {
    const w = commonWidth(images);
    let y = 0;
    for (const im of [...images].sort(readingOrder)) {
      const h = heightAtWidth(im, w);
      boxes.set(im.id, { id: im.id, x: 0, y, width: w, height: h });
      y += h + g;
    }
  } else {
    const cols = Math.ceil(Math.sqrt(images.length));
    const w = commonWidth(images);
    const sized = [...images]
      .sort(readingOrder)
      .map((im) => ({ im, h: heightAtWidth(im, w) }));
    // Each row is as tall as its tallest image, so one portrait shot does not
    // stretch every row.
    const rowCount = Math.ceil(sized.length / cols);
    const rowH = Array.from({ length: rowCount }, (_, r) =>
      Math.max(...sized.slice(r * cols, r * cols + cols).map((s) => s.h)));
    let rowY = 0;
    rowH.forEach((cellH, r) => {
      sized.slice(r * cols, r * cols + cols).forEach(({ im, h }, col) => {
        boxes.set(im.id, {
          id: im.id,
          x: col * (w + g),
          y: rowY + Math.round((cellH - h) / 2),
          width: w,
          height: h,
        });
      });
      rowY += cellH + g;
    });
  }

  return images.map((im) => boxes.get(im.id)!);
}
