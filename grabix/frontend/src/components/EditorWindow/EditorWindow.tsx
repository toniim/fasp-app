import React, { useRef, useEffect, useState } from 'react';
import { Stage, Layer, Image as KonvaImage, Rect, Arrow, Text, Transformer } from 'react-konva';
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
    cropRegion,
    setCropRegion,
    applyCrop,
    addAnnotation,
    updateAnnotation,
    setSelectedAnnotation,
    setSelectedTool,
  } = useEditorStore();

  const [konvaImage, setKonvaImage] = useState<HTMLImageElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 800, height: 600 });
  const [scaleRatio, setScaleRatio] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [stagePos, setStagePos] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null);
  const [textInput, setTextInput] = useState<{ x: number; y: number } | null>(null);
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
  }, [selectedAnnotationId]);

  // Update cursor when zoom changes
  useEffect(() => {
    if (stageRef.current) {
      const container = stageRef.current.container();
      container.style.cursor = zoom > 1 ? 'grab' : 'default';
    }
  }, [zoom]);

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if typing in text input
      if (textInput) return;

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

      // Handle tool shortcuts (only when not in crop mode)
      if (!cropRegion) {
        const key = e.key.toLowerCase();

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
  }, [cropRegion, textInput, scaleRatio, applyCrop, setCropRegion, setSelectedTool]);

  const handleMouseDown = (e: any) => {
    const stage = e.target.getStage();

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

    // For crop tool, start drawing crop region
    if (selectedTool === 'crop') {
      setIsDrawing(true);
      setCropRegion({
        x: pos.x,
        y: pos.y,
        width: 0,
        height: 0,
      });
      return;
    }

    setIsDrawing(true);

    const id = `annotation-${Date.now()}`;
    const baseAnnotation: Annotation = {
      id,
      type: selectedTool,
      x: pos.x,
      y: pos.y,
    };

    switch (selectedTool) {
      case 'rectangle':
        setCurrentAnnotation({
          ...baseAnnotation,
          width: 0,
          height: 0,
          stroke: selectedColor,
          strokeWidth: 2,
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
          strokeWidth: 2,
        });
        break;
      case 'text':
        // Show text input at click position
        setTextInput({ x: pos.x, y: pos.y });
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
    } else if (selectedTool === 'arrow') {
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
        stageRef.current.container().style.cursor = zoom > 1 ? 'grab' : 'default';
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
      addAnnotation(currentAnnotation);
    }

    setIsDrawing(false);
    setCurrentAnnotation(null);
  };

  const handleTextSubmit = () => {
    if (!textInput || !textValue.trim()) {
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
      fontSize: 16,
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

  const handleDragEnd = (e: any, annId: string) => {
    const node = e.target;
    updateAnnotation(annId, {
      x: node.x(),
      y: node.y(),
    });
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
            draggable={isSelected}
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
            draggable={isSelected}
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
            draggable={isSelected}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
            onTransformEnd={(e) => handleTransformEnd(e, ann.id)}
          />
        );
      case 'arrow':
        return (
          <Arrow
            key={ann.id}
            ref={(node) => {
              if (node) shapeRefs.current[ann.id] = node;
            }}
            points={ann.points || []}
            stroke={isSelected ? '#007bff' : ann.stroke}
            strokeWidth={isSelected ? 3 : ann.strokeWidth}
            fill={isSelected ? '#007bff' : ann.stroke}
            pointerLength={10}
            pointerWidth={10}
            draggable={isSelected}
            onClick={() => setSelectedAnnotation(ann.id)}
            onDragEnd={(e) => handleDragEnd(e, ann.id)}
          />
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
            draggable={isSelected}
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

