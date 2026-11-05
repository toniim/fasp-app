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
