import { useEffect, useMemo, useRef, useState } from "react";
import { GRID_SIZE, getSegmentSlots, keyOf, snap } from "../../../shared/lib/geometry.js";

const DEFAULT_VIEWPORT_WIDTH = 1200;
const DEFAULT_VIEWPORT_HEIGHT = 700;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2.5;
const ZOOM_STEP = 0.1;

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
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

function slotId(x, y) {
  return `${x.toFixed(2)}:${y.toFixed(2)}`;
}

function pairKey(a, b) {
  return [a, b].sort().join("|");
}

function getLastPair(ids) {
  if (ids.length < 2) {
    return null;
  }
  return [ids[ids.length - 2], ids[ids.length - 1]];
}

function distance2(a, b) {
  const dx = a.x - b.x;
  const dy = a.y - b.y;
  return dx * dx + dy * dy;
}

function findNearestFreeSlot(point, slots, blocked) {
  let best = null;
  let bestDist = Number.POSITIVE_INFINITY;

  for (const slot of slots) {
    if (blocked.has(slot.id)) {
      continue;
    }
    const d = distance2(point, slot);
    if (d < bestDist) {
      bestDist = d;
      best = slot;
    }
  }

  return best;
}

export default function EditorLayout() {
  const canvasRef = useRef(null);
  const canvasWrapRef = useRef(null);
  const panStateRef = useRef(null);

  const [mode, setMode] = useState("drawTrack");
  const [zoom, setZoom] = useState(1);
  const [camera, setCamera] = useState({ x: -600, y: -350 });
  const [viewport, setViewport] = useState({
    width: DEFAULT_VIEWPORT_WIDTH,
    height: DEFAULT_VIEWPORT_HEIGHT,
  });

  const [segments, setSegments] = useState([]);
  const [startPoint, setStartPoint] = useState(null);
  const [mousePoint, setMousePoint] = useState({ x: 0, y: 0 });
  const [selectedSegmentIds, setSelectedSegmentIds] = useState([]);
  const [dragState, setDragState] = useState(null);
  const [selectionBox, setSelectionBox] = useState(null);
  const [isPanning, setIsPanning] = useState(false);

  const [vehicles, setVehicles] = useState([]);
  const [selectedVehicleIds, setSelectedVehicleIds] = useState([]);
  const [couplings, setCouplings] = useState([]);

  const viewWidth = viewport.width / zoom;
  const viewHeight = viewport.height / zoom;
  const isEditMode = mode === "edit";
  const isPlaceMode = mode === "placeWagon" || mode === "placeLocomotive";

  const selectedSegmentSet = useMemo(() => new Set(selectedSegmentIds), [selectedSegmentIds]);
  const selectedVehicleSet = useMemo(() => new Set(selectedVehicleIds), [selectedVehicleIds]);
  const vehicleById = useMemo(() => new Map(vehicles.map((v) => [v.id, v])), [vehicles]);

  const nodes = useMemo(() => {
    const map = new Map();
    for (const segment of segments) {
      map.set(keyOf(segment.from.x, segment.from.y), segment.from);
      map.set(keyOf(segment.to.x, segment.to.y), segment.to);
    }
    return [...map.values()];
  }, [segments]);

  const railSlots = useMemo(() => {
    const map = new Map();
    for (const segment of segments) {
      for (const point of getSegmentSlots(segment, GRID_SIZE)) {
        const id = slotId(point.x, point.y);
        map.set(id, { id, x: point.x, y: point.y });
      }
    }
    return [...map.values()];
  }, [segments]);

  const adjacentSlotPairs = useMemo(() => {
    const set = new Set();
    for (const segment of segments) {
      const slots = getSegmentSlots(segment, GRID_SIZE);
      for (let i = 0; i < slots.length - 1; i += 1) {
        const a = slotId(slots[i].x, slots[i].y);
        const b = slotId(slots[i + 1].x, slots[i + 1].y);
        set.add(pairKey(a, b));
      }
    }
    return set;
  }, [segments]);

  const occupiedSlots = useMemo(
    () => new Set(vehicles.map((vehicle) => slotId(vehicle.x, vehicle.y))),
    [vehicles]
  );

  const coupledPairSet = useMemo(
    () => new Set(couplings.map((item) => pairKey(item.a, item.b))),
    [couplings]
  );

  useEffect(() => {
    function updateViewport() {
      if (!canvasWrapRef.current) {
        return;
      }
      setViewport({
        width: canvasWrapRef.current.clientWidth,
        height: canvasWrapRef.current.clientHeight,
      });
    }

    updateViewport();
    window.addEventListener("resize", updateViewport);
    return () => window.removeEventListener("resize", updateViewport);
  }, []);

  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key !== "Delete") {
        return;
      }

      if (selectedVehicleIds.length > 0) {
        event.preventDefault();
        const toDelete = new Set(selectedVehicleIds);
        setVehicles((prev) => prev.filter((v) => !toDelete.has(v.id)));
        setCouplings((prev) => prev.filter((c) => !toDelete.has(c.a) && !toDelete.has(c.b)));
        setSelectedVehicleIds([]);
        return;
      }

      if (selectedSegmentIds.length > 0) {
        event.preventDefault();
        setSegments((prev) => prev.filter((segment) => !selectedSegmentSet.has(segment.id)));
        setSelectedSegmentIds([]);
        setDragState(null);
        setSelectionBox(null);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedSegmentIds, selectedSegmentSet, selectedVehicleIds]);

  useEffect(() => {
    if (!isPanning) {
      return;
    }

    function handlePanMove(event) {
      if (!panStateRef.current) {
        return;
      }
      const deltaX = event.clientX - panStateRef.current.startX;
      const deltaY = event.clientY - panStateRef.current.startY;
      setCamera({
        x: panStateRef.current.startCamera.x - deltaX / zoom,
        y: panStateRef.current.startCamera.y - deltaY / zoom,
      });
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
  }, [isPanning, zoom]);

  useEffect(() => {
    if (railSlots.length === 0 || vehicles.length === 0) {
      return;
    }

    setVehicles((prev) => {
      const blocked = new Set();
      const next = [];
      let changed = false;

      for (const vehicle of prev) {
        const nearest = findNearestFreeSlot(vehicle, railSlots, blocked);
        if (!nearest) {
          next.push(vehicle);
          continue;
        }
        blocked.add(nearest.id);
        if (vehicle.x !== nearest.x || vehicle.y !== nearest.y) {
          changed = true;
          next.push({ ...vehicle, x: nearest.x, y: nearest.y });
        } else {
          next.push(vehicle);
        }
      }

      return changed ? next : prev;
    });
  }, [railSlots, vehicles.length]);

  function getWorldPoint(event, snapPoint = true) {
    if (!canvasRef.current) {
      return { x: 0, y: 0 };
    }

    const rect = canvasRef.current.getBoundingClientRect();
    const x = camera.x + ((event.clientX - rect.left) / rect.width) * viewWidth;
    const y = camera.y + ((event.clientY - rect.top) / rect.height) * viewHeight;

    if (snapPoint) {
      return { x: snap(x), y: snap(y) };
    }
    return { x, y };
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
        return { ...segment, [match.endpoint]: point };
      })
    );
  }

  function handleCanvasMouseDown(event) {
    if (event.button !== 0 || !isEditMode || isPanning) {
      return;
    }
    const point = getWorldPoint(event, false);
    setSelectionBox({ start: point, end: point });
    setDragState(null);
  }

  function handleCanvasClick(event) {
    if (mode !== "drawTrack" || isPanning) {
      return;
    }

    const point = getWorldPoint(event, true);
    if (!startPoint) {
      setStartPoint(point);
      return;
    }

    if (startPoint.x === point.x && startPoint.y === point.y) {
      return;
    }

    setSegments((prev) => [
      ...prev,
      { id: crypto.randomUUID(), from: startPoint, to: point },
    ]);
    setStartPoint(null);
  }

  function handleMouseMove(event) {
    const point = getWorldPoint(event, true);
    const rawPoint = getWorldPoint(event, false);
    setMousePoint(point);

    if (selectionBox && isEditMode) {
      setSelectionBox((prev) => (prev ? { ...prev, end: rawPoint } : prev));
      return;
    }

    if (!dragState || !isEditMode) {
      return;
    }

    if (dragState.type === "vehicle") {
      const dx = rawPoint.x - dragState.startMouse.x;
      const dy = rawPoint.y - dragState.startMouse.y;
      const originMap = new Map(dragState.origins.map((item) => [item.id, item]));
      setVehicles((prev) =>
        prev.map((vehicle) => {
          const origin = originMap.get(vehicle.id);
          if (!origin) {
            return vehicle;
          }
          return {
            ...vehicle,
            x: origin.x + dx,
            y: origin.y + dy,
          };
        })
      );
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
    if (dragState?.type === "vehicle") {
      const movedIds = new Set(dragState.origins.map((item) => item.id));
      const originMap = new Map(dragState.origins.map((item) => [item.id, item]));
      const blocked = new Set(
        vehicles
          .filter((vehicle) => !movedIds.has(vehicle.id))
          .map((vehicle) => slotId(vehicle.x, vehicle.y))
      );
      const snappedPositions = new Map();
      let valid = true;

      for (const vehicle of vehicles) {
        if (!movedIds.has(vehicle.id)) {
          continue;
        }
        const nearest = findNearestFreeSlot(vehicle, railSlots, blocked);
        if (!nearest) {
          valid = false;
          break;
        }
        snappedPositions.set(vehicle.id, { x: nearest.x, y: nearest.y, slot: nearest.id });
        blocked.add(nearest.id);
      }

      if (valid) {
        const nextPositionByVehicleId = new Map();
        for (const vehicle of vehicles) {
          const snapped = snappedPositions.get(vehicle.id);
          if (snapped) {
            nextPositionByVehicleId.set(vehicle.id, { x: snapped.x, y: snapped.y });
          } else {
            nextPositionByVehicleId.set(vehicle.id, { x: vehicle.x, y: vehicle.y });
          }
        }

        for (const coupling of couplings) {
          const a = nextPositionByVehicleId.get(coupling.a);
          const b = nextPositionByVehicleId.get(coupling.b);
          if (!a || !b) {
            continue;
          }
          const pair = pairKey(slotId(a.x, a.y), slotId(b.x, b.y));
          if (!adjacentSlotPairs.has(pair)) {
            valid = false;
            break;
          }
        }
      }

      if (!valid) {
        setVehicles((prev) =>
          prev.map((vehicle) => {
            const origin = originMap.get(vehicle.id);
            if (!origin) {
              return vehicle;
            }
            return { ...vehicle, x: origin.x, y: origin.y };
          })
        );
      } else {
        setVehicles((prev) =>
          prev.map((vehicle) => {
            const target = snappedPositions.get(vehicle.id);
            if (!target) {
              return vehicle;
            }
            return { ...vehicle, x: target.x, y: target.y };
          })
        );
      }
    }

    setDragState(null);

    if (!selectionBox) {
      return;
    }

    const rect = normalizeRect(selectionBox.start, selectionBox.end);
    const width = rect.right - rect.left;
    const height = rect.bottom - rect.top;
    if (width < 4 && height < 4) {
      setSelectedSegmentIds([]);
      setSelectedVehicleIds([]);
      setSelectionBox(null);
      return;
    }

    const ids = segments
      .filter((segment) => segmentIntersectsRect(segment, rect))
      .map((segment) => segment.id);
    const vehicleIds = vehicles
      .filter(
        (vehicle) =>
          vehicle.x >= rect.left &&
          vehicle.x <= rect.right &&
          vehicle.y >= rect.top &&
          vehicle.y <= rect.bottom
      )
      .map((vehicle) => vehicle.id);
    setSelectedSegmentIds(ids);
    setSelectedVehicleIds(vehicleIds);
    setSelectionBox(null);
  }

  function handleCanvasWrapMouseDown(event) {
    if (event.button !== 1) {
      return;
    }
    event.preventDefault();
    panStateRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      startCamera: { ...camera },
    };
    setIsPanning(true);
  }

  function startLineDrag(event, segment) {
    if (event.button !== 0 || !isEditMode || isPanning) {
      return;
    }
    event.stopPropagation();

    if (event.shiftKey) {
      return;
    }

    const startMouse = getWorldPoint(event, true);

    if (selectedSegmentSet.has(segment.id) && selectedSegmentIds.length > 1) {
      setDragState({
        type: "multi-line",
        startMouse,
        origins: segments
          .filter((item) => selectedSegmentSet.has(item.id))
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
    if (event.button !== 0 || !isEditMode || isPanning) {
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

  function handleSlotClick(event, slot) {
    if (!isPlaceMode) {
      return;
    }
    event.stopPropagation();
    if (occupiedSlots.has(slot.id)) {
      return;
    }

    const type = mode === "placeLocomotive" ? "locomotive" : "wagon";
    setVehicles((prev) => [
      ...prev,
      { id: crypto.randomUUID(), type, x: slot.x, y: slot.y },
    ]);
  }

  function handleVehicleClick(event, vehicleId) {
    event.stopPropagation();

    if (event.shiftKey) {
      setSelectedVehicleIds((prev) => {
        if (prev.includes(vehicleId)) {
          return prev.filter((id) => id !== vehicleId);
        }
        return [...prev, vehicleId];
      });
      return;
    }

    setSelectedVehicleIds((prev) => {
      if (prev.length === 1 && prev[0] !== vehicleId) {
        return [prev[0], vehicleId];
      }
      return [vehicleId];
    });
  }

  function startVehicleDrag(event, vehicleId) {
    if (event.button !== 0 || !isEditMode || isPanning) {
      return;
    }
    event.stopPropagation();

    if (event.shiftKey) {
      return;
    }

    const movingIds = selectedVehicleIds.includes(vehicleId)
      ? selectedVehicleIds
      : [vehicleId];

    const movingIdSet = new Set(movingIds);
    setDragState({
      type: "vehicle",
      startMouse: getWorldPoint(event, false),
      origins: vehicles
        .filter((vehicle) => movingIdSet.has(vehicle.id))
        .map((vehicle) => ({ id: vehicle.id, x: vehicle.x, y: vehicle.y })),
    });
    setSelectionBox(null);
  }

  function coupleSelectedVehicles() {
    const pair = getLastPair(selectedVehicleIds);
    if (!pair) {
      return;
    }
    const [a, b] = pair;
    const va = vehicleById.get(a);
    const vb = vehicleById.get(b);
    if (!va || !vb) {
      return;
    }

    const slotsPair = pairKey(slotId(va.x, va.y), slotId(vb.x, vb.y));
    if (!adjacentSlotPairs.has(slotsPair)) {
      return;
    }

    const key = pairKey(a, b);
    if (coupledPairSet.has(key)) {
      return;
    }
    setCouplings((prev) => [...prev, { id: crypto.randomUUID(), a, b }]);
  }

  function decoupleSelectedVehicles() {
    const pair = getLastPair(selectedVehicleIds);
    if (!pair) {
      return;
    }
    const [a, b] = pair;
    const key = pairKey(a, b);
    setCouplings((prev) => prev.filter((item) => pairKey(item.a, item.b) !== key));
  }

  function clearLayout() {
    setSegments([]);
    setVehicles([]);
    setCouplings([]);
    setStartPoint(null);
    setSelectedSegmentIds([]);
    setSelectedVehicleIds([]);
    setDragState(null);
    setSelectionBox(null);
  }

  function deleteSelectedSegments() {
    if (!selectedSegmentIds.length) {
      return;
    }
    setSegments((prev) => prev.filter((segment) => !selectedSegmentSet.has(segment.id)));
    setSelectedSegmentIds([]);
    setDragState(null);
    setSelectionBox(null);
  }

  function deleteSelectedVehicles() {
    if (!selectedVehicleIds.length) {
      return;
    }
    const toDelete = new Set(selectedVehicleIds);
    setVehicles((prev) => prev.filter((v) => !toDelete.has(v.id)));
    setCouplings((prev) => prev.filter((c) => !toDelete.has(c.a) && !toDelete.has(c.b)));
    setSelectedVehicleIds([]);
  }

  function deleteSelectedAll() {
    deleteSelectedVehicles();
    deleteSelectedSegments();
  }

  function switchMode(nextMode) {
    setMode(nextMode);
    setStartPoint(null);
    setDragState(null);
    setSelectionBox(null);
    if (nextMode !== "edit") {
      setSelectedSegmentIds([]);
      setSelectedVehicleIds([]);
    }
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

  const majorGrid = GRID_SIZE * 5;
  const selectionRect = selectionBox ? normalizeRect(selectionBox.start, selectionBox.end) : null;
  const activeModeLabel =
    mode === "drawTrack"
      ? "Добавление пути"
      : mode === "placeWagon"
        ? "Добавление вагонов"
        : mode === "placeLocomotive"
          ? "Добавление локомотивов"
          : "Редактирование";

  return (
    <div className="layout">
      <aside className="sidebar">
        <h1 className="title">Trains Lab</h1>
        <p className="subtitle">Локомотивные маневры</p>

        <div className="tools">
          <button
            type="button"
            className={`toolButton ${mode === "drawTrack" ? "active" : ""}`}
            onClick={() => switchMode("drawTrack")}
          >
            Добавление пути
          </button>
          <button
            type="button"
            className={`toolButton ${mode === "placeWagon" ? "active" : ""}`}
            onClick={() => switchMode("placeWagon")}
          >
            Добавление вагонов
          </button>
          <button
            type="button"
            className={`toolButton ${mode === "placeLocomotive" ? "active" : ""}`}
            onClick={() => switchMode("placeLocomotive")}
          >
            Добавление локомотивов
          </button>
          <button
            type="button"
            className={`toolButton ${mode === "edit" ? "active" : ""}`}
            onClick={() => switchMode("edit")}
          >
            Редактирование
          </button>
        </div>

        <p className="counter">Сцепка/расцепка</p>
        <div className="tools">
          <button type="button" className="toolButton" onClick={coupleSelectedVehicles}>
            Сцепить выбранные
          </button>
          <button type="button" className="toolButton" onClick={decoupleSelectedVehicles}>
            Расцепить выбранные
          </button>
          <button type="button" className="toolButton" onClick={deleteSelectedSegments}>
            Удалить выбранные пути
          </button>
          <button type="button" className="toolButton" onClick={deleteSelectedVehicles}>
            Удалить выбранные составы
          </button>
          <button type="button" className="toolButton" onClick={deleteSelectedAll}>
            Удалить всё выбранное
          </button>
          <button type="button" className="toolButton" onClick={clearLayout}>
            Очистить все
          </button>
        </div>

        <p className="counter">Путей: {segments.length}</p>
        <p className="counter">Составов: {vehicles.length}</p>
        <p className="counter">Сцепок: {couplings.length}</p>
        <p className="counter">Выбрано составов: {selectedVehicleIds.length}</p>
      </aside>

      <main className="workspace">
        <header className="toolbar">
          <div>Режим: <strong>{activeModeLabel}</strong></div>
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
            viewBox={`${camera.x} ${camera.y} ${viewWidth} ${viewHeight}`}
            width="100%"
            height="100%"
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

            <rect x={camera.x} y={camera.y} width={viewWidth} height={viewHeight} fill="#ffffff" />
            <rect x={camera.x} y={camera.y} width={viewWidth} height={viewHeight} fill="url(#rail-grid-minor)" />
            <rect x={camera.x} y={camera.y} width={viewWidth} height={viewHeight} fill="url(#rail-grid-major)" />

            {segments.map((segment) => (
              <line
                key={segment.id}
                x1={segment.from.x}
                y1={segment.from.y}
                x2={segment.to.x}
                y2={segment.to.y}
                stroke={selectedSegmentSet.has(segment.id) ? "#2563eb" : "#334155"}
                strokeWidth={selectedSegmentSet.has(segment.id) ? "8" : "6"}
                strokeLinecap="round"
                className={isEditMode ? "draggableLine" : ""}
                onMouseDown={(event) => startLineDrag(event, segment)}
                onClick={(event) => {
                  if (!isEditMode) {
                    return;
                  }
                  event.stopPropagation();
                  if (event.shiftKey) {
                    setSelectedSegmentIds((prev) =>
                      prev.includes(segment.id)
                        ? prev.filter((id) => id !== segment.id)
                        : [...prev, segment.id]
                    );
                    return;
                  }
                  setSelectedSegmentIds([segment.id]);
                }}
              />
            ))}

            {couplings.map((coupling) => {
              const a = vehicleById.get(coupling.a);
              const b = vehicleById.get(coupling.b);
              if (!a || !b) {
                return null;
              }
              return (
                <line
                  key={coupling.id}
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  stroke="#f97316"
                  strokeWidth="10"
                  strokeLinecap="round"
                />
              );
            })}

            {railSlots.map((slot) => (
              <circle
                key={`slot-${slot.id}`}
                cx={slot.x}
                cy={slot.y}
                r="4.5"
                fill={occupiedSlots.has(slot.id) ? "#94a3b8" : "#cbd5e1"}
                className="slotPoint"
                onClick={(event) => handleSlotClick(event, slot)}
              />
            ))}

            {vehicles.map((vehicle) => (
              <rect
                key={vehicle.id}
                x={vehicle.x - GRID_SIZE / 2 + 6}
                y={vehicle.y - GRID_SIZE / 2 + 6}
                width={GRID_SIZE - 12}
                height={GRID_SIZE - 12}
                rx="8"
                fill={vehicle.type === "locomotive" ? "#dc2626" : "#0ea5e9"}
                stroke={selectedVehicleSet.has(vehicle.id) ? "#facc15" : vehicle.type === "locomotive" ? "#7f1d1d" : "#0c4a6e"}
                strokeWidth={selectedVehicleSet.has(vehicle.id) ? "4" : "2"}
                className={isEditMode ? "slotPoint" : ""}
                onMouseDown={(event) => startVehicleDrag(event, vehicle.id)}
                onClick={(event) => handleVehicleClick(event, vehicle.id)}
              />
            ))}

            {nodes.map((node) => (
              <circle
                key={keyOf(node.x, node.y)}
                cx={node.x}
                cy={node.y}
                r={isEditMode ? 7 : 4}
                fill={isEditMode ? "#60a5fa" : "#0f172a"}
                stroke={isEditMode ? "#1e3a8a" : "none"}
                strokeWidth={isEditMode ? "2" : "0"}
                className={isEditMode ? "draggablePoint" : ""}
                onMouseDown={(event) => startNodeDrag(event, node)}
              />
            ))}

            {mode === "drawTrack" && startPoint && (
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

            {isEditMode && selectionRect && (
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
          X: {mousePoint.x} | Y: {mousePoint.y} | Zoom: {Math.round(zoom * 100)}% | Grid: {GRID_SIZE}
        </footer>
      </main>
    </div>
  );
}
