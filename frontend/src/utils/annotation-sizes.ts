// The five Size steps of the toolbar, in screen px. Times the annotation's
// unit (doc px per screen px when it was created) they give doc px, so an
// annotation keeps its proportion to the image whatever the zoom.

const STROKE_WIDTHS = [1, 2, 4, 6, 8]; // shapes and arrows
const FONT_SIZES = [12, 16, 20, 24, 32]; // text
// The marker is a highlighter: much wider than a pen line at the same step.
const MARKER_WIDTHS = [8, 14, 22, 32, 44];

// How much of the picture shows through a marker stroke.
export const MARKER_OPACITY = 0.4;

const step =(size: number) => Math.min(Math.max(Math.round(size), 1), 5) - 1;

export const strokeForSize = (size: number): number => STROKE_WIDTHS[step(size)];
export const fontForSize = (size: number): number => FONT_SIZES[step(size)];
export const markerForSize = (size: number): number => MARKER_WIDTHS[step(size)];
