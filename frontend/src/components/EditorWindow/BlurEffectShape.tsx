import React, { useCallback, useLayoutEffect, useRef, useState } from 'react';
import { Image as KonvaImage } from 'react-konva';
import Konva from 'konva';
import { Annotation, CanvasImage } from '../../types';
import { EffectBox, SoftBlurMode, effectStrength, paintEffect } from '../../utils/blur-effect';

interface BlurEffectShapeProps {
  ann: Annotation;
  mode: SoftBlurMode;
  images: CanvasImage[];
  elements: Map<string, HTMLImageElement>;
  // Bumps when an image finishes decoding.
  version: number;
  // Canvas colour drawn under the images, when it shows (several images).
  background: string | null;
  unit: number;
  // Interaction props of the annotation (drag, click, hover, transform end).
  shapeProps: Record<string, unknown>;
  register: (node: Konva.Image | null) => void;
}

// A blur box that keeps the look of the content: a Konva Image whose pixels are
// the picture under the box, run through a Konva filter (Blur or Pixelate) and
// cut to the box by being exactly the box. As a plain node it moves, resizes,
// rotates and exports like any other annotation.
const BlurEffectShape: React.FC<BlurEffectShapeProps> = ({
  ann, mode, images, elements, version, background, unit, shapeProps, register,
}) => {
  const [canvas] = useState(() => document.createElement('canvas'));
  const nodeRef = useRef<Konva.Image | null>(null);

  // A box drawn right-to-left or bottom-to-top has a negative size; the node
  // wants the top-left corner.
  const rawW = ann.width ?? 0;
  const rawH = ann.height ?? 0;
  const w = Math.abs(rawW);
  const h = Math.abs(rawH);
  const x = rawW < 0 ? ann.x + rawW : ann.x;
  const y = rawH < 0 ? ann.y + rawH : ann.y;
  const rotation = ann.rotation || 0;
  const ready = w >= 1 && h >= 1;
  const strength = effectStrength(mode, unit);

  const repaint = useCallback(
    (box: EffectBox) => {
      paintEffect(canvas, box, mode, strength, images, elements, background);
      nodeRef.current?.getLayer()?.batchDraw();
    },
    [canvas, mode, strength, images, elements, background],
  );

  useLayoutEffect(() => {
    if (ready) repaint({ x, y, width: w, height: h, rotation });
  }, [repaint, ready, x, y, w, h, rotation, version]);

  if (!ready) return null;

  return (
    <KonvaImage
      ref={(node) => {
        nodeRef.current = node;
        register(node);
      }}
      image={canvas}
      x={x}
      y={y}
      width={w}
      height={h}
      rotation={rotation}
      {...shapeProps}
      // While the box is dragged its node moves before the store does: repaint
      // from the node so it shows what is under it now, not under where it was.
      onDragMove={(e: Konva.KonvaEventObject<DragEvent>) =>
        repaint({ x: e.target.x(), y: e.target.y(), width: w, height: h, rotation: e.target.rotation() })
      }
    />
  );
};

export default BlurEffectShape;
