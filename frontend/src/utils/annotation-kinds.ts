import { Annotation, AnnotationTool } from '../types';

// Annotations that are a list of points drawn at the node origin: a move or a
// transform is baked into the points and the node goes back to the origin.
const POINT_TYPES: AnnotationTool[] = ['arrow', 'arrow-text', 'numbered-arrow', 'pen', 'marker', 'line'];

export const hasPoints = (ann: Annotation): ann is Annotation & { points: number[] } =>
  !!ann.points && POINT_TYPES.includes(ann.type);

// Freehand strokes: points collected while the pointer moves.
export const isFreehand = (tool: AnnotationTool): boolean => tool === 'pen' || tool === 'marker';

// Ramer-Douglas-Peucker on a flat [x0, y0, x1, y1, ...] list: drops points that
// lie within epsilon of the line through their neighbours, so a hand-drawn
// stroke keeps its shape with far fewer points.
export function simplifyPoints(points: number[], epsilon: number): number[] {
  const n = points.length / 2;
  if (n < 3) return points;
  const keep = new Array<boolean>(n).fill(false);
  keep[0] = true;
  keep[n - 1] = true;
  // Iterative, so a long stroke cannot overflow the call stack.
  const stack: Array<[number, number]> = [[0, n - 1]];
  while (stack.length) {
    const [a, b] = stack.pop()!;
    const ax = points[a * 2];
    const ay = points[a * 2 + 1];
    const dx = points[b * 2] - ax;
    const dy = points[b * 2 + 1] - ay;
    const len = Math.hypot(dx, dy);
    let far = -1;
    let farDist = epsilon;
    for (let i = a + 1; i < b; i++) {
      const px = points[i * 2] - ax;
      const py = points[i * 2 + 1] - ay;
      // Distance to the segment's line; to the start point when it has no length.
      const d = len === 0 ? Math.hypot(px, py) : Math.abs(px * dy - py * dx) / len;
      if (d > farDist) {
        far = i;
        farDist = d;
      }
    }
    if (far >= 0) {
      keep[far] = true;
      stack.push([a, far], [far, b]);
    }
  }
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    if (keep[i]) out.push(points[i * 2], points[i * 2 + 1]);
  }
  return out;
}
