import Konva from 'konva';
import { BlurMode, CanvasImage } from '../types';

export type SoftBlurMode = Exclude<BlurMode, 'secure'>;

// A blur box in doc px: top-left corner, size, and rotation in degrees about
// that corner (the same meaning as the box of a rectangle annotation).
export interface EffectBox {
  x: number;
  y: number;
  width: number;
  height: number;
  rotation: number;
}

// Strength of a soft mode in doc px: the blur radius, or the side of a mosaic
// block. It goes through the annotation's unit, so a box keeps its look
// whatever the size of the shot.
export function effectStrength(mode: SoftBlurMode, unit: number): number {
  const px = Math.round((mode === 'blur' ? 12 : 10) * unit);
  return Math.min(60, Math.max(4, px));
}

// Runs a Konva filter on pixels. The filters read their setting from the node
// they are called on, so a bare getter stands in for it: there is no node, the
// pixels are not on a stage.
function filterPixels(mode: SoftBlurMode, strength: number, data: ImageData): void {
  if (mode === 'blur') {
    Konva.Filters.Blur.call({ blurRadius: () => strength } as unknown as Konva.Node, data);
  } else {
    Konva.Filters.Pixelate.call({ pixelSize: () => strength } as unknown as Konva.Node, data);
  }
}

// Draws the picture under a box into out (the box's own orientation, one
// canvas pixel per doc px), blurred or pixelated by the Konva filter. Only the
// images (and the canvas colour behind them when there are several) are drawn,
// never annotations: a box that blurs a password must not blur the arrow that
// points at it.
//
// The filter works on a larger canvas than the box and the box is cut out of
// the result. A blur then takes its edge pixels from the picture around the
// box instead of fading out, and the mosaic blocks tile the box exactly (Konva
// averages a partial block at the edge with pixels that are not there).
export function paintEffect(
  out: HTMLCanvasElement,
  box: EffectBox,
  mode: SoftBlurMode,
  strength: number,
  images: CanvasImage[],
  elements: Map<string, HTMLImageElement>,
  background: string | null,
): void {
  const width = Math.max(1, Math.round(box.width));
  const height = Math.max(1, Math.round(box.height));
  out.width = width;
  out.height = height;
  const outCtx = out.getContext('2d');
  if (!outCtx) return;

  const margin = mode === 'blur' ? strength : 0;
  const work = document.createElement('canvas');
  work.width = mode === 'blur' ? width + 2 * margin : Math.ceil(width / strength) * strength;
  work.height = mode === 'blur' ? height + 2 * margin : Math.ceil(height / strength) * strength;
  const ctx = work.getContext('2d', { willReadFrequently: true });
  if (!ctx) return;

  if (background) {
    ctx.fillStyle = background;
    ctx.fillRect(0, 0, work.width, work.height);
  }
  // Doc -> work space is the inverse of the box's own transform (translate to
  // the corner, rotate), applied in reverse order.
  ctx.translate(margin, margin);
  ctx.scale(width / box.width, height / box.height);
  ctx.rotate((-box.rotation * Math.PI) / 180);
  ctx.translate(-box.x, -box.y);
  for (const im of images) {
    const el = elements.get(im.id);
    if (!el) continue;
    if (im.crop) {
      ctx.drawImage(el, im.crop.x, im.crop.y, im.crop.width, im.crop.height, im.x, im.y, im.width, im.height);
    } else {
      ctx.drawImage(el, im.x, im.y, im.width, im.height);
    }
  }

  const data = ctx.getImageData(0, 0, work.width, work.height);
  filterPixels(mode, strength, data);
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.putImageData(data, 0, 0);
  outCtx.drawImage(work, margin, margin, width, height, 0, 0, width, height);
}
