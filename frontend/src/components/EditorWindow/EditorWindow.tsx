import React, { useRef, useEffect, useState } from 'react';
import { Stage, Layer, Image as KonvaImage, Rect, Arrow, Text, Transformer, Line, Circle, Group } from 'react-konva';
import Konva from 'konva';
import { useEditorStore } from '../../store/editorStore';
import { Annotation } from '../../types';
import Toolbar from './Toolbar';
import ActionBar from './ActionBar';
import ZoomBar from './ZoomBar';
import { GetVersion } from '../../../wailsjs/go/main/App';
import './EditorWindow.css';

const EditorWindow: React.FC = () => {
  const {
    image,
    annotations,
    selectedTool,
    selectedAnnotationId,
    selectedColor,
    selectedSize,
    cropRegion,
    setCropRegion,
    applyCrop,
    addAnnotation,
    updateAnnotation,
    deleteAnnotation,
    setSelectedAnnotation,
    setSelectedTool,
    setSelectedSize,
    undo,
    redo,
  } = useEditorStore();

  const [konvaImage, setKonvaImage] = useState<HTMLImageElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 800, height: 600 });
  const [scaleRatio, setScaleRatio] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [stagePos, setStagePos] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null);
  const [textInput, setTextInput] = useState<{ x: number; y: number; fontSize?: number; annotationId?: string } | null>(null);
  const [textValue, setTextValue] = useState('');
  const [version, setVersion] = useState('');

  const stageRef = useRef<any>(null);
  const textInputRef = useRef<HTMLInputElement>(null);
  const transformerRef = useRef<any>(null);
  const shapeRefs = useRef<{ [key: string]: any }>({});

  // Load version
  useEffect(() => {
    GetVersion().then((info) => {
      setVersion(info.version);
    }).catch((err) => {
      console.error('Failed to get version:', err);
    });
  }, []);

  // Load image
  useEffect(() => {
    if (!image) return;

    const img = new window.Image();
    img.src = `data:image/png;base64,${image}`;
    img.onload = () => {
      setKonvaImage(img);
      // Fit image to window - use more space
      const maxWidth = window.innerWidth - 40;
      const maxHeight = window.innerHeight - 140;
      const scale = Math.min(maxWidth / img.width, maxHeight / img.height, 1);
      setStageSize({
        width: img.width * scale,
        height: img.height * scale,
      });
      setScaleRatio(scale);
    };
  }, [image]);

  // Handle window resize
  useEffect(() => {
    if (!konvaImage) return;

    const handleResize = () => {
      // Recalculate stage size to fit new window dimensions
      const maxWidth = window.innerWidth - 40;
      const maxHeight = window.innerHeight - 140;
      const scale = Math.min(maxWidth / konvaImage.width, maxHeight / konvaImage.height, 1);

      setStageSize({
        width: konvaImage.width * scale,
        height: konvaImage.height * scale,
      });
      setScaleRatio(scale);

      // Reset zoom and pan when resizing
      setZoom(1);
      setStagePos({ x: 0, y: 0 });
    };

    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [konvaImage]);

  // Handle Cmd/Ctrl + Scroll to zoom
  useEffect(() => {
    const handleWheel = (e: Event) => {
      const wheelEvent = e as WheelEvent;

      // Check if Cmd (Mac) or Ctrl (Windows/Linux) is pressed
      if (wheelEvent.metaKey || wheelEvent.ctrlKey) {
        wheelEvent.preventDefault();

        const stage = stageRef.current;
        if (!stage) return;

        const oldZoom = zoom;
        const pointer = stage.getPointerPosition();

        if (!pointer) return;

        // Calculate zoom delta
        const scaleBy = 1.05;
        const direction = wheelEvent.deltaY > 0 ? -1 : 1;
        const newZoom = direction > 0
          ? Math.min(oldZoom * scaleBy, 3)
          : Math.max(oldZoom / scaleBy, 0.5);

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
        setStagePos(newPos);
      }
    };

    const canvasContainer = document.querySelector('.editor-canvas');
    if (canvasContainer) {
      canvasContainer.addEventListener('wheel', handleWheel as EventListener, { passive: false });

      return () => {
        canvasContainer.removeEventListener('wheel', handleWheel as EventListener);
      };
    }
  }, [zoom, stagePos]);

  // Attach transformer to selected annotation
  useEffect(() => {
    if (!transformerRef.current) return;

    if (selectedAnnotationId && shapeRefs.current[selectedAnnotationId]) {
      const node = shapeRefs.current[selectedAnnotationId];
      transformerRef.current.nodes([node]);
      transformerRef.current.getLayer().batchDraw();
    } else {
      transformerRef.current.nodes([]);
      transformerRef.current.getLayer()?.batchDraw();
    }
  }, [selectedAnnotationId, annotations]);

  // Update cursor when zoom or tool changes
  useEffect(() => {
    if (stageRef.current) {
      const container = stageRef.current.container();

      // Crop tool always shows crosshair
      if (selectedTool === 'crop') {
        container.style.cursor = 'crosshair';
      } else if (zoom > 1) {
        container.style.cursor = 'grab';
      } else {
        container.style.cursor = 'default';
      }
    }
  }, [zoom, selectedTool]);

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if typing in text input
      if (textInput) return;

      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const cmdOrCtrl = isMac ? e.metaKey : e.ctrlKey;

      // Handle crop mode shortcuts
      if (cropRegion) {
        if (e.key === 'Enter') {
          e.preventDefault();
          // Apply crop
          applyCrop(scaleRatio);
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

        // Cmd+A / Ctrl+A - Select all (select tool)
        if (key === 'a') {
          e.preventDefault();
          setSelectedTool('select');
          // TODO: Select all annotations
          return;
        }

        // Cmd+D / Ctrl+D - Delete selected
        if (key === 'd') {
          e.preventDefault();
          if (selectedAnnotationId) {
            deleteAnnotation(selectedAnnotationId);
            setSelectedAnnotation(null);
          }
          return;
        }

        // Note: Cmd+S, Cmd+Shift+S, Cmd+C, Cmd+Shift+C are handled in ActionBar
        // We don't preventDefault here to let ActionBar handle them
      }

      // Delete or Backspace - Delete selected annotation (without Cmd/Ctrl)
      if (!cmdOrCtrl && (e.key === 'Delete' || e.key === 'Backspace')) {
        if (selectedAnnotationId) {
          e.preventDefault();
          deleteAnnotation(selectedAnnotationId);
          setSelectedAnnotation(null);
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
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [cropRegion, textInput, scaleRatio, applyCrop, setCropRegion, setSelectedTool, selectedSize, setSelectedSize, undo, redo, selectedAnnotationId, deleteAnnotation, setSelectedAnnotation]);

  const handleMouseDown = (e: any) => {
    const stage = e.target.getStage();

    // For crop tool, start drawing crop region (priority over panning)
    if (selectedTool === 'crop') {
      const pos = stage.getPointerPosition();
      setIsDrawing(true);
      setCropRegion({
        x: pos.x,
        y: pos.y,
        width: 0,
        height: 0,
      });
      return;
    }

    // Enable panning when zoom > 1 and clicking on stage background
    if (zoom > 1 && e.target === stage) {
      setIsPanning(true);
      stage.container().style.cursor = 'grabbing';
      return;
    }

    // For select tool, only handle clicks on annotations (handled in renderAnnotation)
    if (selectedTool === 'select') {
      // Click on stage background deselects
      if (e.target === stage) {
        setSelectedAnnotation(null);
      }
      return;
    }

    const pos = stage.getPointerPosition();

    // Check if clicking on an existing annotation of the same type
    // If yes, select it instead of creating new one
    if (e.target !== stage) {
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
    };

    // Calculate actual size based on selectedSize
    const strokeWidth = selectedSize === 1 ? 1 : selectedSize === 2 ? 2 : selectedSize === 3 ? 4 : selectedSize === 4 ? 6 : 8;
    const fontSize = selectedSize === 1 ? 12 : selectedSize === 2 ? 16 : selectedSize === 3 ? 20 : selectedSize === 4 ? 24 : 32;

    switch (selectedTool) {
      case 'rectangle':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          stroke: selectedColor,
          strokeWidth,
          fill: 'transparent',
        });
        break;
      case 'highlight':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          fill: selectedColor,
          opacity: 0.3,
        });
        break;
      case 'blur':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          fill: '#ffffff',
          opacity: 1,
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
        // Show text input at click position
        setTextInput({ x: pos.x, y: pos.y, fontSize });
        setTextValue('');
        setIsDrawing(false);
        // Focus input after render
        setTimeout(() => textInputRef.current?.focus(), 0);
        break;
    }
  };

  const handleMouseMove = (e: any) => {
    const stage = e.target.getStage();

    // Handle panning
    if (isPanning) {
      const newPos = {
        x: stagePos.x + e.evt.movementX,
        y: stagePos.y + e.evt.movementY,
      };
      setStagePos(newPos);
      stage.position(newPos);
      stage.batchDraw();
      return;
    }

    if (!isDrawing) return;

    const pos = stage.getPointerPosition();

    // Handle crop region drawing
    if (selectedTool === 'crop' && cropRegion) {
      setCropRegion({
        ...cropRegion,
        width: pos.x - cropRegion.x,
        height: pos.y - cropRegion.y,
      });
      return;
    }

    if (!currentAnnotation) return;

    if (selectedTool === 'rectangle' || selectedTool === 'highlight' || selectedTool === 'blur') {
      setCurrentAnnotation({
        ...currentAnnotation,
        width: pos.x - currentAnnotation.x,
        height: pos.y - currentAnnotation.y,
      });
    } else if (selectedTool === 'arrow' || selectedTool === 'arrow-text' || selectedTool === 'numbered-arrow') {
      setCurrentAnnotation({
        ...currentAnnotation,
        points: [currentAnnotation.x, currentAnnotation.y, pos.x, pos.y],
      });
    }
  };

  const handleMouseUp = () => {
    // Handle panning end
    if (isPanning) {
      setIsPanning(false);
      if (stageRef.current) {
        const container = stageRef.current.container();
        // Restore cursor based on current tool
        if (selectedTool === 'crop') {
          container.style.cursor = 'crosshair';
        } else if (zoom > 1) {
          container.style.cursor = 'grab';
        } else {
          container.style.cursor = 'default';
        }
      }
      return;
    }

    if (!isDrawing) return;

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

      if (normalizedRegion.width > 10 && normalizedRegion.height > 10) {
        // Keep the normalized region visible
        setCropRegion(normalizedRegion);
      } else {
        setCropRegion(null);
      }
      return;
    }

    if (!currentAnnotation) return;

    if (currentAnnotation.width !== 0 || currentAnnotation.height !== 0 || currentAnnotation.points) {
      // For arrow-text, show text input after drawing
      if (selectedTool === 'arrow-text' && currentAnnotation.points) {
        const [startX, startY] = currentAnnotation.points;
        const tempAnnotation = { ...currentAnnotation };

        // Add annotation first
        addAnnotation(tempAnnotation);

        // Show text input at start point (opposite of arrow)
        const textPadding = 8;
        const textBgPadding = 6;
        const textWidth = 150; // Default width for empty text
        const textX = startX - textWidth - textPadding;
        const textY = startY - 12;

        setTextInput({ x: textX + textBgPadding, y: textY + 4, fontSize: 14, annotationId: tempAnnotation.id });
        setTextValue('');
        setTimeout(() => textInputRef.current?.focus(), 0);
      } else {
        addAnnotation(currentAnnotation);
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
      fontSize: textInput.fontSize || 16,
      fill: selectedColor,
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

    // For annotations with points (arrows), update points instead of x/y
    if (ann.points && (ann.type === 'arrow' || ann.type === 'arrow-text' || ann.type === 'numbered-arrow')) {
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
    const scaleX = node.scaleX();
    const scaleY = node.scaleY();

    // Reset scale
    node.scaleX(1);
    node.scaleY(1);

    const ann = annotations.find((a) => a.id === annId);
    if (!ann) return;

    // Update annotation with new dimensions
    if (ann.type === 'rectangle' || ann.type === 'highlight') {
      updateAnnotation(annId, {
        x: node.x(),
        y: node.y(),
        width: Math.max(5, (ann.width || 0) * scaleX),
        height: Math.max(5, (ann.height || 0) * scaleY),
      });
    } else if (ann.type === 'text') {
      updateAnnotation(annId, {
        x: node.x(),
        y: node.y(),
        fontSize: Math.max(8, (ann.fontSize || 16) * scaleY),
      });
    }
  };

  const renderAnnotation = (ann: Annotation) => {
    const isSelected = ann.id === selectedAnnotationId;
    const isDraggable = selectedTool === 'select' || isSelected;

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
            stroke={isSelected ? '#007bff' : ann.stroke}
            strokeWidth={isSelected ? 3 : ann.strokeWidth}
            fill={ann.fill}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
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
            fill={ann.fill}
            opacity={ann.opacity}
            stroke={isSelected ? '#007bff' : undefined}
            strokeWidth={isSelected ? 2 : 0}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'blur':
        return (
          <Rect
            key={ann.id}
            ref={(node) => {
              if (node) {
                shapeRefs.current[ann.id] = node;
                // Apply blur filter
                node.cache();
                node.filters([Konva.Filters.Blur]);
                node.blurRadius(20);
              }
            }}
            x={ann.x}
            y={ann.y}
            width={ann.width}
            height={ann.height}
            fillPatternImage={konvaImage || undefined}
            fillPatternX={-ann.x}
            fillPatternY={-ann.y}
            stroke={isSelected ? '#007bff' : undefined}
            strokeWidth={isSelected ? 2 : 0}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
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
        const offsetX = -dy / distance * distance * arrowCurvature;
        const offsetY = dx / distance * distance * arrowCurvature;
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
              pointerLength={10}
              pointerWidth={10}
              draggable={isDraggable}
              onClick={() => setSelectedAnnotation(ann.id)}
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
          const arrowLength = 12;

          const arrowPoint1X = arrowEndX - arrowLength * Math.cos(angle - Math.PI / 6);
          const arrowPoint1Y = arrowEndY - arrowLength * Math.sin(angle - Math.PI / 6);
          const arrowPoint2X = arrowEndX - arrowLength * Math.cos(angle + Math.PI / 6);
          const arrowPoint2Y = arrowEndY - arrowLength * Math.sin(angle + Math.PI / 6);

          return (
            <Group
              key={ann.id}
              draggable={isDraggable}
              onClick={() => setSelectedAnnotation(ann.id)}
              onDragEnd={(e) => handleDragEnd(e, ann.id)}
            >
              <Line
                points={curvedPoints}
                stroke={isSelected ? '#007bff' : ann.stroke}
                strokeWidth={isSelected ? 3 : ann.strokeWidth}
                lineCap="round"
                lineJoin="round"
              />
              <Line
                points={[arrowPoint1X, arrowPoint1Y, arrowEndX, arrowEndY, arrowPoint2X, arrowPoint2Y]}
                stroke={isSelected ? '#007bff' : ann.stroke}
                strokeWidth={isSelected ? 3 : ann.strokeWidth}
                fill={isSelected ? '#007bff' : ann.stroke}
                closed={true}
              />
              {/* Control point - only show when selected */}
              {isSelected && (
                <Circle
                  x={controlX}
                  y={controlY}
                  radius={6}
                  fill="#8b5cf6"
                  stroke="white"
                  strokeWidth={2}
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
      case 'arrow-text':
        const [atStartX, atStartY, atEndX, atEndY] = ann.points || [0, 0, 0, 0];
        const textPadding = 8;
        const textBgPadding = 6;

        // Calculate text position at start point (opposite of arrow)
        // Position text to the left of start point
        const textWidth = ann.text ? (ann.text.length * 8) + textBgPadding * 2 : 150;
        const textX = atStartX - textWidth - textPadding;
        const textY = atStartY - 12;

        return (
          <Group
            key={ann.id}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDblClick={() => {
              // Double-click to edit text
              setTextInput({ x: textX + textBgPadding, y: textY + 4, fontSize: 14, annotationId: ann.id });
              setTextValue(ann.text || '');
              setTimeout(() => textInputRef.current?.focus(), 0);
            }}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
          >
            <Arrow
              points={ann.points || []}
              stroke={isSelected ? '#007bff' : ann.stroke}
              strokeWidth={isSelected ? 3 : ann.strokeWidth}
              fill={isSelected ? '#007bff' : ann.stroke}
              pointerLength={10}
              pointerWidth={10}
            />
            {/* Text at the start of arrow (opposite of arrow head) */}
            {ann.text && (
              <>
                <Rect
                  x={textX}
                  y={textY}
                  width={textWidth}
                  height={24}
                  fill="white"
                  stroke={isSelected ? '#007bff' : ann.stroke}
                  strokeWidth={1}
                  cornerRadius={4}
                />
                <Text
                  x={textX + textBgPadding}
                  y={textY + 4}
                  text={ann.text}
                  fontSize={14}
                  fill={ann.stroke}
                  fontStyle="normal"
                />
              </>
            )}
            {/* Show placeholder when selected and no text */}
            {isSelected && !ann.text && (
              <Text
                x={textX + textBgPadding}
                y={textY + 4}
                text="Double-click to add text"
                fontSize={12}
                fill="#999"
                fontStyle="italic"
              />
            )}
          </Group>
        );
      case 'numbered-arrow':
        const [startX, startY, endX, endY] = ann.points || [0, 0, 0, 0];
        const circleRadius = 16;

        return (
          <Group
            key={ann.id}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
          >
            <Arrow
              points={ann.points || []}
              stroke={isSelected ? '#007bff' : ann.stroke}
              strokeWidth={isSelected ? 3 : ann.strokeWidth}
              fill={isSelected ? '#007bff' : ann.stroke}
              pointerLength={10}
              pointerWidth={10}
            />
            <Circle
              x={endX}
              y={endY}
              radius={circleRadius}
              fill={isSelected ? '#007bff' : ann.fill}
              stroke={isSelected ? '#007bff' : ann.stroke}
              strokeWidth={2}
            />
            <Text
              x={endX - circleRadius}
              y={endY - circleRadius}
              width={circleRadius * 2}
              height={circleRadius * 2}
              text={String(ann.number || 1)}
              fontSize={16}
              fontStyle="bold"
              fill="white"
              align="center"
              verticalAlign="middle"
            />
          </Group>
        );
      case 'text':
        return (
          <Text
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            x={ann.x}
            y={ann.y}
            text={ann.text || ''}
            fontSize={ann.fontSize || 16}
            fill={isSelected ? '#007bff' : ann.fill}
            draggable={isDraggable}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      default:
        return null;
    }
  };

  if (!image || !konvaImage) {
    return <div className="editor-loading">Loading...</div>;
  }

  return (
    <div className="editor-window">
      <Toolbar />
      <div className="editor-canvas">
        <Stage
          ref={stageRef as any}
          width={stageSize.width}
          height={stageSize.height}
          scaleX={zoom}
          scaleY={zoom}
          x={stagePos.x}
          y={stagePos.y}
          draggable={false}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
        >
          <Layer>
            <KonvaImage image={konvaImage} width={stageSize.width} height={stageSize.height} />
            {/* Image border */}
            <Rect
              x={0}
              y={0}
              width={stageSize.width}
              height={stageSize.height}
              stroke="rgba(139, 92, 246, 0.4)"
              strokeWidth={2}
              listening={false}
            />
            {annotations.map(renderAnnotation)}
            {currentAnnotation && renderAnnotation(currentAnnotation)}
            {cropRegion && (
              <Rect
                x={cropRegion.x}
                y={cropRegion.y}
                width={cropRegion.width}
                height={cropRegion.height}
                stroke="#007bff"
                strokeWidth={2}
                dash={[10, 5]}
                fill="rgba(0, 123, 255, 0.1)"
              />
            )}
            <Transformer
              ref={transformerRef}
              boundBoxFunc={(oldBox, newBox) => {
                // Limit resize
                if (newBox.width < 5 || newBox.height < 5) {
                  return oldBox;
                }
                return newBox;
              }}
              enabledAnchors={['top-left', 'top-right', 'bottom-left', 'bottom-right', 'middle-left', 'middle-right', 'top-center', 'bottom-center']}
              anchorSize={8}
              anchorStroke="#007bff"
              anchorFill="#ffffff"
              anchorStrokeWidth={2}
              anchorCornerRadius={2}
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
      </div>
      <ActionBar stageRef={stageRef} scaleRatio={scaleRatio} />
      <ZoomBar zoom={zoom} onZoomChange={setZoom} />
      {version && (
        <div className="version-badge">
          {version}
        </div>
      )}
    </div>
  );
};

export default EditorWindow;

