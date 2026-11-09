import { Annotation } from '../types';

// Space the editor chrome takes around the stage (toolbar, action bar, padding).
const STAGE_MARGIN_X = 40;
const STAGE_MARGIN_Y = 140;

// fitScale returns the factor that fits an image of the given size into the
// editor window without upscaling it. Annotations are stored in stage
// coordinates (image pixels * this factor), so every place that changes the
// image size must use this same function to keep them aligned.
export function fitScale(imageWidth: number, imageHeight: number): number {
  const maxWidth = window.innerWidth - STAGE_MARGIN_X;
  const maxHeight = window.innerHeight - STAGE_MARGIN_Y;
  return Math.min(maxWidth / imageWidth, maxHeight / imageHeight, 1);
}

export interface Size {
  width: number;
  height: number;
}

export interface Position {
  x: number;
  y: number;
}

// Konva sizes the canvas backing store as cssSize * dpr, which the browser
// truncates, while Chromium snaps the element's on-screen box to whole device
// pixels. On a fractional dpr (Windows at 110%: 2279 css px * 1.104 = 2516.4)
// the box ends up one pixel larger than the backing store and the compositor
// stretches the whole canvas, blurring every pixel. Sizing the stage to a whole
// number of device pixels (plus a hair, so the truncation cannot drop one)
// keeps backing store and box identical. alignToDevicePixels has to put the
// canvas origin on the pixel grid as well, or the box can still round up.
const DEVICE_PIXEL_SLACK = 0.05;

function deviceAligned(css: number, dpr: number): number {
  return (Math.floor(css * dpr) + DEVICE_PIXEL_SLACK) / dpr;
}

// viewportSize returns the area available to the canvas. The stage is always
// this size and the image is positioned/scaled inside it, so zooming never
// clips the capture against an image-sized canvas.
export function viewportSize(): Size {
  const dpr = window.devicePixelRatio || 1;
  return {
    width: deviceAligned(Math.max(200, window.innerWidth - STAGE_MARGIN_X), dpr),
    height: deviceAligned(Math.max(200, window.innerHeight - STAGE_MARGIN_Y), dpr),
  };
}

// snapToDevicePixel rounds a stage offset (css px) so that it lands on a whole
// device pixel. A fractional offset makes Konva draw the image between pixels,
// which resamples it even at real size.
export function snapToDevicePixel(value: number): number {
  const dpr = window.devicePixelRatio || 1;
  return Math.round(value * dpr) / dpr;
}

// alignToDevicePixels nudges an element so its top-left corner sits on the
// device pixel grid (see deviceAligned for why the canvas needs this).
export function alignToDevicePixels(el: HTMLElement): void {
  const dpr = window.devicePixelRatio || 1;
  el.style.position = 'relative';
  el.style.left = '0px';
  el.style.top = '0px';
  const rect = el.getBoundingClientRect();
  const offset = (v: number) => (Math.round(v * dpr) - v * dpr) / dpr;
  el.style.left = `${offset(rect.left)}px`;
  el.style.top = `${offset(rect.top)}px`;
}

// canPan reports whether the zoomed content overflows the viewport on either
// axis, i.e. whether there is anything to pan to (clampPosition centers the
// content otherwise).
export function canPan(content: Size, viewport: Size): boolean {
  return content.width > viewport.width || content.height > viewport.height;
}

// clampPosition keeps the content reachable inside the viewport: centered when
// it is smaller than the viewport, otherwise constrained so its edges cannot be
// panned past the viewport edges. The result is snapped to device pixels.
export function clampPosition(pos: Position, content: Size, viewport: Size): Position {
  const axis = (view: number, size: number, value: number) => snapToDevicePixel(
    size <= view ? (view - size) / 2 : Math.min(0, Math.max(view - size, value)),
  );

  return {
    x: axis(viewport.width, content.width, pos.x),
    y: axis(viewport.height, content.height, pos.y),
  };
}

// scaleAnnotation returns a copy of the annotation with every stage-space
// length multiplied by factor.
export function scaleAnnotation(ann: Annotation, factor: number): Annotation {
  if (factor === 1) return ann;
  const scaled: Annotation = { ...ann, x: ann.x * factor, y: ann.y * factor };
  if (ann.width !== undefined) scaled.width = ann.width * factor;
  if (ann.height !== undefined) scaled.height = ann.height * factor;
  if (ann.strokeWidth !== undefined) scaled.strokeWidth = ann.strokeWidth * factor;
  if (ann.fontSize !== undefined) scaled.fontSize = ann.fontSize * factor;
  if (ann.points) scaled.points = ann.points.map((p) => p * factor);
  return scaled;
}
