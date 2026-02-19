import { useEffect, useMemo, useRef, useState } from "react";
import { GRID_SIZE, keyOf, snap } from "../../../shared/lib/geometry.js";

const CANVAS_WIDTH = 1200;
const CANVAS_HEIGHT = 700;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2;
const ZOOM_STEP = 0.1;

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function toCanvasPointFromRect(event, rect) {
  const scaleX = CANVAS_WIDTH / rect.width;
  const scaleY = CANVAS_HEIGHT / rect.height;
  const x = (event.clientX - rect.left) * scaleX;
  const y = (event.clientY - rect.top) * scaleY;
  return { x: snap(x), y: snap(y) };
}

function toCanvasPointRawFromRect(event, rect) {
  const scaleX = CANVAS_WIDTH / rect.width;
  const scaleY = CANVAS_HEIGHT / rect.height;
  const x = (event.clientX - rect.left) * scaleX;
  const y = (event.clientY - rect.top) * scaleY;
  return { x, y };
}

function toCanvasPoint(event) {
  return toCanvasPointFromRect(event, event.currentTarget.getBoundingClientRect());
}

function normalizeRect(start, end) {
  return {
    left: Math.min(start.x, end.x),
    right: Math.max(start.x, end.x),
    top: Math.min(start.y, end.y),
    bottom: Math.max(start.y, end.y),
  };
}

function segmentIntersectsRect(segment, rect) {
  const lineRect = normalizeRect(segment.from, segment.to);
  return (
    lineRect.right >= rect.left &&
    lineRect.left <= rect.right &&
    lineRect.bottom >= rect.top &&
    lineRect.top <= rect.bottom
  );
}

export default function EditorLayout() {
  const canvasRef = useRef(null);
  const canvasWrapRef = useRef(null);
  const panStateRef = useRef(null);
  const [tool, setTool] = useState("draw");
  const [zoom, setZoom] = useState(1);
  const [segments, setSegments] = useState([]);
  const [startPoint, setStartPoint] = useState(null);
  const [mousePoint, setMousePoint] = useState({ x: 0, y: 0 });
  const [selectedSegmentIds, setSelectedSegmentIds] = useState([]);
  const [dragState, setDragState] = useState(null);
  const [selectionBox, setSelectionBox] = useState(null);
  const [isPanning, setIsPanning] = useState(false);

  const nodes = useMemo(() => {
    const map = new Map();
    for (const segment of segments) {
      const fromKey = keyOf(segment.from.x, segment.from.y);
      const toKey = keyOf(segment.to.x, segment.to.y);

      if (!map.has(fromKey)) {
        map.set(fromKey, { x: segment.from.x, y: segment.from.y, links: 0 });
      }
      if (!map.has(toKey)) {
        map.set(toKey, { x: segment.to.x, y: segment.to.y, links: 0 });
      }
      map.get(fromKey).links += 1;
      map.get(toKey).links += 1;
    }
    return [...map.values()];
  }, [segments]);

  const selectedSet = useMemo(() => new Set(selectedSegmentIds), [selectedSegmentIds]);

  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key !== "Delete" || !selectedSegmentIds.length) {
        return;
      }
      event.preventDefault();
      setSegments((prev) => prev.filter((segment) => !selectedSet.has(segment.id)));
      setSelectedSegmentIds([]);
      setDragState(null);
      setSelectionBox(null);
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedSegmentIds, selectedSet]);

  useEffect(() => {
    if (!isPanning) {
      return;
    }

    function handlePanMove(event) {
      if (!panStateRef.current || !canvasWrapRef.current) {
        return;
      }

      const deltaX = event.clientX - panStateRef.current.startX;
      const deltaY = event.clientY - panStateRef.current.startY;
      canvasWrapRef.current.scrollLeft = panStateRef.current.scrollLeft - deltaX;
      canvasWrapRef.current.scrollTop = panStateRef.current.scrollTop - deltaY;
    }

    function handlePanEnd() {
      panStateRef.current = null;
      setIsPanning(false);
    }

    window.addEventListener("mousemove", handlePanMove);
    window.addEventListener("mouseup", handlePanEnd);
    return () => {
      window.removeEventListener("mousemove", handlePanMove);
      window.removeEventListener("mouseup", handlePanEnd);
    };
  }, [isPanning]);

  function deleteSelected() {
    if (!selectedSegmentIds.length) {
      return;
    }
    setSegments((prev) => prev.filter((segment) => !selectedSet.has(segment.id)));
    setSelectedSegmentIds([]);
    setDragState(null);
    setSelectionBox(null);
  }

  function updateSegment(segmentId, updater) {
    setSegments((prev) =>
      prev.map((segment) => (segment.id === segmentId ? updater(segment) : segment))
    );
  }

  function moveConnectedNode(affectedEndpoints, point) {
    setSegments((prev) =>
      prev.map((segment) => {
        const match = affectedEndpoints.find((item) => item.segmentId === segment.id);
        if (!match) {
          return segment;
        }
        return {
          ...segment,
          [match.endpoint]: point,
        };
      })
    );
  }

  function handleCanvasClick(event) {
    if (tool !== "draw") {
      return;
    }

    const point = toCanvasPoint(event);
    if (!startPoint) {
      setStartPoint(point);
      return;
    }

    if (startPoint.x === point.x && startPoint.y === point.y) {
      return;
    }

    setSegments((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        from: startPoint,
        to: point,
      },
    ]);
    setStartPoint(null);
  }

  function handleCanvasMouseDown(event) {
    if (tool !== "select") {
      return;
    }
    const point = toCanvasPointRawFromRect(event, event.currentTarget.getBoundingClientRect());
    setSelectionBox({ start: point, end: point });
    setDragState(null);
  }

  function handleMouseMove(event) {
    const point = toCanvasPoint(event);
    setMousePoint(point);

    if (selectionBox && tool === "select") {
      const rawPoint = toCanvasPointRawFromRect(event, event.currentTarget.getBoundingClientRect());
      setSelectionBox((prev) => (prev ? { ...prev, end: rawPoint } : prev));
      return;
    }

    if (!dragState || tool !== "select") {
      return;
    }

    if (dragState.type === "line") {
      const dx = snap(point.x - dragState.startMouse.x);
      const dy = snap(point.y - dragState.startMouse.y);
      updateSegment(dragState.segmentId, (segment) => ({
        ...segment,
        from: { x: dragState.originFrom.x + dx, y: dragState.originFrom.y + dy },
        to: { x: dragState.originTo.x + dx, y: dragState.originTo.y + dy },
      }));
      return;
    }

    if (dragState.type === "multi-line") {
      const dx = snap(point.x - dragState.startMouse.x);
      const dy = snap(point.y - dragState.startMouse.y);
      const originMap = new Map(dragState.origins.map((item) => [item.segmentId, item]));
      setSegments((prev) =>
        prev.map((segment) => {
          const origin = originMap.get(segment.id);
          if (!origin) {
            return segment;
          }
          return {
            ...segment,
            from: { x: origin.from.x + dx, y: origin.from.y + dy },
            to: { x: origin.to.x + dx, y: origin.to.y + dy },
          };
        })
      );
      return;
    }

    if (dragState.type === "node") {
      moveConnectedNode(dragState.affectedEndpoints, point);
    }
  }

  function handleMouseUp() {
    setDragState(null);
    if (!selectionBox) {
      return;
    }

    const rect = normalizeRect(selectionBox.start, selectionBox.end);
    const width = rect.right - rect.left;
    const height = rect.bottom - rect.top;
    if (width < 4 && height < 4) {
      setSelectedSegmentIds([]);
      setSelectionBox(null);
      return;
    }

    const ids = segments
      .filter((segment) => segmentIntersectsRect(segment, rect))
      .map((segment) => segment.id);
    setSelectedSegmentIds(ids);
    setSelectionBox(null);
  }

  function clearLayout() {
    setSegments([]);
    setStartPoint(null);
    setSelectedSegmentIds([]);
    setDragState(null);
    setSelectionBox(null);
  }

  function switchTool(nextTool) {
    setTool(nextTool);
    setStartPoint(null);
    setDragState(null);
    setSelectionBox(null);
    if (nextTool !== "select") {
      setSelectedSegmentIds([]);
    }
  }

  function startLineDrag(event, segment) {
    if (tool !== "select") {
      return;
    }
    event.stopPropagation();
    const rect = canvasRef.current.getBoundingClientRect();
    const startMouse = toCanvasPointFromRect(event, rect);

    if (selectedSet.has(segment.id) && selectedSegmentIds.length > 1) {
      setDragState({
        type: "multi-line",
        startMouse,
        origins: segments
          .filter((item) => selectedSet.has(item.id))
          .map((item) => ({
            segmentId: item.id,
            from: item.from,
            to: item.to,
          })),
      });
      setSelectionBox(null);
      return;
    }

    setSelectedSegmentIds([segment.id]);
    setDragState({
      type: "line",
      segmentId: segment.id,
      startMouse,
      originFrom: segment.from,
      originTo: segment.to,
    });
    setSelectionBox(null);
  }

  function startNodeDrag(event, node) {
    if (tool !== "select") {
      return;
    }
    event.stopPropagation();

    const nodeKey = keyOf(node.x, node.y);
    const affectedEndpoints = [];
    const affectedIds = [];
    for (const segment of segments) {
      if (keyOf(segment.from.x, segment.from.y) === nodeKey) {
        affectedEndpoints.push({ segmentId: segment.id, endpoint: "from" });
        affectedIds.push(segment.id);
      }
      if (keyOf(segment.to.x, segment.to.y) === nodeKey) {
        affectedEndpoints.push({ segmentId: segment.id, endpoint: "to" });
        affectedIds.push(segment.id);
      }
    }

    setSelectedSegmentIds([...new Set(affectedIds)]);
    setDragState({ type: "node", affectedEndpoints });
    setSelectionBox(null);
  }

  function zoomIn() {
    setZoom((prev) => clamp(Number((prev + ZOOM_STEP).toFixed(2)), MIN_ZOOM, MAX_ZOOM));
  }

  function zoomOut() {
    setZoom((prev) => clamp(Number((prev - ZOOM_STEP).toFixed(2)), MIN_ZOOM, MAX_ZOOM));
  }

  function resetZoom() {
    setZoom(1);
  }

  function handleCanvasWrapMouseDown(event) {
    if (event.button !== 1 || !canvasWrapRef.current) {
      return;
    }

    event.preventDefault();
    panStateRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      scrollLeft: canvasWrapRef.current.scrollLeft,
      scrollTop: canvasWrapRef.current.scrollTop,
    };
    setIsPanning(true);
  }

  const selectionRect = selectionBox ? normalizeRect(selectionBox.start, selectionBox.end) : null;
  const majorGrid = GRID_SIZE * 5;

  return (
    <div className="layout">
      <aside className="sidebar">
        <h1 className="title">Trains Lab</h1>
        <p className="subtitle">Локомотивные маневры</p>

        <div className="tools">
          <button
            type="button"
            className={`toolButton ${tool === "draw" ? "active" : ""}`}
            onClick={() => switchTool("draw")}
          >
            Прокладка пути
          </button>
          <button
            type="button"
            className={`toolButton ${tool === "select" ? "active" : ""}`}
            onClick={() => switchTool("select")}
          >
            Выделение
          </button>
          <button type="button" className="toolButton" onClick={deleteSelected}>
            Удалить выбранное
          </button>
          <button type="button" className="toolButton" onClick={clearLayout}>
            Очистить всё
          </button>
        </div>

        <p className="counter">Сегментов: {segments.length}</p>
        <p className="counter">Выделено: {selectedSegmentIds.length}</p>
      </aside>

      <main className="workspace">
        <header className="toolbar">
          <div>
            Текущий инструмент:{" "}
            <strong>{tool === "draw" ? "Прокладка пути" : "Выделение"}</strong>
          </div>
          <div className="zoomControls">
            <button type="button" className="zoomButton" onClick={zoomOut}>
              -
            </button>
            <button type="button" className="zoomButton" onClick={resetZoom}>
              {Math.round(zoom * 100)}%
            </button>
            <button type="button" className="zoomButton" onClick={zoomIn}>
              +
            </button>
          </div>
        </header>

        <section
          ref={canvasWrapRef}
          className={`canvasWrap ${isPanning ? "panning" : ""}`}
          onMouseDown={handleCanvasWrapMouseDown}
          onAuxClick={(event) => {
            if (event.button === 1) {
              event.preventDefault();
            }
          }}
        >
          <svg
            ref={canvasRef}
            className="canvas"
            viewBox={`0 0 ${CANVAS_WIDTH} ${CANVAS_HEIGHT}`}
            width={CANVAS_WIDTH * zoom}
            height={CANVAS_HEIGHT * zoom}
            onClick={handleCanvasClick}
            onMouseDown={handleCanvasMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
          >
            <defs>
              <pattern id="rail-grid-minor" width={GRID_SIZE} height={GRID_SIZE} patternUnits="userSpaceOnUse">
                <path
                  d={`M ${GRID_SIZE} 0 L 0 0 0 ${GRID_SIZE}`}
                  fill="none"
                  stroke="#d8e2ee"
                  strokeWidth="1"
                  shapeRendering="crispEdges"
                />
              </pattern>
              <pattern id="rail-grid-major" width={majorGrid} height={majorGrid} patternUnits="userSpaceOnUse">
                <path
                  d={`M ${majorGrid} 0 L 0 0 0 ${majorGrid}`}
                  fill="none"
                  stroke="#b6c7db"
                  strokeWidth="1.2"
                  shapeRendering="crispEdges"
                />
              </pattern>
            </defs>

            <rect width={CANVAS_WIDTH} height={CANVAS_HEIGHT} fill="#ffffff" />
            <rect width={CANVAS_WIDTH} height={CANVAS_HEIGHT} fill="url(#rail-grid-minor)" />
            <rect width={CANVAS_WIDTH} height={CANVAS_HEIGHT} fill="url(#rail-grid-major)" />

            {segments.map((segment) => (
              <line
                key={segment.id}
                x1={segment.from.x}
                y1={segment.from.y}
                x2={segment.to.x}
                y2={segment.to.y}
                stroke={selectedSet.has(segment.id) ? "#2563eb" : "#334155"}
                strokeWidth={selectedSet.has(segment.id) ? "8" : "6"}
                strokeLinecap="round"
                className={tool === "select" ? "draggableLine" : ""}
                onMouseDown={(event) => startLineDrag(event, segment)}
                onClick={(event) => {
                  if (tool !== "select") {
                    return;
                  }
                  event.stopPropagation();
                  setSelectedSegmentIds([segment.id]);
                }}
              />
            ))}

            {nodes.map((node) => (
              <circle
                key={keyOf(node.x, node.y)}
                cx={node.x}
                cy={node.y}
                r={tool === "select" ? 8 : 4}
                fill={tool === "select" ? "#60a5fa" : "#0f172a"}
                stroke={tool === "select" ? "#1e3a8a" : "none"}
                strokeWidth={tool === "select" ? "2" : "0"}
                className={tool === "select" ? "draggablePoint" : ""}
                onMouseDown={(event) => startNodeDrag(event, node)}
              />
            ))}

            {tool === "draw" && startPoint && (
              <line
                x1={startPoint.x}
                y1={startPoint.y}
                x2={mousePoint.x}
                y2={mousePoint.y}
                stroke="#2563eb"
                strokeWidth="4"
                strokeDasharray="10 8"
                strokeLinecap="round"
              />
            )}

            {tool === "select" && selectionRect && (
              <rect
                className="selectionBox"
                x={selectionRect.left}
                y={selectionRect.top}
                width={Math.max(selectionRect.right - selectionRect.left, 1)}
                height={Math.max(selectionRect.bottom - selectionRect.top, 1)}
              />
            )}
          </svg>
        </section>

        <footer className="statusbar">
          X: {mousePoint.x} | Y: {mousePoint.y} | Zoom: {Math.round(zoom * 100)}% |{" "}
          {tool === "draw"
            ? startPoint
              ? "Выбрана начальная точка"
              : "Ожидание первого клика"
            : "Рамкой выделяй линии, Del или кнопка удаляют выбранные"}
        </footer>
      </main>
    </div>
  );
}
