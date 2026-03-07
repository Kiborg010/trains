import { useEffect, useMemo, useRef, useState } from "react";
import { GRID_SIZE, getSegmentSlots, keyOf, snap } from "../../../shared/lib/geometry.js";
import {
  addScenarioCommand,
  applyLayoutOperation,
  createScenario,
  deleteLayout,
  getLayout,
  getScenario,
  listLayouts,
  listScenarios,
  planMovement,
  resolveVehicles,
  saveLayout,
  updateLayout,
} from "../../../shared/api/simulation.js";

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

function hasVehiclePositionChanges(current, next) {
  if (!Array.isArray(next) || current.length !== next.length) {
    return true;
  }
  const nextMap = new Map(next.map((item) => [item.id, item]));
  for (const vehicle of current) {
    const match = nextMap.get(vehicle.id);
    if (!match) {
      return true;
    }
    if (vehicle.x !== match.x || vehicle.y !== match.y || vehicle.type !== match.type) {
      return true;
    }
  }
  return false;
}

function normalizeUnitCode(value) {
  const raw = String(value || "").trim().toLowerCase();
  if (!raw) {
    return "";
  }

  const first = raw[0];
  const normalizedPrefix =
    first === "l" ? "л" : first === "v" ? "в" : first;
  if (normalizedPrefix !== "л" && normalizedPrefix !== "в") {
    return "";
  }

  const digits = raw.slice(1).replace(/[^\d]/g, "");
  if (!digits) {
    return "";
  }

  return `${normalizedPrefix}${digits}`;
}

function buildVehicleCodeMap(vehicles) {
  const map = new Map();

  let locoCounter = 0;
  let wagonCounter = 0;

  for (const vehicle of vehicles) {
    const normalizedCode = normalizeUnitCode(vehicle.code);
    if (!normalizedCode) {
      continue;
    }

    if (vehicle.type === "locomotive" && normalizedCode.startsWith("л")) {
      map.set(vehicle.id, normalizedCode);
      const n = Number.parseInt(normalizedCode.slice(1), 10);
      if (!Number.isNaN(n)) {
        locoCounter = Math.max(locoCounter, n);
      }
      continue;
    }

    if (vehicle.type === "wagon" && normalizedCode.startsWith("в")) {
      map.set(vehicle.id, normalizedCode);
      const n = Number.parseInt(normalizedCode.slice(1), 10);
      if (!Number.isNaN(n)) {
        wagonCounter = Math.max(wagonCounter, n);
      }
    }
  }

  for (const vehicle of vehicles) {
    if (map.has(vehicle.id)) {
      continue;
    }

    if (vehicle.type === "locomotive") {
      locoCounter += 1;
      map.set(vehicle.id, `л${locoCounter}`);
    } else {
      wagonCounter += 1;
      map.set(vehicle.id, `в${wagonCounter}`);
    }
  }
  return map;
}

function applyTimelineStepToVehicles(vehicles, step) {
  const stepMap = new Map(step.map((item) => [item.id, item]));
  return vehicles.map((vehicle) => {
    const next = stepMap.get(vehicle.id);
    if (!next) {
      return vehicle;
    }
    return { ...vehicle, x: next.x, y: next.y };
  });
}

function buildScenarioStepsFromBackendScenario(scenario) {
  if (!scenario || !Array.isArray(scenario.commands)) {
    return [];
  }

  const steps = [];
  const positionsByVehicleId = new Map();
  const codeByVehicleId = buildVehicleCodeMap(scenario.initialState?.vehicles || []);

  for (const vehicle of scenario.initialState?.vehicles || []) {
    positionsByVehicleId.set(vehicle.id, {
      pathId: vehicle.pathId || "",
      pathIndex: Number(vehicle.pathIndex ?? 0),
    });
  }

  for (const command of scenario.commands) {
    if (command.type !== "MOVE_LOCO") {
      continue;
    }
    const payload = command.payload || {};
    const locoId = payload.locoId;
    const fromPos = positionsByVehicleId.get(locoId) || { pathId: "", pathIndex: 0 };
    const toPathId = String(payload.targetPathId || "").trim();
    const toIndex = Number(payload.targetIndex ?? 0);

    steps.push({
      id: command.id || crypto.randomUUID(),
      unitCode: normalizeUnitCode(codeByVehicleId.get(locoId)) || "",
      fromPathId: fromPos.pathId,
      fromIndex: Number(fromPos.pathIndex ?? 0),
      toPathId,
      toIndex,
    });

    positionsByVehicleId.set(locoId, {
      pathId: toPathId,
      pathIndex: toIndex,
    });
  }

  return steps;
}

export default function EditorLayout({ activePanel, setActivePanel }) {
  const canvasRef = useRef(null);
  const canvasWrapRef = useRef(null);
  const panStateRef = useRef(null);
  const movementTimerRef = useRef(null);

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
  const [selectedLocomotiveId, setSelectedLocomotiveId] = useState(null);
  const [targetPathId, setTargetPathId] = useState("");
  const [targetPathIndex, setTargetPathIndex] = useState(null);
  const [isMoving, setIsMoving] = useState(false);
  const [movementHint, setMovementHint] = useState("");
  const [movementCellsPassed, setMovementCellsPassed] = useState(0);
  const [scenarioUnitCode, setScenarioUnitCode] = useState("");
  const [scenarioFromPathId, setScenarioFromPathId] = useState("");
  const [scenarioFromIndex, setScenarioFromIndex] = useState("");
  const [scenarioToPathId, setScenarioToPathId] = useState("");
  const [scenarioToIndex, setScenarioToIndex] = useState("");
  const [scenarioSteps, setScenarioSteps] = useState([]);
  const [layoutName, setLayoutName] = useState("Схема 1");
  const [savedLayouts, setSavedLayouts] = useState([]);
  const [selectedLayoutId, setSelectedLayoutId] = useState("");
  const [scenarioName, setScenarioName] = useState("Сценарий 1");
  const [savedScenarios, setSavedScenarios] = useState([]);
  const [selectedScenarioId, setSelectedScenarioId] = useState("");

  const viewWidth = viewport.width / zoom;
  const viewHeight = viewport.height / zoom;
  const isManeuversPanel = activePanel === "maneuvers";
  const isCouplingPanel = activePanel === "coupling";
  const isMovementPanel = activePanel === "movement";
  const isEditMode = isManeuversPanel && mode === "edit";
  const isPlaceMode =
    isManeuversPanel && (mode === "placeWagon" || mode === "placeLocomotive");
  const isMoveMode = isMovementPanel && mode === "move";

  const selectedSegmentSet = useMemo(() => new Set(selectedSegmentIds), [selectedSegmentIds]);
  const selectedVehicleSet = useMemo(() => new Set(selectedVehicleIds), [selectedVehicleIds]);
  const vehicleById = useMemo(() => new Map(vehicles.map((v) => [v.id, v])), [vehicles]);
  const vehicleCodeById = useMemo(() => buildVehicleCodeMap(vehicles), [vehicles]);

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
      const points = getSegmentSlots(segment, GRID_SIZE);
      points.forEach((point, index) => {
        const id = `${segment.id}:${index}`;
        map.set(id, { id, pathId: segment.id, index, x: point.x, y: point.y });
      });
    }
    return [...map.values()];
  }, [segments]);

  const occupiedSlots = useMemo(
    () =>
      new Set(
        vehicles
          .filter((vehicle) => vehicle.pathId != null)
          .map((vehicle) => `${vehicle.pathId}:${vehicle.pathIndex}`)
      ),
    [vehicles]
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
      if (!isManeuversPanel) {
        return;
      }

      if (selectedVehicleIds.length > 0) {
        event.preventDefault();
        void deleteSelectedVehicles();
        return;
      }

      if (selectedSegmentIds.length > 0) {
        event.preventDefault();
        void deleteSelectedSegments();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isManeuversPanel, selectedSegmentIds, selectedVehicleIds]);

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
    return () => {
      if (movementTimerRef.current) {
        clearInterval(movementTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!segments.length || !vehicles.length) {
      return;
    }

    let cancelled = false;
    async function syncVehiclesToRails() {
      try {
        const response = await resolveVehicles({
          gridSize: GRID_SIZE,
          segments,
          vehicles,
          couplings,
          movedVehicleIds: [],
          strictCouplings: true,
        });

        if (!response.ok || cancelled) {
          return;
        }

        setVehicles((prev) =>
          hasVehiclePositionChanges(prev, response.vehicles) ? response.vehicles : prev
        );
      } catch (error) {
        // Keep current state if backend is temporarily unavailable.
      }
    }

    syncVehiclesToRails();
    return () => {
      cancelled = true;
    };
  }, [segments, couplings]);

  useEffect(() => {
    let cancelled = false;

    async function loadSavedData() {
      try {
        const [layoutsResp, scenariosResp] = await Promise.all([
          listLayouts(),
          listScenarios(),
        ]);
        if (!cancelled) {
          setSavedLayouts(layoutsResp.layouts || []);
          setSavedScenarios(scenariosResp.scenarios || []);
        }
      } catch (error) {
        if (!cancelled) {
          setMovementHint("Не удалось загрузить сохраненные схемы/сценарии.");
        }
      }
    }

    loadSavedData();
    return () => {
      cancelled = true;
    };
  }, []);

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

  function stopMovement(clearSelection = false) {
    if (movementTimerRef.current) {
      clearInterval(movementTimerRef.current);
      movementTimerRef.current = null;
    }
    setIsMoving(false);
    if (clearSelection) {
      setSelectedLocomotiveId(null);
      setTargetPathId("");
      setTargetPathIndex(null);
    }
  }

  async function applyLayoutAction(action, payload = {}) {
    const response = await applyLayoutOperation({
      gridSize: GRID_SIZE,
      state: {
        segments,
        vehicles,
        couplings,
      },
      action,
      ...payload,
    });

    if (!response.ok) {
      throw new Error(response.message || "Layout action failed.");
    }

    const nextState = response.state || {};
    setSegments(nextState.segments || []);
    setVehicles(nextState.vehicles || []);
    setCouplings(nextState.couplings || []);
    if (response.message) {
      setMovementHint(response.message);
    }
    return response;
  }

  async function refreshSavedLayouts() {
    const response = await listLayouts();
    setSavedLayouts(response.layouts || []);
  }

  async function refreshSavedScenarios() {
    const response = await listScenarios();
    setSavedScenarios(response.scenarios || []);
  }

  async function handleSaveLayout() {
    try {
      const payload = {
        name: layoutName.trim() || "Схема",
        state: { segments, vehicles, couplings },
      };

      let response;
      if (selectedLayoutId) {
        response = await updateLayout(selectedLayoutId, payload);
      } else {
        response = await saveLayout(payload);
      }

      const saved = response.layout;
      if (saved?.id != null) {
        setSelectedLayoutId(String(saved.id));
      }
      await refreshSavedLayouts();
      setMovementHint("Схема сохранена.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось сохранить схему.");
    }
  }

  async function handleLoadLayout() {
    if (!selectedLayoutId) {
      setMovementHint("Выбери схему для загрузки.");
      return;
    }
    try {
      const response = await getLayout(selectedLayoutId);
      const state = response.layout?.state || {};
      stopMovement(true);
      setSegments(state.segments || []);
      setVehicles(state.vehicles || []);
      setCouplings(state.couplings || []);
      setSelectedSegmentIds([]);
      setSelectedVehicleIds([]);
      setDragState(null);
      setSelectionBox(null);
      setMovementHint("Схема загружена.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось загрузить схему.");
    }
  }

  async function handleDeleteLayout() {
    if (!selectedLayoutId) {
      setMovementHint("Выбери схему для удаления.");
      return;
    }
    try {
      await deleteLayout(selectedLayoutId);
      setSelectedLayoutId("");
      await refreshSavedLayouts();
      setMovementHint("Схема удалена.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось удалить схему.");
    }
  }

  async function handleSaveScenario() {
    try {
      if (!scenarioSteps.length) {
        setMovementHint("Добавь шаги сценария перед сохранением.");
        return;
      }

      const initialState = { segments, vehicles, couplings };
      const createResp = await createScenario({
        name: scenarioName.trim() || "Сценарий",
        initialState,
      });
      if (!createResp.ok || !createResp.scenario?.id) {
        throw new Error(createResp.message || "Не удалось создать сценарий.");
      }

      const codeMap = buildVehicleCodeMap(vehicles);
      const idByCode = new Map();
      for (const vehicle of vehicles) {
        const code = normalizeUnitCode(codeMap.get(vehicle.id));
        if (code) {
          idByCode.set(code, vehicle.id);
        }
      }

      for (const step of scenarioSteps) {
        const code = normalizeUnitCode(step.unitCode);
        const locoId = idByCode.get(code);
        if (!locoId) {
          throw new Error(`Локомотив ${code || step.unitCode} не найден для сохранения.`);
        }

        const commandPayload = {
          type: "MOVE_LOCO",
          payload: {
            locoId,
            targetPathId: step.toPathId,
            targetIndex: step.toIndex,
          },
        };
        await addScenarioCommand(createResp.scenario.id, commandPayload);
      }

      setSelectedScenarioId(String(createResp.scenario.id));
      await refreshSavedScenarios();
      setMovementHint("Сценарий сохранен.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось сохранить сценарий.");
    }
  }

  async function handleLoadScenario() {
    if (!selectedScenarioId) {
      setMovementHint("Выбери сценарий для загрузки.");
      return;
    }
    try {
      const scenario = await getScenario(selectedScenarioId);
      setScenarioName(scenario.name || "Сценарий");
      const loadedSteps = buildScenarioStepsFromBackendScenario(scenario);
      setScenarioSteps(loadedSteps);

      if (scenario.initialState) {
        setSegments(scenario.initialState.segments || []);
        setVehicles(scenario.initialState.vehicles || []);
        setCouplings(scenario.initialState.couplings || []);
      }

      setMovementHint("Сценарий загружен.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось загрузить сценарий.");
    }
  }

  useEffect(() => {
    if (selectedLocomotiveId) {
      const selected = vehicleById.get(selectedLocomotiveId);
      const label = selected ? vehicleCodeById.get(selected.id) : "";
      if (label) {
        setScenarioUnitCode(label);
        if (selected.pathId) {
          setScenarioFromPathId(selected.pathId);
          setScenarioFromIndex(String(selected.pathIndex ?? ""));
        } else {
          setScenarioFromPathId("");
          setScenarioFromIndex("");
        }
      }
    }
  }, [selectedLocomotiveId, vehicleById, vehicleCodeById]);

  useEffect(() => {
    if (targetPathId) {
      setScenarioToPathId(targetPathId);
      setScenarioToIndex(targetPathIndex == null ? "" : String(targetPathIndex));
    }
  }, [targetPathId, targetPathIndex]);

  async function buildMovementTimeline(locoId, targetPathIdValue, targetIndexValue, sourceVehicles) {
    const response = await planMovement({
      gridSize: GRID_SIZE,
      segments,
      vehicles: sourceVehicles,
      couplings,
      selectedLocomotiveId: locoId,
      targetPathId: targetPathIdValue,
      targetIndex: targetIndexValue,
    });

    if (!response.ok) {
      throw new Error(response.message || "Не удалось рассчитать движение.");
    }

    const timeline = response.timeline || [];
    if (!timeline.length) {
      throw new Error("Маршрут не найден.");
    }

    return timeline;
  }

  async function playTimeline(timeline, startMessage = "Движение запущено.") {
    if (!timeline.length) {
      return false;
    }

    setMovementHint(startMessage);
    setMovementCellsPassed(0);
    setIsMoving(true);

    let stepIndex = 0;

    // будем хранить актуальное состояние
    let currentVehicles = vehicles.map(v => ({ ...v }));

    movementTimerRef.current = setInterval(async () => {
      const step = timeline[stepIndex];

      if (!step) {
        clearInterval(movementTimerRef.current);
        movementTimerRef.current = null;
        setIsMoving(false);

        try {
          const resolved = await resolveVehicles({
            gridSize: GRID_SIZE,
            segments,
            vehicles: currentVehicles,
            couplings,
            movedVehicleIds: currentVehicles.map(v => v.id),
            strictCouplings: true,
          });

          if (resolved.ok && Array.isArray(resolved.vehicles)) {
            setVehicles(resolved.vehicles);
          }

        } catch (error) {
          console.error("resolveVehicles failed", error);
        }

        setMovementHint("Движение завершено.");
        return;
      }

      // обновляем локальное состояние
      currentVehicles = applyTimelineStepToVehicles(currentVehicles, step);

      // обновляем React state
      setVehicles(currentVehicles);

      setMovementCellsPassed(prev => prev + 1);
      stepIndex += 1;

    }, 180);

    return true;
  }

  async function executeMovement(locoId) {
    const timeline = await buildMovementTimeline(locoId, targetPathId, targetPathIndex, vehicles);
    return playTimeline(timeline, "Движение запущено.");
  }

function addScenarioStep() {
    const unitCode = normalizeUnitCode(scenarioUnitCode);
    const fromPathId = scenarioFromPathId.trim();
    const fromIndex = Number.parseInt(scenarioFromIndex, 10);
    const toPathId = scenarioToPathId.trim();
    const toIndex = Number.parseInt(scenarioToIndex, 10);

    if (!unitCode || !fromPathId || Number.isNaN(fromIndex) || !toPathId || Number.isNaN(toIndex)) {
      setMovementHint("Заполни номер объекта, путь и индекс откуда/куда.");
      return;
    }

    setScenarioSteps((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        unitCode,
        fromPathId,
        fromIndex,
        toPathId,
        toIndex,
      },
    ]);
    setMovementHint("Шаг добавлен в сценарий.");
  }

  function removeScenarioStep(stepId) {
    setScenarioSteps((prev) => prev.filter((step) => step.id !== stepId));
  }

  function clearScenarioSteps() {
    setScenarioSteps([]);
  }

  async function runSimpleScenario() {
    if (isMoving) {
      return;
    }

    const steps = scenarioSteps.length
      ? scenarioSteps
      : [
          {
            id: "single",
            unitCode: normalizeUnitCode(scenarioUnitCode),
            fromPathId: scenarioFromPathId.trim(),
            fromIndex: Number.parseInt(scenarioFromIndex, 10),
            toPathId: scenarioToPathId.trim(),
            toIndex: Number.parseInt(scenarioToIndex, 10),
          },
        ];

    try {
      if (
        !steps.length ||
        !steps[0].unitCode ||
        !steps[0].fromPathId ||
        Number.isNaN(steps[0].fromIndex) ||
        !steps[0].toPathId ||
        Number.isNaN(steps[0].toIndex)
      ) {
        setMovementHint("Добавь хотя бы один корректный шаг сценария.");
        return;
      }

      let workingVehicles = vehicles.map((vehicle) => ({ ...vehicle }));
      const combinedTimeline = [];
      let lastLocomotiveId = null;
      let lastTargetPathId = null;
      let lastTargetIndex = null;

      for (const step of steps) {
        const stepCode = normalizeUnitCode(step.unitCode);
        if (!stepCode) {
          setMovementHint(`Некорректный номер объекта в шаге: ${step.unitCode}.`);
          return;
        }
        const codeMap = buildVehicleCodeMap(workingVehicles);
        const target = workingVehicles.find(
          (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === stepCode
        );
        if (!target) {
          setMovementHint(`Объект с номером ${stepCode} не найден.`);
          return;
        }
        if (target.type !== "locomotive") {
          setMovementHint(`Объект ${stepCode} не локомотив.`);
          return;
        }

        if (target.pathId !== step.fromPathId || Number(target.pathIndex) !== step.fromIndex) {
          setMovementHint(
            `Локомотив ${codeMap.get(target.id) || target.id} сейчас в ${target.pathId}:${target.pathIndex}, а не в ${step.fromPathId}:${step.fromIndex}.`
          );
          return;
        }

        const timeline = await buildMovementTimeline(
          target.id,
          step.toPathId,
          step.toIndex,
          workingVehicles
        );
        for (const frame of timeline) {
          combinedTimeline.push(frame);
          workingVehicles = applyTimelineStepToVehicles(workingVehicles, frame);
        }
        try {
          const resolved = await resolveVehicles({
            gridSize: GRID_SIZE,
            segments,
            vehicles: workingVehicles,
            couplings,
            movedVehicleIds: workingVehicles.map((v) => v.id),
            strictCouplings: true,
          });
          if (resolved.ok && Array.isArray(resolved.vehicles)) {
            workingVehicles = resolved.vehicles;
          }
        } catch (error) {
          setMovementHint("Не удалось обновить позиции после шага.");
          return;
        }
        lastLocomotiveId = target.id;
        lastTargetPathId = step.toPathId;
        lastTargetIndex = step.toIndex;
      }

      if (!combinedTimeline.length) {
        setMovementHint("Сценарий не содержит шагов движения.");
        return;
      }

      if (lastLocomotiveId) {
        setSelectedLocomotiveId(lastLocomotiveId);
      }
      if (lastTargetPathId) {
        setTargetPathId(lastTargetPathId);
        setTargetPathIndex(lastTargetIndex);
      }

      playTimeline(combinedTimeline, "Сценарий движения запущен.");
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
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
    if (event.button !== 0 || !isEditMode || isPanning || isMoving) {
      return;
    }
    const point = getWorldPoint(event, false);
    setSelectionBox({ start: point, end: point });
    setDragState(null);
  }

  async function handleCanvasClick(event) {
    if (!isManeuversPanel || mode !== "drawTrack" || isPanning || isMoving) {
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

    try {
      await applyLayoutAction("add_segment", {
        from: startPoint,
        to: point,
      });
      setStartPoint(null);
    } catch (error) {
      setMovementHint(error.message);
    }
  }

  function handleMouseMove(event) {
    const point = getWorldPoint(event, true);
    const rawPoint = getWorldPoint(event, false);
    setMousePoint(point);

    if (selectionBox && isEditMode) {
      setSelectionBox((prev) => (prev ? { ...prev, end: rawPoint } : prev));
      return;
    }

    if (!dragState || !isEditMode || isMoving) {
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

  async function handleMouseUp() {
    if (dragState?.type === "vehicle") {
      const originMap = new Map(dragState.origins.map((item) => [item.id, item]));
      try {
        await applyLayoutAction("resolve_vehicles", {
          movedVehicleIds: dragState.origins.map((item) => item.id),
          strictCouplings: true,
        });
      } catch (error) {
        setMovementHint(error.message || "Backend connection error.");
        setVehicles((prev) =>
          prev.map((vehicle) => {
            const origin = originMap.get(vehicle.id);
            if (!origin) {
              return vehicle;
            }
            return { ...vehicle, x: origin.x, y: origin.y };
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
    if (event.button !== 1 || isMoving) {
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
    if (event.button !== 0 || !isEditMode || isPanning || isMoving) {
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
    if (event.button !== 0 || !isEditMode || isPanning || isMoving) {
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

  async function handleSlotClick(event, slot) {
    if (isMoving) {
      return;
    }

    if (isMoveMode) {
      event.stopPropagation();
      setTargetPathId(slot.pathId);
      setTargetPathIndex(slot.index);
      setMovementHint(`Цель: ${slot.pathId}:${slot.index}`);
      return;
    }

    if (!isPlaceMode) {
      return;
    }
    event.stopPropagation();
    const type = mode === "placeLocomotive" ? "locomotive" : "wagon";

    try {
      await applyLayoutAction("place_vehicle", {
        vehicleType: type,
        targetPathId: slot.pathId,
        targetIndex: slot.index,
      });
    } catch (error) {
      setMovementHint(error.message || "Backend connection error.");
    }
  }

  function handleVehicleClick(event, vehicleId) {
    event.stopPropagation();

    if (isMoveMode) {
      const vehicle = vehicleById.get(vehicleId);
      if (!vehicle || vehicle.type !== "locomotive") {
        return;
      }
      setSelectedLocomotiveId(vehicleId);
      setSelectedVehicleIds([vehicleId]);
      setMovementHint("Локомотив выбран. Укажи целевую точку.");
      return;
    }

    if (!(isManeuversPanel || isCouplingPanel)) {
      return;
    }

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

  async function startLocomotiveMovement() {
    if (isMoving) {
      return;
    }
    try {
      await executeMovement(selectedLocomotiveId);
    } catch (error) {
      setMovementHint("Ошибка связи с backend.");
    }
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

  async function coupleSelectedVehicles() {
    if (selectedVehicleIds.length < 2) {
      setMovementHint("Выбери два состава для сцепки.");
      return;
    }

    try {
      await applyLayoutAction("couple", {
        selectedVehicleIds,
      });
      setMovementHint("Сцепка выполнена.");
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
  }

  async function decoupleSelectedVehicles() {
    if (selectedVehicleIds.length < 2) {
      return;
    }
    try {
      await applyLayoutAction("decouple", { selectedVehicleIds });
      setMovementHint("Расцепка выполнена.");
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
  }

  async function clearLayout() {
    stopMovement(true);
    try {
      await applyLayoutAction("clear");
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
    setStartPoint(null);
    setSelectedSegmentIds([]);
    setSelectedVehicleIds([]);
    setDragState(null);
    setSelectionBox(null);
    setMovementCellsPassed(0);
  }

  async function deleteSelectedSegments() {
    if (!selectedSegmentIds.length) {
      return;
    }
    try {
      await applyLayoutAction("delete_segments", { ids: selectedSegmentIds });
      setSelectedSegmentIds([]);
      setDragState(null);
      setSelectionBox(null);
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
  }

  async function deleteSelectedVehicles() {
    if (!selectedVehicleIds.length) {
      return;
    }
    try {
      await applyLayoutAction("delete_vehicles", { ids: selectedVehicleIds });
      setSelectedVehicleIds([]);
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
  }

  async function deleteSelectedAll() {
    await deleteSelectedVehicles();
    await deleteSelectedSegments();
  }

  function switchMode(nextMode) {
    if (mode !== nextMode) {
      stopMovement(false);
    }
    setMode(nextMode);
    setStartPoint(null);
    setDragState(null);
    setSelectionBox(null);
    if (nextMode !== "edit") {
      setSelectedSegmentIds([]);
      setSelectedVehicleIds([]);
    }
  }

  function switchPanel(nextPanel) {
    if (nextPanel === activePanel) {
      return;
    }
    setActivePanel(nextPanel);

    if (nextPanel === "maneuvers") {
      if (!["drawTrack", "placeWagon", "placeLocomotive", "edit"].includes(mode)) {
        switchMode("drawTrack");
      }
      return;
    }

    if (nextPanel === "movement") {
      switchMode("move");
      return;
    }

    stopMovement(false);
    setMode("view");
    setStartPoint(null);
    setDragState(null);
    setSelectionBox(null);
    setSelectedSegmentIds([]);
    setSelectedVehicleIds([]);
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
          : mode === "move"
            ? "Движение"
            : mode === "edit"
              ? "Редактирование"
              : "Просмотр";

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="panelContent">
          {activePanel === "maneuvers" && (
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
              <input
                className="toolInput"
                value={layoutName}
                onChange={(event) => setLayoutName(event.target.value)}
                placeholder="Название схемы"
              />
              <select
                className="toolInput"
                value={selectedLayoutId}
                onChange={(event) => setSelectedLayoutId(event.target.value)}
              >
                <option value="">Выбери схему</option>
                {savedLayouts.map((layout) => (
                  <option key={layout.id} value={String(layout.id)}>
                    {layout.name || `Схема ${layout.id}`}
                  </option>
                ))}
              </select>
              <button type="button" className="toolButton" onClick={handleSaveLayout}>
                Сохранить схему
              </button>
              <button type="button" className="toolButton" onClick={handleLoadLayout}>
                Загрузить схему
              </button>
              <button type="button" className="toolButton" onClick={handleDeleteLayout}>
                Удалить схему
              </button>
            </div>
          )}

          {activePanel === "coupling" && (
            <div className="tools">
              <button type="button" className="toolButton" onClick={coupleSelectedVehicles}>
                Сцепить выбранные
              </button>
              <button type="button" className="toolButton" onClick={decoupleSelectedVehicles}>
                Расцепить выбранные
              </button>
            </div>
          )}

          {activePanel === "movement" && (
            <div className="tools">
              <button
                type="button"
                className={`toolButton ${mode === "move" ? "active" : ""}`}
                onClick={() => switchMode("move")}
              >
                Режим движения
              </button>
              <button type="button" className="toolButton" onClick={startLocomotiveMovement}>
                Старт движения
              </button>
              <button type="button" className="toolButton" onClick={() => stopMovement(false)}>
                Стоп
              </button>
            </div>
          )}

          {activePanel === "scenario" && (
            <div className="tools">
              <input
                className="toolInput"
                value={scenarioName}
                onChange={(event) => setScenarioName(event.target.value)}
                placeholder="Название сценария"
              />
              <select
                className="toolInput"
                value={selectedScenarioId}
                onChange={(event) => setSelectedScenarioId(event.target.value)}
              >
                <option value="">Выбери сценарий</option>
                {savedScenarios.map((scenario) => (
                  <option key={scenario.id} value={String(scenario.id)}>
                    {scenario.name || `Сценарий ${scenario.id}`}
                  </option>
                ))}
              </select>
              <button type="button" className="toolButton" onClick={handleSaveScenario}>
                Сохранить сценарий
              </button>
              <button type="button" className="toolButton" onClick={handleLoadScenario}>
                Загрузить сценарий
              </button>
              <input
                className="toolInput"
                value={scenarioUnitCode}
                onChange={(event) => setScenarioUnitCode(event.target.value)}
                placeholder="Номер объекта (л1, в1)"
              />
              <input
                className="toolInput"
                value={scenarioFromPathId}
                onChange={(event) => setScenarioFromPathId(event.target.value)}
                placeholder="Откуда путь (id)"
              />
              <input
                className="toolInput"
                value={scenarioFromIndex}
                onChange={(event) => setScenarioFromIndex(event.target.value)}
                placeholder="Откуда индекс"
              />
              <input
                className="toolInput"
                value={scenarioToPathId}
                onChange={(event) => setScenarioToPathId(event.target.value)}
                placeholder="Куда путь (id)"
              />
              <input
                className="toolInput"
                value={scenarioToIndex}
                onChange={(event) => setScenarioToIndex(event.target.value)}
                placeholder="Куда индекс"
              />
              <button type="button" className="toolButton" onClick={addScenarioStep}>
                Добавить шаг
              </button>
              <button type="button" className="toolButton" onClick={clearScenarioSteps}>
                Очистить шаги
              </button>
              {scenarioSteps.length > 0 && (
                <div className="scenarioSteps">
                  {scenarioSteps.map((step, index) => (
                    <div key={step.id} className="scenarioStepRow">
                      <span className="scenarioStepText">
                        {index + 1}. {step.unitCode}: {step.fromPathId}:{step.fromIndex} {"->"}{" "}
                        {step.toPathId}:{step.toIndex}
                      </span>
                      <button
                        type="button"
                        className="scenarioStepRemove"
                        onClick={() => removeScenarioStep(step.id)}
                      >
                        x
                      </button>
                    </div>
                  ))}
                </div>
              )}
              <button type="button" className="toolButton" onClick={runSimpleScenario}>
                Выполнить шаги
              </button>
            </div>
          )}

          {activePanel === "metrics" && (
            <div className="tools">
              <p className="counter">Путей: {segments.length}</p>
              <p className="counter">Составов: {vehicles.length}</p>
              <p className="counter">Сцепок: {couplings.length}</p>
              <p className="counter">Выбрано составов: {selectedVehicleIds.length}</p>
              <p className="counter">Локомотив: {selectedLocomotiveId ? "выбран" : "не выбран"}</p>
              <p className="counter">Цель: {targetPathId ? "выбрана" : "не выбрана"}</p>
              <p className="counter">Пройдено ячеек: {movementCellsPassed}</p>
            </div>
          )}
        </div>

        <p className="counter">{movementHint || "-"}</p>
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
              >
                <title>Путь: {segment.id}</title>
              </line>
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
                fill={
                  targetPathId === slot.pathId && targetPathIndex === slot.index
                    ? "#22c55e"
                    : occupiedSlots.has(slot.id)
                      ? "#94a3b8"
                      : "#cbd5e1"
                }
                className="slotPoint"
                onClick={(event) => handleSlotClick(event, slot)}
              >
                <title>
                  Путь: {slot.pathId}, звено: {slot.index}
                </title>
             </circle>
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
              >
                <title>{vehicleCodeById.get(vehicle.id) || vehicle.id}</title>
              </rect>
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

