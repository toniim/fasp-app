// Space the editor chrome takes around the stage (toolbar, action bar, padding).
const STAGE_MARGIN_X = 40;
const STAGE_MARGIN_Y = 140;

// fitScale returns the factor that fits a document area (the frame) of the
// given size into the editor window without upscaling it. It is a view scale
// only: the stage scale is zoom * fitScale and the document stays in document
// pixels, so changing it never touches images or annotations.
export function fitScale(imageWidth: number, imageHeight: number): number {
  const maxWidth = window.innerWidth - STAGE_MARGIN_X;
  const maxHeight = window.innerHeight - STAGE_MARGIN_Y;
  return Math.min(maxWidth / imageWidth, maxHeight / imageHeight, 1);
}

// Gap between a new image and the existing content, in doc px.
export const IMAGE_GAP = 24;
// Space around the images inside the frame once there are two or more.
export const MULTI_IMAGE_PADDING = 32;

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

export function framePadding(imageCount: number): number {
  return imageCount >= 2 ? MULTI_IMAGE_PADDING : 0;
}

// Bounding box of boxes, or null when there are none.
export function boundsOf(boxes: Box[]): Box | null {
  if (boxes.length === 0) return null;
  let left = Infinity;
  let top = Infinity;
  let right = -Infinity;
  let bottom = -Infinity;
  for (const b of boxes) {
    left = Math.min(left, b.x);
    top = Math.min(top, b.y);
    right = Math.max(right, b.x + b.width);
    bottom = Math.max(bottom, b.y + b.height);
  }
  return { x: left, y: top, width: right - left, height: bottom - top };
}

// Grows a box outward by pad on every side and snaps it to whole doc px.
function inflateToIntegers(box: Box, pad: number): Box {
  const left = Math.floor(box.x - pad);
  const top = Math.floor(box.y - pad);
  const right = Math.ceil(box.x + box.width + pad);
  const bottom = Math.ceil(box.y + box.height + pad);
  return { x: left, y: top, width: right - left, height: bottom - top };
}

// Auto frame: the images' bounding box plus padding.
export function autoFrame(images: Box[]): Box {
  const bounds = boundsOf(images);
  if (!bounds) return { x: 0, y: 0, width: 0, height: 0 };
  return inflateToIntegers(bounds, framePadding(images.length));
}

// Manual frame after an image was added: keep the frame and grow it to hold
// the new image plus padding. imageCount is the count after the add.
export function growFrame(frame: Box, added: Box, imageCount: number): Box {
  const need = inflateToIntegers(added, framePadding(imageCount));
  const left = Math.min(frame.x, need.x);
  const top = Math.min(frame.y, need.y);
  const right = Math.max(frame.x + frame.width, need.x + need.width);
  const bottom = Math.max(frame.y + frame.height, need.y + need.height);
  return { x: left, y: top, width: right - left, height: bottom - top };
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

// keepInView is clampPosition without the centering: content smaller than the
// viewport stays where it is as long as it is fully visible, and is pushed back
// inside otherwise. Used when an image edit grows the frame, so the view does
// not move under the pointer.
export function keepInView(pos: Position, content: Size, viewport: Size): Position {
  const axis = (view: number, size: number, value: number) => snapToDevicePixel(
    size <= view ? Math.min(view - size, Math.max(0, value)) : Math.min(0, Math.max(view - size, value)),
  );

  return {
    x: axis(viewport.width, content.width, pos.x),
    y: axis(viewport.height, content.height, pos.y),
  };
}
