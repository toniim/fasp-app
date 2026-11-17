import React, { useRef, useEffect, useLayoutEffect, useCallback, useState } from 'react';
import { Stage, Layer, Image as KonvaImage, Rect, Arrow, Text, Transformer, Line, Circle, Group, Shape } from 'react-konva';
import { useEditorStore } from '../../store/editorStore';
import { Annotation } from '../../types';
import Toolbar from './Toolbar';
import ActionBar from './ActionBar';
import ZoomBar from './ZoomBar';
import Toast from '../Toast/Toast';
import { GetVersion, ReadClipboardImage, ReadImageFile } from '../../../wailsjs/go/main/App';
import { OnFileDrop, OnFileDropOff } from '../../../wailsjs/runtime/runtime';
import { IMAGE_GAP, fitScale, viewportSize, clampPosition, keepInView, canPan, alignToDevicePixels } from '../../utils/editor-geometry';
import { backdropLayout, backdropPreset, paintBackdrop, useBackdropPhoto } from '../../utils/backdrops';
import { MARKER_OPACITY, fontForSize, markerForSize, strokeForSize } from '../../utils/annotation-sizes';
import { hasPoints, isFreehand, simplifyPoints } from '../../utils/annotation-kinds';
import { initEditorPrefs } from '../../utils/editor-prefs';
import BlurEffectShape from './BlurEffectShape';
import './EditorWindow.css';

// Node name of everything that counts as empty canvas for the marquee: the
// background rect, and the image itself when it is the only one.
const BACKGROUND_NAME = 'background-rect';
const IMAGE_NAME = 'canvas-image';

const IMAGE_EXTENSIONS = ['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp'];

// Highlights work as a spotlight: the frame is dimmed except inside the
// highlight boxes. Each box narrows the clip to "frame minus this box"
// (even-odd); clips intersect, so the result is the frame minus the union of
// the boxes and overlapping boxes both stay bright.
type SpotlightBox = { x: number; y: number; width: number; height: number; rotation: number; radius: number };

function drawSpotlight(
  ctx: CanvasRenderingContext2D,
  frame: { x: number; y: number; width: number; height: number },
  boxes: SpotlightBox[],
  dim: number,
) {
  ctx.save();
  const base = ctx.getTransform();
  for (const b of boxes) {
    ctx.beginPath();
    ctx.rect(frame.x, frame.y, frame.width, frame.height);
    // Path points take the transform current when they are added: rotate the
    // box around its (x, y) corner like Konva does, then go back for the clip.
    ctx.translate(b.x, b.y);
    ctx.rotate((b.rotation * Math.PI) / 180);
    // roundRect scales radii that do not fit down to the box.
    ctx.roundRect(0, 0, b.width, b.height, Math.max(0, b.radius));
    ctx.setTransform(base);
    ctx.clip('evenodd');
  }
  ctx.fillStyle = `rgba(0, 0, 0, ${dim})`;
  ctx.fillRect(frame.x, frame.y, frame.width, frame.height);
  ctx.restore();
}

const isImagePath = (path: string): boolean => {
  const dot = path.lastIndexOf('.');
  return dot >= 0 && IMAGE_EXTENSIONS.includes(path.slice(dot + 1).toLowerCase());
};

// Wails rejects with the Go error string rather than an Error instance.
const errorText = (error: unknown): string => {
  if (error instanceof Error) return error.message;
  const text = String(error ?? '').trim();
  return text === '' ? 'unknown error' : text;
};

const EditorWindow: React.FC = () => {
  const {
    images,
    frame,
    background,
    annotations,
    selectedTool,
    selectedAnnotationId,
    selectedImageId,
    updateImage,
    removeImage,
    setSelectedImage,
    selectedColor,
    selectedSize,
    highlightOptions,
    rectangleRadius,
    blurMode,
    backdrop,
    cropRegion,
    pendingCrop,
    setCropRegion,
    applyCrop,
    keepPendingCrop,
    addImage,
    addAnnotation,
    updateAnnotation,
    deleteAnnotation,
    deleteAnnotations,
    setSelectedAnnotation,
    setSelectedTool,
    setSelectedSize,
    undo,
    redo,
  } = useEditorStore();

  // With a backdrop the picture is the frame plus a margin. viewFrame is what
  // the editor fits, pans and exports; the document (images, annotations,
  // clip) stays in frame.
  const backdropBox = backdrop.enabled ? backdropLayout(frame, backdrop) : null;
  const viewFrame = backdropBox ? backdropBox.outer : frame;
  const backdropLook = backdropPreset(backdrop);
  // A photo backdrop shows its stand-in colours until the photo is decoded.
  const backdropPhoto = useBackdropPhoto(backdrop.enabled ? backdropLook.image : undefined);

  // One decoded element per image id. The map lives in a ref (elements are
  // not render state); loadedCount bumps when one finishes decoding.
  const imageElsRef = useRef<Map<string, HTMLImageElement>>(new Map());
  const [loadedCount, setLoadedCount] = useState(0);
  const [blurPatternImage, setBlurPatternImage] = useState<HTMLImageElement | null>(null);
  // Size of the frame on screen at zoom 1 (frame * scaleRatio).
  const [stageSize, setStageSize] = useState({ width: 800, height: 600 });
  // The stage is always the size of the available area; the document is drawn
  // inside it. A stage sized to the frame would clip the capture as soon as
  // zoom pushes it past the (smaller) frame box.
  const [viewport, setViewport] = useState(() => viewportSize());
  // View scale that fits the frame into the window. The document is in doc px;
  // the stage scale is zoom * scaleRatio.
  const [scaleRatio, setScaleRatio] = useState(1);
  const [zoom, setZoom] = useState(1);
  // Screen px -> doc px for lengths that should look the same on screen
  // whatever the fit scale (editor chrome, creation thresholds).
  const px = 1 / scaleRatio;
  // Doc px -> screen px.
  const viewScale = zoom * scaleRatio;
  // Render as soon as one image is decoded: an image still decoding (a new
  // add) is skipped for a frame instead of unmounting the whole stage.
  const ready = images.some((im) => imageElsRef.current.has(im.id));
  const [stagePos, setStagePos] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  // Where the pointer and stage were when the pan started. The pan follows the
  // total pointer offset from here; summing per-event movementX/Y drifts
  // because each event's movement is rounded to whole CSS pixels.
  const panStartRef = useRef<{ pointer: { x: number; y: number }; pos: { x: number; y: number } } | null>(null);
  const [isModifierPressed, setIsModifierPressed] = useState(false);
  const [isShiftPressed, setIsShiftPressed] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);
  // Panning only makes sense when the zoomed image is larger than the editor.
  const panAvailable = canPan(
    { width: stageSize.width * zoom, height: stageSize.height * zoom },
    viewport,
  );
  // Shift turns any tool into a temporary hand tool.
  const shiftPanning = isShiftPressed && panAvailable;
  // The grab cursor shows whenever a mouse-down would start a pan.
  const panCursor = (isModifierPressed || isShiftPressed) && panAvailable;
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null);
  const [textInput, setTextInput] = useState<{ x: number; y: number; fontSize?: number; annotationId?: string } | null>(null);
  const [textValue, setTextValue] = useState('');
  const [version, setVersion] = useState('');
  const [hoveredAnnotationId, setHoveredAnnotationId] = useState<string | null>(null);
  // Id of the text-box annotation currently being edited inline (in-canvas).
  const [editingTextId, setEditingTextId] = useState<string | null>(null);
  // Marquee (rubber-band) multi-selection with the select tool.
  const [selectionRect, setSelectionRect] = useState<{ x: number; y: number; width: number; height: number } | null>(null);
  // In the store, so the toolbar's colours can recolour the whole selection.
  const marqueeSelectedIds = useEditorStore((s) => s.multiSelectedIds);
  const setMarqueeSelectedIds = useEditorStore((s) => s.setMultiSelectedIds);

  const stageRef = useRef<any>(null);
  const textInputRef = useRef<HTMLInputElement>(null);
  const textEditRef = useRef<HTMLTextAreaElement>(null);
  const transformerRef = useRef<any>(null);
  const shapeRefs = useRef<{ [key: string]: any }>({});
  const imageNodeRefs = useRef<{ [key: string]: any }>({});
  // A lone image acts as the canvas background (marquee and drawing start on
  // it, as in a single-image shot); with two or more each image is an object
  // that can be selected, moved and resized.
  const imagesSelectable = images.length > 1;
  const imageSelected = imagesSelectable && selectedImageId !== null;
  // The crop tool crops the selected image; with none selected it crops the
  // canvas. The drawn region is held inside the target image's box.
  const cropTarget = selectedTool === 'crop' && imageSelected
    ? images.find((im) => im.id === selectedImageId) ?? null
    : null;
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'info' } | null>(null);
  const closeToast = useCallback(() => setToast(null), []);
  // Latest view for the drop handler, which is registered once.
  const viewRef = useRef({ frame: viewFrame, stagePos, viewScale });
  viewRef.current = { frame: viewFrame, stagePos, viewScale };

  // Saved tool settings (backdrop, highlight, corners, blur mode) load once per
  // run; later changes are saved by the same module.
  useEffect(() => {
    initEditorPrefs();
  }, []);

  // Load version
  useEffect(() => {
    GetVersion().then((info) => {
      setVersion(info.version);
    }).catch((err) => {
      console.error('Failed to get version:', err);
    });
  }, []);

  // Decode each image once, keyed by id; drop elements of images that left the
  // document (a new shot, undo of an add).
  useEffect(() => {
    const els = imageElsRef.current;
    const ids = new Set(images.map((im) => im.id));
    els.forEach((_, id) => {
      if (!ids.has(id)) els.delete(id);
    });

    let cancelled = false;
    images.forEach((im) => {
      if (els.has(im.id)) return;
      const el = new window.Image();
      el.onload = () => {
        if (cancelled) return;
        els.set(im.id, el);
        setLoadedCount((n) => n + 1);
      };
      el.onerror = () => {
        console.error('Failed to decode image', im.id);
      };
      el.src = `data:image/png;base64,${im.src}`;
    });

    return () => {
      cancelled = true;
    };
  }, [images]);

  // Fit the frame to the window and reset the view. Only view state changes:
  // images and annotations are in doc px and never move with the window size.
  const refit = useCallback(() => {
    if (viewFrame.width <= 0 || viewFrame.height <= 0) return;
    const scale = fitScale(viewFrame.width, viewFrame.height);
    const size = {
      width: viewFrame.width * scale,
      height: viewFrame.height * scale,
    };
    const view = viewportSize();

    setStageSize(size);
    setViewport(view);
    setScaleRatio(scale);

    // Open every new frame fitted to the window so the whole capture is
    // visible; "Real Size" in the zoom bar shows it pixel-exact. A resize
    // refits as well: the editor window is shown and resized right after a
    // capture, so anything else would undo the fit on open.
    setZoom(1);
    setStagePos(clampPosition({ x: 0, y: 0 }, size, view));
  }, [viewFrame.width, viewFrame.height]);

  // Set by an image move or resize just before it commits. The auto frame
  // follows the images, so such an edit changes the frame; refitting then
  // would rescale the whole view under the pointer.
  const keepViewRef = useRef(false);
  const prevFrameRef = useRef(viewFrame);

  // Layout effect: a new frame (crop, undo of a crop, backdrop on or off) must
  // not paint once with the previous fit scale. After an image move or resize
  // the zoom stays and every doc point stays where it was on screen; anything
  // else refits.
  useLayoutEffect(() => {
    const prev = prevFrameRef.current;
    prevFrameRef.current = viewFrame;
    if (keepViewRef.current) {
      keepViewRef.current = false;
      const size = { width: viewFrame.width * scaleRatio, height: viewFrame.height * scaleRatio };
      setStageSize(size);
      // Screen x = stagePos.x + (doc x - viewFrame.x) * viewScale: shift the
      // stage by the frame origin's move so the doc content does not jump, and
      // pull it back only if part of the frame would leave the window.
      setStagePos((pos) => keepInView(
        {
          x: pos.x + (viewFrame.x - prev.x) * viewScale,
          y: pos.y + (viewFrame.y - prev.y) * viewScale,
        },
        { width: size.width * zoom, height: size.height * zoom },
        viewport,
      ));
      return;
    }
    refit();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [viewFrame.x, viewFrame.y, viewFrame.width, viewFrame.height]);

  // An image edit that left the frame unchanged must not carry the flag over
  // to a later frame change (undo, add), which has to refit.
  useLayoutEffect(() => {
    keepViewRef.current = false;
  });

  // The listener exists from mount, so a resize that fires while images are
  // still decoding (Windows maximises the window right after a capture) is not
  // missed.
  useEffect(() => {
    window.addEventListener('resize', refit);
    return () => {
      window.removeEventListener('resize', refit);
    };
  }, [refit]);

  // Create blur pattern image using canvas (simple diagonal stripes)
  useEffect(() => {
    // Create a small canvas for the pattern
    const canvas = document.createElement('canvas');
    canvas.width = 20;
    canvas.height = 20;
    const ctx = canvas.getContext('2d');

    if (ctx) {
      // Fill with dark gray background
      ctx.fillStyle = '#1a1a1a';
      ctx.fillRect(0, 0, 20, 20);

      // Draw diagonal stripes
      ctx.strokeStyle = '#2a2a2a';
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(0, 0);
      ctx.lineTo(20, 20);
      ctx.moveTo(0, 20);
      ctx.lineTo(20, 0);
      ctx.stroke();

      // Convert canvas to image
      const img = new window.Image();
      img.src = canvas.toDataURL();
      img.onload = () => {
        setBlurPatternImage(img);
      };
    }
  }, []);

  // Put the stage canvas on the device pixel grid; together with the
  // device-aligned viewport size this keeps the canvas unscaled on screen. The
  // stage only mounts once the images are decoded, hence `ready`.
  useLayoutEffect(() => {
    const container = stageRef.current?.container();
    if (container) alignToDevicePixels(container);
  }, [viewport, ready]);

  // Handle scroll: Cmd/Ctrl + Scroll to zoom, regular scroll to pan when zoomed
  useEffect(() => {
    const handleWheel = (e: Event) => {
      const wheelEvent = e as WheelEvent;
      const stage = stageRef.current;
      if (!stage) return;

      // Check if Cmd (Mac) or Ctrl (Windows/Linux) is pressed for zoom
      if (wheelEvent.metaKey || wheelEvent.ctrlKey) {
        wheelEvent.preventDefault();

        const oldZoom = zoom;
        const pointer = stage.getPointerPosition();

        if (!pointer) return;

        // Calculate zoom delta
        const scaleBy = 1.05;
        const direction = wheelEvent.deltaY > 0 ? -1 : 1;
        const newZoom = direction > 0
          ? Math.min(oldZoom * scaleBy, 3)
          : Math.max(oldZoom / scaleBy, 0.5);

        // Keep the content inside the viewport: centred when it fits, clamped
        // to the edges once zoom pushes it past the viewport.
        const content = {
          width: stageSize.width * newZoom,
          height: stageSize.height * newZoom,
        };

        if (newZoom <= 1) {
          const centered = clampPosition({ x: 0, y: 0 }, content, viewport);
          setZoom(newZoom);
          setStagePos(centered);
          return;
        }

        // Calculate new position to zoom towards mouse pointer
        const mousePointTo = {
          x: (pointer.x - stagePos.x) / oldZoom,
          y: (pointer.y - stagePos.y) / oldZoom,
        };

        const newPos = {
          x: pointer.x - mousePointTo.x * newZoom,
          y: pointer.y - mousePointTo.y * newZoom,
        };

        setZoom(newZoom);
        setStagePos(clampPosition(newPos, content, viewport));
      } else if (zoom > 1) {
        // Regular scroll to pan when zoomed in
        wheelEvent.preventDefault();

        // Shift + scroll => horizontal pan. The webview doesn't remap the
        // vertical wheel delta to deltaX, so do it ourselves: treat deltaY
        // as horizontal movement when Shift is held.
        const horizontalDelta = wheelEvent.shiftKey
          ? wheelEvent.deltaX || wheelEvent.deltaY
          : wheelEvent.deltaX;
        const verticalDelta = wheelEvent.shiftKey ? 0 : wheelEvent.deltaY;

        const content = {
          width: stageSize.width * zoom,
          height: stageSize.height * zoom,
        };

        setStagePos(clampPosition({
          x: stagePos.x - horizontalDelta,
          y: stagePos.y - verticalDelta,
        }, content, viewport));
      }
    };

    const canvasContainer = document.querySelector('.editor-canvas');
    if (canvasContainer) {
      canvasContainer.addEventListener('wheel', handleWheel as EventListener, { passive: false });

      return () => {
        canvasContainer.removeEventListener('wheel', handleWheel as EventListener);
      };
    }
  }, [zoom, stagePos, stageSize, viewport, ready]);

  // Center image when zoom <= 1 (from ZoomBar or any source)
  // Keep the image inside the viewport whenever zoom or the viewport changes
  // (ZoomBar, Fit / Real Size buttons, window resize).
  useEffect(() => {
    const content = {
      width: stageSize.width * zoom,
      height: stageSize.height * zoom,
    };
    setStagePos((prev) => {
      const next = clampPosition(prev, content, viewport);
      return prev.x === next.x && prev.y === next.y ? prev : next;
    });
    // Not on stageSize: refit positions the view itself, and an image move or
    // resize keeps the view where it is (see keepViewRef).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [zoom, viewport]);

  // Clear marquee multi-selection when switching tools
  useEffect(() => {
    setMarqueeSelectedIds([]);
    setSelectionRect(null);
  }, [selectedTool]);

  // Attach transformer to selected annotation
  useEffect(() => {
    if (!transformerRef.current) return;

    // No handles while cropping: the image is the crop target, not a thing to resize.
    if (selectedImageId && imagesSelectable && selectedTool !== 'crop' && imageNodeRefs.current[selectedImageId]) {
      transformerRef.current.nodes([imageNodeRefs.current[selectedImageId]]);
      transformerRef.current.getLayer().batchDraw();
    // Don't attach while inline-editing a text box (its node is unmounted).
    } else if (selectedAnnotationId && !editingTextId && shapeRefs.current[selectedAnnotationId]) {
      const node = shapeRefs.current[selectedAnnotationId];
      transformerRef.current.nodes([node]);
      transformerRef.current.getLayer().batchDraw();
    } else {
      transformerRef.current.nodes([]);
      transformerRef.current.getLayer()?.batchDraw();
    }
  }, [selectedAnnotationId, annotations, editingTextId, selectedImageId, images, imagesSelectable, loadedCount, selectedTool]);



  // Track the pan modifiers: Cmd/Ctrl pans from the background, Shift is a
  // temporary hand tool that pans from anywhere with any tool.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey) {
        setIsModifierPressed(true);
      }
      if (e.shiftKey) {
        setIsShiftPressed(true);
      }
    };
    const handleKeyUp = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) {
        setIsModifierPressed(false);
      }
      if (!e.shiftKey) {
        setIsShiftPressed(false);
      }
    };
    // Also reset when window loses focus
    const handleBlur = () => {
      setIsModifierPressed(false);
      setIsShiftPressed(false);
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('keyup', handleKeyUp);
    window.addEventListener('blur', handleBlur);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
      window.removeEventListener('blur', handleBlur);
    };
  }, []);

  // Update cursor when zoom, tool, or modifier changes
  useEffect(() => {
    if (stageRef.current) {
      const container = stageRef.current.container();

      if (panCursor) {
        // Grab cursor only while a pan modifier is held and there is room to pan
        container.style.cursor = 'grab';
      } else if (selectedTool === 'crop') {
        container.style.cursor = 'crosshair';
      } else {
        container.style.cursor = 'default';
      }
    }
  }, [panCursor, selectedTool]);

  const pasteImage = useCallback(async () => {
    try {
      const data = await ReadClipboardImage();
      await addImage(data);
    } catch (error) {
      console.error('Failed to paste image:', error);
      setToast({ message: `Paste failed: ${errorText(error)}`, type: 'error' });
    }
  }, [addImage]);

  // Dropped image files become new images, the first with its top-left at the
  // drop point, each next one to the right of the previous with the image gap.
  useEffect(() => {
    const handleDrop = async (x: number, y: number, paths: string[]) => {
      const files = paths.filter(isImagePath);
      const skipped = paths.length - files.length;

      // Window coordinates -> doc px: undo the container offset and the pan,
      // divide by the view scale, add the frame origin.
      const stage = stageRef.current;
      let cursor: { x: number; y: number } | undefined;
      if (stage) {
        const rect = stage.container().getBoundingClientRect();
        const view = viewRef.current;
        cursor = {
          x: (x - rect.left - view.stagePos.x) / view.viewScale + view.frame.x,
          y: (y - rect.top - view.stagePos.y) / view.viewScale + view.frame.y,
        };
      }

      let failure = '';
      for (const path of files) {
        try {
          const data = await ReadImageFile(path);
          const placed = await addImage(data, cursor ? { at: cursor } : undefined);
          if (!placed) return; // the shot was replaced meanwhile
          if (cursor) cursor = { x: placed.x + placed.width + useEditorStore.getState().imageGap, y: placed.y };
        } catch (error) {
          console.error('Failed to add dropped image:', path, error);
          failure = errorText(error);
        }
      }

      const notes: string[] = [];
      if (failure) notes.push(`Failed to add an image: ${failure}`);
      if (skipped > 0) notes.push(`Skipped ${skipped} file${skipped === 1 ? '' : 's'} that ${skipped === 1 ? 'is' : 'are'} not an image`);
      if (notes.length) setToast({ message: notes.join('. '), type: failure ? 'error' : 'info' });
    };

    OnFileDrop((x: number, y: number, paths: string[]) => {
      void handleDrop(x, y, paths);
    }, false);
    return () => {
      OnFileDropOff();
    };
  }, [addImage]);

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if typing in a text input / inline text box
      if (textInput || editingTextId) return;
      // A dialog (Settings, Record) is open: its fields get the keys.
      if (document.querySelector('.settings-window')) return;

      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const cmdOrCtrl = isMac ? e.metaKey : e.ctrlKey;

      // Add shot: Escape keeps the capture as it is (also with no region drawn).
      if (pendingCrop && e.key === 'Escape') {
        e.preventDefault();
        keepPendingCrop();
        return;
      }

      // Handle crop mode shortcuts
      if (cropRegion) {
        if (e.key === 'Enter') {
          e.preventDefault();
          // Apply crop
          applyCrop();
          return;
        } else if (e.key === 'Escape') {
          e.preventDefault();
          // Cancel crop
          setCropRegion(null);
          return;
        }
      }

      // Handle Cmd/Ctrl shortcuts
      if (cmdOrCtrl) {
        const key = e.key.toLowerCase();

        // Cmd+Z / Ctrl+Z - Undo
        if (key === 'z' && !e.shiftKey) {
          e.preventDefault();
          undo();
          return;
        }

        // Cmd+Shift+Z / Ctrl+Shift+Z - Redo
        if (key === 'z' && e.shiftKey) {
          e.preventDefault();
          redo();
          return;
        }

        // Cmd+1-5 / Ctrl+1-5 - Size shortcuts
        if (key >= '1' && key <= '5') {
          e.preventDefault();
          setSelectedSize(parseInt(key));
          return;
        }

        // Cmd+V / Ctrl+V - Paste an image from the clipboard as a new image.
        // Left alone while the focus is in a field, where it pastes text.
        if (key === 'v' && !e.shiftKey) {
          const el = e.target as HTMLElement | null;
          if (el?.closest?.('input, textarea, select, [contenteditable="true"]')) return;
          e.preventDefault();
          if (!e.repeat) void pasteImage();
          return;
        }

        // Cmd+A / Ctrl+A - Select all (select tool)
        if (key === 'a') {
          e.preventDefault();
          setSelectedTool('select');
          setSelectedAnnotation(null);
          setMarqueeSelectedIds(annotations.map((ann) => ann.id));
          return;
        }

        // Cmd+D / Ctrl+D - Delete selected
        if (key === 'd') {
          e.preventDefault();
          const ids = Array.from(new Set([
            ...(selectedAnnotationId ? [selectedAnnotationId] : []),
            ...marqueeSelectedIds,
          ]));
          if (ids.length) {
            deleteAnnotations(ids);
            setSelectedAnnotation(null);
            setMarqueeSelectedIds([]);
          } else if (selectedImageId) {
            removeImage(selectedImageId);
          }
          return;
        }

        // Note: Cmd+S, Cmd+Shift+S, Cmd+C, Cmd+Shift+C are handled in ActionBar
        // We don't preventDefault here to let ActionBar handle them
      }

      // Delete or Backspace - Delete selected annotation(s) (without Cmd/Ctrl)
      if (!cmdOrCtrl && (e.key === 'Delete' || e.key === 'Backspace')) {
        const ids = Array.from(new Set([
          ...(selectedAnnotationId ? [selectedAnnotationId] : []),
          ...marqueeSelectedIds,
        ]));
        if (ids.length) {
          e.preventDefault();
          deleteAnnotations(ids);
          setSelectedAnnotation(null);
          setMarqueeSelectedIds([]);
          return;
        }
        if (selectedImageId) {
          e.preventDefault();
          removeImage(selectedImageId);
          return;
        }
      }

      // Handle tool shortcuts (only when not in crop mode and no Cmd/Ctrl)
      if (!cropRegion && !cmdOrCtrl) {
        const key = e.key.toLowerCase();

        // Size shortcuts
        if (key === '[') {
          e.preventDefault();
          setSelectedSize(Math.max(1, selectedSize - 1));
          return;
        } else if (key === ']') {
          e.preventDefault();
          setSelectedSize(Math.min(5, selectedSize + 1));
          return;
        }

        switch (key) {
          case 's':
            e.preventDefault();
            setSelectedTool('select');
            break;
          case 'c':
            e.preventDefault();
            setSelectedTool('crop');
            break;
          case 'r':
            e.preventDefault();
            setSelectedTool('rectangle');
            break;
          case 'a':
            e.preventDefault();
            setSelectedTool('arrow');
            break;
          case 'n':
            e.preventDefault();
            setSelectedTool('numbered-arrow');
            break;
          case 'w':
            e.preventDefault();
            setSelectedTool('arrow-text');
            break;
          case 't':
            e.preventDefault();
            setSelectedTool('text');
            break;
          case 'h':
            e.preventDefault();
            setSelectedTool('highlight');
            break;
          case 'b':
            e.preventDefault();
            setSelectedTool('blur');
            break;
          case 'p':
            e.preventDefault();
            setSelectedTool('pen');
            break;
          case 'm':
            e.preventDefault();
            setSelectedTool('marker');
            break;
          case 'l':
            e.preventDefault();
            setSelectedTool('line');
            break;
          case 'e':
            e.preventDefault();
            setSelectedTool('ellipse');
            break;
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [cropRegion, pendingCrop, keepPendingCrop, pasteImage, textInput, editingTextId, annotations, marqueeSelectedIds, applyCrop, setCropRegion, setSelectedTool, selectedSize, setSelectedSize, undo, redo, selectedAnnotationId, deleteAnnotation, deleteAnnotations, setSelectedAnnotation, selectedImageId, removeImage]);

  // Pointer position in doc px: undo the pan and the view scale, then add the
  // layer offset (the frame origin sits at the stage origin).
  const getAdjustedPointerPosition = (stage: any) => {
    const pointer = stage.getPointerPosition();
    if (!pointer) return { x: 0, y: 0 };
    return {
      x: (pointer.x - stagePos.x) / viewScale + viewFrame.x,
      y: (pointer.y - stagePos.y) / viewScale + viewFrame.y,
    };
  };

  // Holds a point inside the crop target's box (no-op for a canvas crop).
  const clampToCropTarget = (p: { x: number; y: number }) =>
    cropTarget
      ? {
          x: Math.min(cropTarget.x + cropTarget.width, Math.max(cropTarget.x, p.x)),
          y: Math.min(cropTarget.y + cropTarget.height, Math.max(cropTarget.y, p.y)),
        }
      : p;

  const startPan = (stage: any) => {
    const pointer = stage.getPointerPosition();
    if (!pointer) return;
    panStartRef.current = { pointer: { x: pointer.x, y: pointer.y }, pos: { ...stagePos } };
    setIsPanning(true);
    stage.container().style.cursor = 'grabbing';
  };

  const handleMouseDown = (e: any) => {
    const stage = e.target.getStage();
    const clickedOnStage = e.target === stage;
    const clickedOnBackground = clickedOnStage || e.target.hasName(BACKGROUND_NAME);

    // Clicking a Transformer handle (resize/rotate anchor) must only transform
    // the selected object — never start drawing a new annotation or crop region.
    let ancestor = e.target;
    while (ancestor) {
      if (typeof ancestor.getClassName === 'function' && ancestor.getClassName() === 'Transformer') {
        return;
      }
      ancestor = ancestor.getParent && ancestor.getParent();
    }

    // Shift = temporary hand tool: pan from anywhere, before any tool acts
    if (shiftPanning) {
      startPan(stage);
      return;
    }

    // For crop tool, start drawing crop region (priority over Cmd/Ctrl panning)
    if (selectedTool === 'crop') {
      const pos = clampToCropTarget(getAdjustedPointerPosition(stage));
      setIsDrawing(true);
      setCropRegion({
        x: pos.x,
        y: pos.y,
        width: 0,
        height: 0,
      });
      return;
    }

    // Cmd/Ctrl + drag on the background pans when the image overflows the editor
    if (isModifierPressed && panAvailable && clickedOnBackground) {
      startPan(stage);
      return;
    }

    // For select tool, only handle clicks on annotations (handled in renderAnnotation)
    if (selectedTool === 'select') {
      if (clickedOnBackground) {
        // Start a marquee selection; clear any current selection.
        setSelectedAnnotation(null);
        setSelectedImage(null);
        setMarqueeSelectedIds([]);
        const pos = getAdjustedPointerPosition(stage);
        setSelectionRect({ x: pos.x, y: pos.y, width: 0, height: 0 });
        setIsDrawing(true);
      } else {
        // Clicked an annotation: its onClick does the single-select; drop marquee.
        setMarqueeSelectedIds([]);
      }
      return;
    }

    const pos = getAdjustedPointerPosition(stage);

    // Check if clicking on an existing annotation of the same type
    // If yes, select it instead of creating new one. Freehand tools always
    // draw: a stroke across an earlier one must not select it.
    if (e.target !== stage && !isFreehand(selectedTool)) {
      // Find the annotation that was clicked
      const clickedAnnotation = annotations.find(ann => {
        const shape = shapeRefs.current[ann.id];
        if (!shape) return false;

        // Check if the clicked target is this shape or its child
        let target = e.target;
        while (target) {
          // Check if target is the shape itself
          if (target === shape) {
            // Check if annotation type matches current tool
            if (ann.type === selectedTool) {
              return true;
            }
            // For arrow-text and numbered-arrow, also match with 'arrow' tool
            if (selectedTool === 'arrow' && (ann.type === 'arrow-text' || ann.type === 'numbered-arrow')) {
              return true;
            }
            break;
          }
          // Check if target is a child of this shape (for Groups)
          if (target.parent === shape) {
            // Check if annotation type matches current tool
            if (ann.type === selectedTool) {
              return true;
            }
            // For arrow-text and numbered-arrow, also match with 'arrow' tool
            if (selectedTool === 'arrow' && (ann.type === 'arrow-text' || ann.type === 'numbered-arrow')) {
              return true;
            }
            break;
          }
          target = target.parent;
        }
        return false;
      });

      if (clickedAnnotation) {
        // Select the annotation instead of creating new one
        setSelectedAnnotation(clickedAnnotation.id);
        return;
      }
    }

    setIsDrawing(true);

    const id = `annotation-${Date.now()}`;
    const baseAnnotation: Annotation = {
      id,
      type: selectedTool,
      x: pos.x,
      y: pos.y,
      unit: px,
    };

    // Calculate actual size based on selectedSize. The presets are screen
    // sizes; dividing by scaleRatio (times px) turns them into doc px so they
    // look the same on screen as before.
    const strokeWidth = strokeForSize(selectedSize) * px;
    const fontSize = fontForSize(selectedSize) * px;

    switch (selectedTool) {
      case 'rectangle':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          stroke: selectedColor,
          strokeWidth,
          fill: 'transparent',
          cornerRadius: rectangleRadius,
        });
        break;
      case 'ellipse':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          stroke: selectedColor,
          strokeWidth,
          fill: 'transparent',
        });
        break;
      case 'line':
      case 'pen':
        setCurrentAnnotation({
          ...baseAnnotation,
          points: [pos.x, pos.y, pos.x, pos.y],
          stroke: selectedColor,
          strokeWidth,
        });
        break;
      case 'marker':
        setCurrentAnnotation({
          ...baseAnnotation,
          points: [pos.x, pos.y, pos.x, pos.y],
          stroke: selectedColor,
          strokeWidth: markerForSize(selectedSize) * px,
          opacity: MARKER_OPACITY,
        });
        break;
      case 'highlight':
        // A spotlight box: drawSpotlight dims everything around it.
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          cornerRadius: highlightOptions.radius,
        });
        break;
      case 'blur':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          fill: '#ffffff',
          opacity: 1,
          blurMode,
        });
        break;
      case 'arrow':
        setCurrentAnnotation({
          ...baseAnnotation,
          points: [pos.x, pos.y, pos.x, pos.y],
          stroke: selectedColor,
          strokeWidth,
          curvature: 0, // Start with straight arrow, can be curved by dragging control point
        });
        break;
      case 'arrow-text':
        setCurrentAnnotation({
          ...baseAnnotation,
          points: [pos.x, pos.y, pos.x, pos.y],
          stroke: selectedColor,
          strokeWidth,
          text: '', // Will be set after drawing
          curvature: 0,
        });
        break;
      case 'numbered-arrow':
        // Auto-increment number based on existing numbered arrows
        const existingNumbers = annotations
          .filter(a => a.type === 'numbered-arrow')
          .map(a => a.number || 0);
        const nextNumber = existingNumbers.length > 0 ? Math.max(...existingNumbers) + 1 : 1;

        setCurrentAnnotation({
          ...baseAnnotation,
          points: [pos.x, pos.y, pos.x, pos.y],
          stroke: selectedColor,
          strokeWidth,
          fill: selectedColor,
          number: nextNumber,
        });
        break;
      case 'text':
        // Text is now drawn as a box; the text is typed inline afterwards.
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          text: '',
          fontSize,
          fill: selectedColor,
        });
        break;
    }
  };

  const handleMouseMove = (e: any) => {
    const stage = e.target.getStage();

    // Handle panning
    if (isPanning) {
      const start = panStartRef.current;
      const pointer = stage.getPointerPosition();
      if (!start || !pointer) return;
      const content = {
        width: stageSize.width * zoom,
        height: stageSize.height * zoom,
      };
      setStagePos(clampPosition({
        x: start.pos.x + pointer.x - start.pointer.x,
        y: start.pos.y + pointer.y - start.pointer.y,
      }, content, viewport));
      return;
    }

    if (!isDrawing) return;

    // Update marquee selection rectangle
    if (selectionRect) {
      const mpos = getAdjustedPointerPosition(stage);
      setSelectionRect({
        ...selectionRect,
        width: mpos.x - selectionRect.x,
        height: mpos.y - selectionRect.y,
      });
      return;
    }

    const pos = getAdjustedPointerPosition(stage);

    // Handle crop region drawing
    if (selectedTool === 'crop' && cropRegion) {
      const end = clampToCropTarget(pos);
      setCropRegion({
        ...cropRegion,
        width: end.x - cropRegion.x,
        height: end.y - cropRegion.y,
      });
      return;
    }

    if (!currentAnnotation) return;

    if (selectedTool === 'rectangle' || selectedTool === 'ellipse' || selectedTool === 'highlight' || selectedTool === 'blur' || selectedTool === 'text') {
      setCurrentAnnotation({
        ...currentAnnotation,
        width: pos.x - currentAnnotation.x,
        height: pos.y - currentAnnotation.y,
      });
    } else if (selectedTool === 'arrow' || selectedTool === 'arrow-text' || selectedTool === 'numbered-arrow' || selectedTool === 'line') {
      setCurrentAnnotation({
        ...currentAnnotation,
        points: [currentAnnotation.x, currentAnnotation.y, pos.x, pos.y],
      });
    } else if (isFreehand(selectedTool) && currentAnnotation.points) {
      // Add a point once the pointer has moved a pixel: hand jitter below that
      // only bloats the stroke.
      const pts = currentAnnotation.points;
      const lastX = pts[pts.length - 2];
      const lastY = pts[pts.length - 1];
      if (Math.hypot(pos.x - lastX, pos.y - lastY) >= px) {
        setCurrentAnnotation({ ...currentAnnotation, points: [...pts, pos.x, pos.y] });
      }
    }
  };

  const handleMouseUp = () => {
    // Handle panning end
    if (isPanning) {
      setIsPanning(false);
      panStartRef.current = null;
      if (stageRef.current) {
        const container = stageRef.current.container();
        // Restore cursor based on modifier/tool/zoom
        if (panCursor) {
          container.style.cursor = 'grab';
        } else if (selectedTool === 'crop') {
          container.style.cursor = 'crosshair';
        } else {
          container.style.cursor = 'default';
        }
      }
      return;
    }

    if (!isDrawing) return;

    // Finalize marquee selection: select every annotation the box overlaps.
    if (selectionRect) {
      const rx = selectionRect.width < 0 ? selectionRect.x + selectionRect.width : selectionRect.x;
      const ry = selectionRect.height < 0 ? selectionRect.y + selectionRect.height : selectionRect.y;
      const rw = Math.abs(selectionRect.width);
      const rh = Math.abs(selectionRect.height);

      if (rw > 3 * px && rh > 3 * px) {
        const ids = annotations
          .filter((ann) => {
            const node = shapeRefs.current[ann.id];
            if (!node) return false;
            const box = node.getClientRect({ relativeTo: node.getLayer() });
            // AABB overlap test
            return !(
              box.x > rx + rw ||
              box.x + box.width < rx ||
              box.y > ry + rh ||
              box.y + box.height < ry
            );
          })
          .map((ann) => ann.id);
        setMarqueeSelectedIds(ids);
      }

      setSelectionRect(null);
      setIsDrawing(false);
      return;
    }

    // Handle crop region completion
    if (selectedTool === 'crop' && cropRegion) {
      setIsDrawing(false);
      // Normalize crop region (handle negative width/height)
      const normalizedRegion = {
        x: cropRegion.width < 0 ? cropRegion.x + cropRegion.width : cropRegion.x,
        y: cropRegion.height < 0 ? cropRegion.y + cropRegion.height : cropRegion.y,
        width: Math.abs(cropRegion.width),
        height: Math.abs(cropRegion.height),
      };

      if (normalizedRegion.width > 10 * px && normalizedRegion.height > 10 * px) {
        // Keep the normalized region visible
        setCropRegion(normalizedRegion);
      } else {
        setCropRegion(null);
      }
      return;
    }

    if (!currentAnnotation) return;

    // Text tool: finalize the box, then start inline editing immediately.
    if (selectedTool === 'text') {
      const minW = 120 * px;
      const minH = (currentAnnotation.fontSize || 16 * px) + 12 * px;
      let x = currentAnnotation.x;
      let y = currentAnnotation.y;
      let w = currentAnnotation.width || 0;
      let h = currentAnnotation.height || 0;
      // Normalize negative drag direction
      if (w < 0) { x += w; w = -w; }
      if (h < 0) { y += h; h = -h; }
      // A plain click (no drag) or tiny box gets a sensible default size
      if (w < minW) w = minW;
      if (h < minH) h = minH;

      const textAnn = { ...currentAnnotation, x, y, width: w, height: h, text: '' };
      addAnnotation(textAnn);
      setEditingTextId(textAnn.id);
      setSelectedAnnotation(textAnn.id);
      setSelectedTool('select');
      setIsDrawing(false);
      setCurrentAnnotation(null);
      setTimeout(() => textEditRef.current?.focus(), 0);
      return;
    }

    let annToAdd = currentAnnotation;

    // Arrows: enforce a minimum length so a click (no drag) still produces a
    // proper arrow with a visible line, not just the arrowhead. A zero-length
    // arrow also breaks the curve math (division by zero) when selected.
    if (
      (selectedTool === 'arrow' || selectedTool === 'arrow-text' || selectedTool === 'numbered-arrow') &&
      annToAdd.points
    ) {
      const [sx, sy, ex, ey] = annToAdd.points;
      const dist = Math.hypot(ex - sx, ey - sy);
      const MIN_LEN = 60 * px;
      if (dist < MIN_LEN) {
        // Extend along the drag direction; default to rightward on a pure click.
        let ux = ex - sx;
        let uy = ey - sy;
        if (dist < 1) {
          ux = 1;
          uy = 0;
        } else {
          ux /= dist;
          uy /= dist;
        }
        annToAdd = { ...annToAdd, points: [sx, sy, sx + ux * MIN_LEN, sy + uy * MIN_LEN] };
      }
    }

    // Freehand strokes keep their shape with far fewer points.
    if (isFreehand(selectedTool) && annToAdd.points) {
      annToAdd = { ...annToAdd, points: simplifyPoints(annToAdd.points, 0.8 * px) };
    }
    // A click without a drag is no line.
    let discard = false;
    if (selectedTool === 'line' && annToAdd.points) {
      const [sx, sy, ex, ey] = annToAdd.points;
      discard = Math.hypot(ex - sx, ey - sy) < 3 * px;
    }
    // Boxes of the ellipse and blur tools store their top-left corner and a
    // positive size, so a move or resize starts from the box the node shows.
    if ((selectedTool === 'ellipse' || selectedTool === 'blur') && annToAdd.width !== undefined && annToAdd.height !== undefined) {
      const bw = annToAdd.width;
      const bh = annToAdd.height;
      annToAdd = {
        ...annToAdd,
        x: bw < 0 ? annToAdd.x + bw : annToAdd.x,
        y: bh < 0 ? annToAdd.y + bh : annToAdd.y,
        width: Math.abs(bw),
        height: Math.abs(bh),
      };
    }

    if (!discard && (annToAdd.width !== 0 || annToAdd.height !== 0 || annToAdd.points)) {
      // For arrow-text, start inline editing of the label (same UX as the text tool)
      if (selectedTool === 'arrow-text' && annToAdd.points) {
        const tempAnnotation = { ...annToAdd, fontSize: annToAdd.fontSize || 16 * px };
        addAnnotation(tempAnnotation);
        setSelectedAnnotation(tempAnnotation.id);
        setEditingTextId(tempAnnotation.id);
        setTimeout(() => textEditRef.current?.focus(), 0);
      } else {
        addAnnotation(annToAdd);
      }
    }

    setIsDrawing(false);
    setCurrentAnnotation(null);
  };

  const handleTextSubmit = () => {
    if (!textInput) {
      return;
    }

    // Check if this is for arrow-text annotation
    if ((textInput as any).annotationId) {
      const annotationId = (textInput as any).annotationId;
      if (textValue.trim()) {
        updateAnnotation(annotationId, { text: textValue });
      }
      setTextInput(null);
      setTextValue('');
      return;
    }

    // Regular text annotation
    if (!textValue.trim()) {
      setTextInput(null);
      setTextValue('');
      return;
    }

    const id = `annotation-${Date.now()}`;
    addAnnotation({
      id,
      type: 'text',
      x: textInput.x,
      y: textInput.y,
      text: textValue,
      fontSize: textInput.fontSize || 16 * px,
      fill: selectedColor,
      unit: px,
    });

    setTextInput(null);
    setTextValue('');
  };

  const handleTextKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleTextSubmit();
    } else if (e.key === 'Escape') {
      setTextInput(null);
      setTextValue('');
    }
  };

  // Position of the arrow-text label: just above the arrow's start point (its
  // tail). Stable (independent of text/direction) so it doesn't jump while
  // typing and matches between the inline editor and the rendered text.
  const arrowTextLabelPos = (points: number[], _text: string, fontSize: number, unit: number) => {
    const sx = points[0] || 0;
    const sy = points[1] || 0;
    const gap = 6 * unit;
    return { x: sx, y: sy - fontSize - gap };
  };

  // Finish inline text editing. A plain text box left empty is removed; an
  // arrow-text label left empty just stays textless (its arrow remains).
  const commitTextEditing = () => {
    const id = editingTextId;
    setEditingTextId(null);
    if (!id) return;
    const ann = annotations.find((a) => a.id === id);
    if (ann && ann.type === 'text' && !(ann.text || '').trim()) {
      deleteAnnotation(id);
      setSelectedAnnotation(null);
    }
  };

  const handleTextEditKeyDown = (e: React.KeyboardEvent) => {
    // Enter adds a new line (multi-line box). Escape / blur commit.
    if (e.key === 'Escape') {
      e.preventDefault();
      commitTextEditing();
    }
  };

  // Helper function to calculate curved arrow path
  const getCurvedArrowPath = (points: number[], curvature: number = 0.3): number[] => {
    if (points.length < 4) return points;

    const [x1, y1, x2, y2] = points;

    // Calculate midpoint
    const midX = (x1 + x2) / 2;
    const midY = (y1 + y2) / 2;

    // Calculate perpendicular offset for curve
    const dx = x2 - x1;
    const dy = y2 - y1;
    const distance = Math.sqrt(dx * dx + dy * dy);

    // Degenerate (zero-length) arrow: no curve, avoid division by zero / NaN.
    if (distance < 0.0001) return [x1, y1, x2, y2];

    // Control point offset (perpendicular to line)
    const offsetX = -dy / distance * distance * curvature;
    const offsetY = dx / distance * distance * curvature;

    const controlX = midX + offsetX;
    const controlY = midY + offsetY;

    // Generate bezier curve points
    const curvePoints: number[] = [];
    const steps = 20;

    for (let i = 0; i <= steps; i++) {
      const t = i / steps;
      const invT = 1 - t;

      // Quadratic bezier formula
      const x = invT * invT * x1 + 2 * invT * t * controlX + t * t * x2;
      const y = invT * invT * y1 + 2 * invT * t * controlY + t * t * y2;

      curvePoints.push(x, y);
    }

    return curvePoints;
  };

  const handleDragEnd = (e: any, annId: string) => {
    const node = e.target;
    const ann = annotations.find((a) => a.id === annId);
    if (!ann) return;

    // For annotations with points (arrows, lines, strokes), update points instead of x/y
    if (hasPoints(ann)) {
      const deltaX = node.x();
      const deltaY = node.y();

      // Only update if there's actual movement
      if (deltaX !== 0 || deltaY !== 0) {
        const newPoints = ann.points.map((p, i) => i % 2 === 0 ? p + deltaX : p + deltaY);

        updateAnnotation(annId, {
          points: newPoints,
        });
      }

      // Reset node position after update
      node.position({ x: 0, y: 0 });
    } else {
      // For other annotations, update x/y
      updateAnnotation(annId, {
        x: node.x(),
        y: node.y(),
      });
    }
  };

  const handleTransformEnd = (e: any, annId: string) => {
    const node = e.target;
    const ann = annotations.find((a) => a.id === annId);
    if (!ann) return;

    const scaleX = node.scaleX();
    const scaleY = node.scaleY();

    // Point-based annotations (arrows, lines, strokes): bake the transformer's
    // scale/rotation/position into the points so the resize/rotate sticks, then
    // reset the node.
    if (hasPoints(ann)) {
      const transform = node.getTransform().copy();
      const newPoints: number[] = [];
      for (let i = 0; i < ann.points.length; i += 2) {
        const p = transform.point({ x: ann.points[i], y: ann.points[i + 1] });
        newPoints.push(p.x, p.y);
      }
      node.scaleX(1);
      node.scaleY(1);
      node.rotation(0);
      node.position({ x: 0, y: 0 });
      updateAnnotation(annId, { points: newPoints });
      return;
    }

    // Reset scale for box-based annotations
    node.scaleX(1);
    node.scaleY(1);

    // Update annotation with new dimensions (and rotation from the transformer)
    if (ann.type === 'rectangle' || ann.type === 'ellipse' || ann.type === 'highlight' || ann.type === 'blur') {
      updateAnnotation(annId, {
        x: node.x(),
        y: node.y(),
        width: Math.max(5 * px, (ann.width || 0) * scaleX),
        height: Math.max(5 * px, (ann.height || 0) * scaleY),
        rotation: node.rotation(),
      });
    } else if (ann.type === 'text') {
      // Resize scales the font with the vertical scale (corner/vertical drag
      // grows the text); horizontal-only drag just changes the wrap width.
      const newFontSize = Math.max(8 * px, (ann.fontSize || 16 * px) * scaleY);
      updateAnnotation(annId, {
        x: node.x(),
        y: node.y(),
        width: Math.max(20 * px, (ann.width || 0) * scaleX),
        height: Math.max(newFontSize + 8 * px, (ann.height || 0) * scaleY),
        fontSize: newFontSize,
        rotation: node.rotation(),
      });
    }
  };

  const handleAnnotationMouseEnter = (annId: string) => {
    setHoveredAnnotationId(annId);
    const stage = stageRef.current;
    // While Shift-panning the cursor stays a hand: dragging moves the view.
    if (stage && !shiftPanning && (selectedTool === 'select' || annId === selectedAnnotationId)) {
      stage.container().style.cursor = 'move';
    }
  };

  const handleAnnotationMouseLeave = () => {
    setHoveredAnnotationId(null);
    const stage = stageRef.current;
    if (stage && !isPanning) {
      // Restore the editor's base cursor (the cursor effect re-applies grab/
      // crosshair states on the next tool/zoom/modifier change).
      stage.container().style.cursor = panCursor ? 'grab' : selectedTool === 'crop' ? 'crosshair' : 'default';
    }
  };

  // Images are objects only with the select tool; with a drawing tool a click
  // on an image starts an annotation, as on a single-image shot.
  const imageInteractive = imagesSelectable && selectedTool === 'select' && !shiftPanning;

  const handleImageMouseDown = (id: string) => {
    if (imageInteractive) {
      setMarqueeSelectedIds([]);
      setSelectedImage(id);
    }
  };

  const handleImageMouseEnter = () => {
    const stage = stageRef.current;
    if (stage && imageInteractive) stage.container().style.cursor = 'move';
  };

  const handleImageMouseLeave = () => {
    const stage = stageRef.current;
    if (stage && !isPanning) {
      stage.container().style.cursor = panCursor ? 'grab' : selectedTool === 'crop' ? 'crosshair' : 'default';
    }
  };

  // A move commits on drag end only, so one drag is one undo step.
  const handleImageDragEnd = (e: any, id: string) => {
    const node = e.target;
    const x = Math.round(node.x());
    const y = Math.round(node.y());
    node.position({ x, y });
    keepViewRef.current = true;
    updateImage(id, { x, y });
  };

  // The transformer scales the node; bake the scale into the size (keeping the
  // aspect ratio) and reset it, then commit once.
  const handleImageTransformEnd = (e: any, id: string) => {
    const node = e.target;
    const im = images.find((i) => i.id === id);
    if (!im) return;
    const width = Math.max(1, Math.round(im.width * node.scaleX()));
    const height = Math.max(1, Math.round((width * im.height) / im.width));
    const x = Math.round(node.x());
    const y = Math.round(node.y());
    node.scaleX(1);
    node.scaleY(1);
    node.position({ x, y });
    node.size({ width, height });
    keepViewRef.current = true;
    updateImage(id, { x, y, width, height });
  };

  const renderAnnotation = (ann: Annotation) => {
    const isSelected = ann.id === selectedAnnotationId || marqueeSelectedIds.includes(ann.id);
    // Fixed visual sizes the annotation owns (arrow head, numbered circle,
    // label gap, blur tile) scale with its creation-time unit; editor chrome
    // (selection outline, hit areas, control point) scales with the current px.
    const u = ann.unit ?? 1;
    // Shift-panning must move the view, not the annotation under the cursor,
    // and must not select it either (Konva still fires click on mouse-up).
    const isDraggable = !shiftPanning && (selectedTool === 'select' || isSelected);
    const selectOnClick = () => {
      if (!shiftPanning) setSelectedAnnotation(ann.id);
    };

    switch (ann.type) {
      case 'rectangle':
        return (
          <Rect
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            x={ann.x}
            y={ann.y}
            width={ann.width}
            height={ann.height}
            rotation={ann.rotation || 0}
            stroke={ann.stroke}
            strokeWidth={ann.strokeWidth}
            // Transparent fill makes the whole interior a selectable/draggable
            // area, and a wide hit-stroke makes the border easy to grab.
            fill={ann.fill || 'transparent'}
            cornerRadius={(ann.cornerRadius ?? 0) * u}
            hitStrokeWidth={Math.max(ann.strokeWidth || 2 * px, 16 * px)}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'ellipse': {
        const ew = ann.width ?? 0;
        const eh = ann.height ?? 0;
        return (
          <Shape
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            x={ann.x}
            y={ann.y}
            width={ew}
            height={eh}
            rotation={ann.rotation || 0}
            stroke={ann.stroke}
            strokeWidth={ann.strokeWidth}
            // As for the rectangle: a transparent fill makes the inside
            // grabbable, a wide hit-stroke the outline.
            fill="transparent"
            hitStrokeWidth={Math.max(ann.strokeWidth || 2 * px, 16 * px)}
            sceneFunc={(ctx, shape) => {
              const c = ctx._context;
              c.beginPath();
              c.ellipse(ew / 2, eh / 2, Math.abs(ew) / 2, Math.abs(eh) / 2, 0, 0, Math.PI * 2);
              ctx.fillStrokeShape(shape);
            }}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      }
      case 'line':
      case 'pen':
      case 'marker':
        // Points are in doc px and the node stays at the origin: a move or
        // transform is baked into the points (see handleDragEnd).
        return (
          <Line
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            points={ann.points || []}
            stroke={ann.stroke}
            strokeWidth={ann.strokeWidth}
            opacity={ann.opacity ?? 1}
            // Freehand strokes are smoothed; a line is straight.
            tension={ann.type === 'line' ? 0 : 0.4}
            lineCap={ann.type === 'marker' ? 'square' : 'round'}
            lineJoin="round"
            hitStrokeWidth={Math.max(ann.strokeWidth || 2 * px, 16 * px)}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'highlight':
        return (
          <Rect
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            x={ann.x}
            y={ann.y}
            width={ann.width}
            height={ann.height}
            rotation={ann.rotation || 0}
            // The box itself draws nothing (the spotlight shape dims around
            // it); the transparent fill keeps its interior clickable.
            fill="transparent"
            cornerRadius={(ann.cornerRadius ?? 0) * u}
            stroke={isSelected ? '#007bff' : undefined}
            strokeWidth={isSelected ? 2 * px : 0}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'blur':
        // Blur and pixelate keep the look of the content, so they can still be
        // read back in part: they are for tidying, not for secrets.
        if (ann.blurMode === 'blur' || ann.blurMode === 'pixelate') {
          return (
            <BlurEffectShape
              key={ann.id}
              ann={ann}
              mode={ann.blurMode}
              images={images}
              elements={imageElsRef.current}
              version={loadedCount}
              background={imagesSelectable ? background : null}
              unit={u}
              shapeProps={{
                draggable: isDraggable,
                onClick: selectOnClick,
                onMouseEnter: () => handleAnnotationMouseEnter(ann.id),
                onMouseLeave: handleAnnotationMouseLeave,
                onDragEnd: (e: any) => handleDragEnd(e, ann.id),
                onTransformEnd: (e: any) => handleTransformEnd(e, ann.id),
              }}
              register={(node) => {
                if (node) shapeRefs.current[ann.id] = node;
              }}
            />
          );
        }
        // Secure: redacts sensitive information with a textured pattern (softer
        // look than solid black). It must stay fully opaque: any transparency
        // lets the text underneath show through, and a contrast boost on the
        // exported image makes it readable again.
        return (
          <Rect
            key={`${ann.id}-${blurPatternImage ? 'pattern' : 'solid'}`}
            ref={(node) => {
              if (node) {
                shapeRefs.current[ann.id] = node;
              }
            }}
            x={ann.x}
            y={ann.y}
            width={ann.width}
            height={ann.height}
            rotation={ann.rotation || 0}
            fillPatternImage={blurPatternImage || undefined}
            fillPatternRepeat="repeat"
            fillPatternScale={{ x: u, y: u }}
            fill={blurPatternImage ? undefined : '#000000'}
            opacity={1}
            stroke={isSelected ? '#007bff' : undefined}
            strokeWidth={isSelected ? 2 * px : 0}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'arrow':
        const arrowCurvature = ann.curvature || 0;
        const [x1, y1, x2, y2] = ann.points || [0, 0, 0, 0];
        const midX = (x1 + x2) / 2;
        const midY = (y1 + y2) / 2;
        const dx = x2 - x1;
        const dy = y2 - y1;
        const distance = Math.sqrt(dx * dx + dy * dy);
        const safeDist = distance || 1; // avoid division by zero on degenerate arrows
        const offsetX = -dy / safeDist * distance * arrowCurvature;
        const offsetY = dx / safeDist * distance * arrowCurvature;
        const controlX = midX + offsetX;
        const controlY = midY + offsetY;

        if (arrowCurvature === 0 && !isSelected) {
          // Straight arrow without control point (not selected)
          return (
            <Arrow
              key={ann.id}
              ref={(node) => {
                if (node) shapeRefs.current[ann.id] = node;
              }}
              points={ann.points || []}
              stroke={ann.stroke}
              strokeWidth={ann.strokeWidth}
              fill={ann.stroke}
              pointerLength={10 * u}
              pointerWidth={10 * u}
              hitStrokeWidth={20 * px}
              draggable={isDraggable}
              onClick={selectOnClick}
              onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
              onMouseLeave={handleAnnotationMouseLeave}
              onDragEnd={(e) => handleDragEnd(e, ann.id)}
              onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
            />
          );
        } else {
          // Curved arrow OR selected straight arrow (show control point)
          const curvedPoints = getCurvedArrowPath(ann.points || [], arrowCurvature);
          const lastIdx = curvedPoints.length - 2;
          const arrowEndX = curvedPoints[lastIdx];
          const arrowEndY = curvedPoints[lastIdx + 1];
          const prevIdx = Math.max(0, lastIdx - 4);
          const prevX = curvedPoints[prevIdx];
          const prevY = curvedPoints[prevIdx + 1];

          const angle = Math.atan2(arrowEndY - prevY, arrowEndX - prevX);
          const arrowLength = 12 * u;

          const arrowPoint1X = arrowEndX - arrowLength * Math.cos(angle - Math.PI / 6);
          const arrowPoint1Y = arrowEndY - arrowLength * Math.sin(angle - Math.PI / 6);
          const arrowPoint2X = arrowEndX - arrowLength * Math.cos(angle + Math.PI / 6);
          const arrowPoint2Y = arrowEndY - arrowLength * Math.sin(angle + Math.PI / 6);

          return (
            <Group
              key={ann.id}
              ref={(node) => {
                if (node) shapeRefs.current[ann.id] = node;
              }}
              draggable={isDraggable}
              onClick={selectOnClick}
              onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
              onMouseLeave={handleAnnotationMouseLeave}
              onDragEnd={(e) => handleDragEnd(e, ann.id)}
              onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
            >
              <Line
                points={curvedPoints}
                stroke={ann.stroke}
                strokeWidth={ann.strokeWidth}
                hitStrokeWidth={20 * px}
                lineCap="round"
                lineJoin="round"
              />
              <Line
                points={[arrowPoint1X, arrowPoint1Y, arrowEndX, arrowEndY, arrowPoint2X, arrowPoint2Y]}
                stroke={ann.stroke}
                strokeWidth={ann.strokeWidth}
                fill={ann.stroke}
                closed={true}
              />
              {/* Control point - only show when selected */}
              {isSelected && (
                <Circle
                  x={controlX}
                  y={controlY}
                  radius={6 * px}
                  fill="#8b5cf6"
                  stroke="white"
                  strokeWidth={2 * px}
                  draggable={true}
                  onDragStart={(e) => {
                    // Prevent group from dragging when dragging control point
                    e.cancelBubble = true;
                  }}
                  onDragMove={(e) => {
                    e.cancelBubble = true;
                    const node = e.target;
                    const newControlX = node.x();
                    const newControlY = node.y();

                    // Calculate new curvature based on control point position
                    const newOffsetX = newControlX - midX;
                    const newOffsetY = newControlY - midY;
                    const newCurvature = Math.sqrt(newOffsetX * newOffsetX + newOffsetY * newOffsetY) / distance;

                    // Determine sign based on which side of the line
                    const crossProduct = dx * newOffsetY - dy * newOffsetX;
                    const signedCurvature = crossProduct > 0 ? newCurvature : -newCurvature;

                    updateAnnotation(ann.id, { curvature: signedCurvature });
                  }}
                  onDragEnd={(e) => {
                    e.cancelBubble = true;
                  }}
                />
              )}
            </Group>
          );
        }
      case 'arrow-text': {
        const isEditingLabel = editingTextId === ann.id;
        const atFontSize = ann.fontSize || 16 * u;
        const atLabel = arrowTextLabelPos(ann.points || [], ann.text || '', atFontSize, u);
        return (
          <Group
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDblClick={() => {
              setSelectedAnnotation(ann.id);
              setEditingTextId(ann.id);
              setTimeout(() => textEditRef.current?.focus(), 0);
            }}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          >
            <Arrow
              points={ann.points || []}
              stroke={ann.stroke}
              strokeWidth={ann.strokeWidth}
              fill={ann.stroke}
              pointerLength={10 * u}
              pointerWidth={10 * u}
              hitStrokeWidth={20 * px}
            />
            {/* Plain text label (rendered like the text tool); hidden while editing */}
            {!isEditingLabel && ann.text && (
              <Text
                x={atLabel.x}
                y={atLabel.y}
                text={ann.text}
                fontSize={atFontSize}
                fill={ann.stroke}
              />
            )}
            {!isEditingLabel && isSelected && !ann.text && (
              <Text
                x={atLabel.x}
                y={atLabel.y}
                text="Double-click to add text"
                fontSize={12 * px}
                fill="#999"
                fontStyle="italic"
              />
            )}
          </Group>
        );
      }
      case 'numbered-arrow':
        const [startX, startY, endX, endY] = ann.points || [0, 0, 0, 0];
        const circleRadius = 16 * u;

        return (
          <Group
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            draggable={isDraggable}
            onClick={selectOnClick}
            onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
            onMouseLeave={handleAnnotationMouseLeave}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          >
            <Arrow
              points={ann.points || []}
              stroke={ann.stroke}
              strokeWidth={ann.strokeWidth}
              fill={ann.stroke}
              pointerLength={10 * u}
              pointerWidth={10 * u}
              hitStrokeWidth={20 * px}
            />
            <Circle
              x={endX}
              y={endY}
              radius={circleRadius}
              fill={ann.fill}
              stroke={ann.stroke}
              strokeWidth={2 * u}
            />
            <Text
              x={endX - circleRadius}
              y={endY - circleRadius}
              width={circleRadius * 2}
              height={circleRadius * 2}
              text={String(ann.number || 1)}
              fontSize={16 * u}
              fontStyle="bold"
              fill="white"
              align="center"
              verticalAlign="middle"
            />
          </Group>
        );
      case 'text': {
        const isEditing = editingTextId === ann.id;
        // Show a dashed box outline while the box is empty (being drawn / before
        // any text is typed). Once it has text it renders clean like other shapes.
        const showBorder = !(ann.text && ann.text.trim());
        const textPad = 4 * u;
        return (
          <Group key={ann.id}>
            {showBorder && (
              <Rect
                x={ann.x}
                y={ann.y}
                width={ann.width}
                height={ann.height}
                rotation={ann.rotation || 0}
                stroke={isSelected || isEditing ? '#007bff' : 'rgba(120, 120, 120, 0.7)'}
                strokeWidth={px}
                dash={[4 * px, 4 * px]}
                listening={false}
              />
            )}
            {!isEditing && (
              <Text
                ref={(node) => {
                  if (node) shapeRefs.current[ann.id] = node;
                }}
                x={ann.x}
                y={ann.y}
                width={ann.width}
                height={ann.height}
                padding={textPad}
                wrap="word"
                text={ann.text || ''}
                fontSize={ann.fontSize || 16 * u}
                rotation={ann.rotation || 0}
                fill={ann.fill}
                draggable={isDraggable}
                onClick={selectOnClick}
                onDblClick={() => {
                  setSelectedAnnotation(ann.id);
                  setEditingTextId(ann.id);
                  setTimeout(() => textEditRef.current?.focus(), 0);
                }}
                onMouseEnter={() => handleAnnotationMouseEnter(ann.id)}
                onMouseLeave={handleAnnotationMouseLeave}
                onDragEnd={(e) => handleDragEnd(e, ann.id)}
                onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
              />
            )}
          </Group>
        );
      }
      default:
        return null;
    }
  };

  if (!ready) {
    return <div className="editor-loading">Loading...</div>;
  }

  // Highlight boxes, including the one being drawn, for the spotlight shade.
  const spotlightBoxes = (currentAnnotation ? [...annotations, currentAnnotation] : annotations).filter(
    (a) => a.type === 'highlight',
  );

  return (
    <div className="editor-window">
      <Toolbar />
      <div className="editor-canvas">
        <Stage
          ref={stageRef as any}
          width={viewport.width}
          height={viewport.height}
          scaleX={viewScale}
          scaleY={viewScale}
          x={stagePos.x}
          y={stagePos.y}
          draggable={false}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
        >
          {/* Backdrop: the background picture over the whole view frame and
              the shot's shadow, under the document. */}
          {backdropBox && (
            <Layer offsetX={viewFrame.x} offsetY={viewFrame.y} listening={false}>
              <Shape sceneFunc={(ctx) => paintBackdrop(ctx._context, viewFrame, backdropLook, backdropPhoto)} />
              {backdrop.shadow && (
                <Rect
                  x={frame.x}
                  y={frame.y}
                  width={frame.width}
                  height={frame.height}
                  cornerRadius={backdropBox.radius}
                  // Hidden under the shot; white, so a transparent shot does
                  // not show a black box.
                  fill="#ffffff"
                  shadowColor="#000000"
                  shadowBlur={backdropBox.shadowBlur}
                  shadowOffsetY={backdropBox.shadowOffset}
                  shadowOpacity={0.45}
                />
              )}
            </Layer>
          )}
          {/* The layer offset puts the view frame origin at the stage origin;
              the clip keeps everything outside the frame (cropped-away pixels,
              annotations that left it) off screen. With a backdrop the clip
              has the shot's rounded corners. */}
          <Layer
            offsetX={viewFrame.x}
            offsetY={viewFrame.y}
            clipX={frame.x}
            clipY={frame.y}
            clipWidth={frame.width}
            clipHeight={frame.height}
            clipFunc={backdropBox && backdropBox.radius > 0
              ? (ctx) => {
                  const c = ctx._context;
                  c.beginPath();
                  c.roundRect(frame.x, frame.y, frame.width, frame.height, backdropBox.radius);
                }
              : undefined}
          >
            {/* Background under the images, covering the frame. A shot with one
                image keeps its own pixels (a transparent PNG stays transparent);
                the color only shows once there are two or more images. The rect
                is always present as the empty-canvas hit target. */}
            <Rect
              name={BACKGROUND_NAME}
              x={frame.x}
              y={frame.y}
              width={frame.width}
              height={frame.height}
              fill={imagesSelectable ? background : 'transparent'}
            />
            {images.map((im) => {
              const el = imageElsRef.current.get(im.id);
              return el ? (
                <KonvaImage
                  key={im.id}
                  ref={(node) => {
                    if (node) imageNodeRefs.current[im.id] = node;
                    else delete imageNodeRefs.current[im.id];
                  }}
                  name={imagesSelectable ? IMAGE_NAME : BACKGROUND_NAME}
                  image={el}
                  // Non-destructive crop in source px; a zero-size crop draws the whole image.
                  crop={im.crop ?? { x: 0, y: 0, width: 0, height: 0 }}
                  x={im.x}
                  y={im.y}
                  width={im.width}
                  height={im.height}
                  draggable={imageInteractive}
                  onMouseDown={() => handleImageMouseDown(im.id)}
                  onMouseEnter={handleImageMouseEnter}
                  onMouseLeave={handleImageMouseLeave}
                  onDragEnd={(e) => handleImageDragEnd(e, im.id)}
                  onTransformEnd={(e) => handleImageTransformEnd(e, im.id)}
                />
              ) : null;
            })}
            {/* Frame border; a backdrop already shows where the shot ends. */}
            {!backdropBox && (
              <Rect
                x={frame.x}
                y={frame.y}
                width={frame.width}
                height={frame.height}
                stroke="rgba(139, 92, 246, 0.4)"
                strokeWidth={2 * px}
                listening={false}
              />
            )}
            {/* Under the other annotations, so boxes and arrows outside a
                highlight stay at full brightness. */}
            {spotlightBoxes.length > 0 && (
              <Shape
                listening={false}
                // Read each box from its node when there is one: during a drag
                // or resize the node moves before the store does, and the
                // bright area must follow it.
                sceneFunc={(ctx) =>
                  drawSpotlight(
                    ctx._context,
                    frame,
                    spotlightBoxes.map((a) => {
                      const node = shapeRefs.current[a.id];
                      const radius = (a.cornerRadius ?? 0) * (a.unit ?? 1);
                      return node
                        ? {
                            x: node.x(),
                            y: node.y(),
                            width: node.width() * node.scaleX(),
                            height: node.height() * node.scaleY(),
                            rotation: node.rotation(),
                            radius,
                          }
                        : { x: a.x, y: a.y, width: a.width || 0, height: a.height || 0, rotation: a.rotation || 0, radius };
                    }),
                    highlightOptions.dim,
                  )
                }
              />
            )}
            {annotations.map(renderAnnotation)}
            {currentAnnotation && renderAnnotation(currentAnnotation)}
            {/* Marquee selection rectangle */}
            {selectionRect && (
              <Rect
                x={selectionRect.width < 0 ? selectionRect.x + selectionRect.width : selectionRect.x}
                y={selectionRect.height < 0 ? selectionRect.y + selectionRect.height : selectionRect.y}
                width={Math.abs(selectionRect.width)}
                height={Math.abs(selectionRect.height)}
                fill="rgba(0, 123, 255, 0.1)"
                stroke="#007bff"
                strokeWidth={px}
                dash={[4 * px, 4 * px]}
                listening={false}
              />
            )}
            {cropRegion && (() => {
              const normX = cropRegion.width < 0 ? cropRegion.x + cropRegion.width : cropRegion.x;
              const normY = cropRegion.height < 0 ? cropRegion.y + cropRegion.height : cropRegion.y;
              const normW = Math.abs(cropRegion.width);
              const normH = Math.abs(cropRegion.height);
              // Dim only what the crop cuts away: the target image, or the
              // whole frame for a canvas crop. Other images stay as they are.
              const dim = cropTarget ?? frame;
              
              return (
                <Group>
                  {/* Top Dimmer */}
                  <Rect
                    x={dim.x}
                    y={dim.y}
                    width={dim.width}
                    height={Math.max(0, normY - dim.y)}
                    fill="rgba(0, 0, 0, 0.5)"
                    listening={false}
                  />
                  {/* Bottom Dimmer */}
                  <Rect
                    x={dim.x}
                    y={normY + normH}
                    width={dim.width}
                    height={Math.max(0, dim.y + dim.height - (normY + normH))}
                    fill="rgba(0, 0, 0, 0.5)"
                    listening={false}
                  />
                  {/* Left Dimmer */}
                  <Rect
                    x={dim.x}
                    y={normY}
                    width={Math.max(0, normX - dim.x)}
                    height={normH}
                    fill="rgba(0, 0, 0, 0.5)"
                    listening={false}
                  />
                  {/* Right Dimmer */}
                  <Rect
                    x={normX + normW}
                    y={normY}
                    width={Math.max(0, dim.x + dim.width - (normX + normW))}
                    height={normH}
                    fill="rgba(0, 0, 0, 0.5)"
                    listening={false}
                  />
                  {/* Crop Selection Box */}
                  <Rect
                    x={normX}
                    y={normY}
                    width={normW}
                    height={normH}
                    stroke="#007bff"
                    strokeWidth={2 * px}
                    dash={[10 * px, 5 * px]}
                    fill="transparent"
                    listening={false}
                  />
                </Group>
              );
            })()}
          </Layer>
          {/* Editor chrome that must not be clipped by the frame: transformer
              handles and the hover box of an annotation at the frame edge reach
              outside it. Same offset as the document layer, so both share doc px.
              The export region is the frame, so nothing outside it is saved. */}
          <Layer name="editor-chrome" offsetX={viewFrame.x} offsetY={viewFrame.y}>
            {/* Marks the image the crop tool will crop, before and while the
                region is drawn. */}
            {cropTarget && (
              <Rect
                x={cropTarget.x - 2 * px}
                y={cropTarget.y - 2 * px}
                width={cropTarget.width + 4 * px}
                height={cropTarget.height + 4 * px}
                stroke="#007bff"
                strokeWidth={3 * px}
                listening={false}
              />
            )}
            {/* Several annotations selected together: a dashed box around each,
                since they keep their own colours. */}
            {marqueeSelectedIds.map((id) => {
              const node = shapeRefs.current[id];
              const layer = node?.getLayer();
              if (!node || !layer || !annotations.some((a) => a.id === id)) return null;
              const box = node.getClientRect({ relativeTo: layer });
              const pad = 4 * px;
              return (
                <Rect
                  key={`multi-${id}`}
                  x={box.x - pad}
                  y={box.y - pad}
                  width={box.width + pad * 2}
                  height={box.height + pad * 2}
                  stroke="#007bff"
                  strokeWidth={1.5 * px}
                  dash={[6 * px, 4 * px]}
                  listening={false}
                />
              );
            })}
            {/* Hover bounding box: shows the selectable rectangular area when
                hovering an annotation that isn't the currently selected one. */}
            {hoveredAnnotationId && hoveredAnnotationId !== selectedAnnotationId && (() => {
              const node = shapeRefs.current[hoveredAnnotationId];
              if (!node) return null;
              const layer = node.getLayer();
              if (!layer) return null;
              const box = node.getClientRect({ relativeTo: layer });
              const pad = 6 * px;
              return (
                <Rect
                  x={box.x - pad}
                  y={box.y - pad}
                  width={box.width + pad * 2}
                  height={box.height + pad * 2}
                  stroke="#007bff"
                  strokeWidth={px}
                  dash={[4 * px, 4 * px]}
                  fill="rgba(0, 123, 255, 0.06)"
                  cornerRadius={3 * px}
                  listening={false}
                />
              );
            })()}
            <Transformer
              ref={transformerRef}
              // Images keep their aspect ratio and do not rotate; annotations
              // keep the full handle set.
              rotateEnabled={!imageSelected}
              keepRatio={imageSelected}
              rotateAnchorOffset={24}
              boundBoxFunc={(oldBox, newBox) => {
                // Limit resize
                if (newBox.width < 5 || newBox.height < 5) {
                  return oldBox;
                }
                return newBox;
              }}
              enabledAnchors={imageSelected
                ? ['top-left', 'top-right', 'bottom-left', 'bottom-right']
                : ['top-left', 'top-right', 'bottom-left', 'bottom-right', 'middle-left', 'middle-right', 'top-center', 'bottom-center']}
              anchorSize={12}
              anchorStroke="#007bff"
              anchorFill="#ffffff"
              anchorStrokeWidth={2}
              anchorCornerRadius={3}
              borderStroke="#007bff"
              borderStrokeWidth={2}
              borderDash={[4, 4]}
            />
          </Layer>
        </Stage>
        {textInput && (
          <div
            className="text-input-overlay"
            style={{
              position: 'absolute',
              left: textInput.x,
              top: textInput.y,
              zIndex: 1000,
            }}
          >
            <input
              ref={textInputRef}
              type="text"
              value={textValue}
              onChange={(e) => setTextValue(e.target.value)}
              onKeyDown={handleTextKeyDown}
              onBlur={handleTextSubmit}
              placeholder="Type text..."
              style={{
                padding: '4px 8px',
                fontSize: '16px',
                border: '2px solid #007bff',
                borderRadius: '4px',
                outline: 'none',
              }}
            />
          </div>
        )}
        {/* Inline text-box editor: a transparent textarea overlaid exactly on
            the box. Typing updates the annotation live (no Enter to render);
            committed on blur. */}
        {editingTextId && stageRef.current && (() => {
          const ann = annotations.find((a) => a.id === editingTextId);
          if (!ann) return null;
          const rect = stageRef.current.container().getBoundingClientRect();
          const u = ann.unit ?? 1;
          const fontSize = ann.fontSize || 16 * u;

          // Text-box uses its own x/y/width; arrow-text uses the label anchor.
          // Everything is in doc px; viewScale maps it to screen px.
          let lx: number, ly: number, w: number, h: number, pad: number;
          if (ann.type === 'arrow-text') {
            const lp = arrowTextLabelPos(ann.points || [], ann.text || '', fontSize, u);
            lx = lp.x;
            ly = lp.y;
            w = 180 * u;
            h = fontSize + 8 * u;
            pad = 0;
          } else {
            lx = ann.x || 0;
            ly = ann.y || 0;
            w = ann.width || 0;
            h = ann.height || 0;
            pad = 4 * u;
          }
          const left = rect.left + (lx - viewFrame.x) * viewScale + stagePos.x;
          const top = rect.top + (ly - viewFrame.y) * viewScale + stagePos.y;
          return (
            <textarea
              ref={textEditRef}
              value={ann.text || ''}
              onChange={(e) => updateAnnotation(ann.id, { text: e.target.value })}
              onBlur={commitTextEditing}
              onKeyDown={handleTextEditKeyDown}
              autoFocus
              placeholder="Type text…"
              style={{
                position: 'fixed',
                left,
                top,
                width: w * viewScale,
                height: h * viewScale,
                padding: pad * viewScale,
                boxSizing: 'border-box',
                margin: 0,
                fontFamily: 'Arial, sans-serif',
                fontSize: fontSize * viewScale,
                lineHeight: 1,
                color: ann.fill || ann.stroke || '#000000',
                background: 'transparent',
                border: '1px dashed #007bff',
                outline: 'none',
                resize: 'none',
                overflow: 'hidden',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                zIndex: 1000,
                caretColor: ann.fill || ann.stroke || '#000000',
              }}
            />
          );
        })()}
      </div>
      <ActionBar stageRef={stageRef} frame={viewFrame} />
      {toast && <Toast message={toast.message} type={toast.type} onClose={closeToast} />}
      <ZoomBar zoom={zoom} onZoomChange={setZoom} scaleRatio={scaleRatio} />
      {version && (
        <div className="version-badge">
          {version}
        </div>
      )}
    </div>
  );
};

export default EditorWindow;

