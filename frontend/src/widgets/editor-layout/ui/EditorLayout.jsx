import { useEffect, useMemo, useRef, useState } from "react";
import { GRID_SIZE, getSegmentSlots, keyOf, snap } from "../../../shared/lib/geometry.js";
import {
  addScenarioCommand,
  applyLayoutOperation,
  createScenario,
  deleteScenario,
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
const SCENARIO_STEP_MOVE = "MOVE_LOCO";
const SCENARIO_STEP_COUPLE = "COUPLE";
const SCENARIO_STEP_DECOUPLE = "DECOUPLE";

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

function normalizeScenarioStepType(type) {
  const normalized = String(type || "").trim().toUpperCase();
  if (normalized === SCENARIO_STEP_COUPLE || normalized === SCENARIO_STEP_DECOUPLE) {
    return normalized;
  }
  return SCENARIO_STEP_MOVE;
}

function normalizeScenarioStep(step) {
  const type = normalizeScenarioStepType(step?.type);
  const payload = step?.payload && typeof step.payload === "object" ? step.payload : {};
  const id = step?.id || crypto.randomUUID();

  if (type === SCENARIO_STEP_MOVE) {
    return {
      id,
      type,
      payload: {
        unitCode: normalizeUnitCode(payload.unitCode ?? step?.unitCode ?? ""),
        fromPathId: String(payload.fromPathId ?? step?.fromPathId ?? "").trim(),
        fromIndex: Number(payload.fromIndex ?? step?.fromIndex ?? Number.NaN),
        toPathId: String(payload.toPathId ?? step?.toPathId ?? "").trim(),
        toIndex: Number(payload.toIndex ?? step?.toIndex ?? Number.NaN),
      },
    };
  }

  const rawCodes = Array.isArray(payload.unitCodes) ? payload.unitCodes : [];
  const unitCodes = rawCodes.map((value) => normalizeUnitCode(value)).filter(Boolean).slice(0, 2);
  return {
    id,
    type,
    payload: {
      unitCodes,
    },
  };
}

function formatScenarioStepText(step, index) {
  if (step.type === SCENARIO_STEP_MOVE) {
    return `${index + 1}. ${step.payload.unitCode}: ${step.payload.fromPathId}:${step.payload.fromIndex} -> ${step.payload.toPathId}:${step.payload.toIndex}`;
  }
  if (step.type === SCENARIO_STEP_COUPLE) {
    return `${index + 1}. СЦЕПКА: ${(step.payload.unitCodes || []).join(" + ")}`;
  }
  return `${index + 1}. РАСЦЕПКА: ${(step.payload.unitCodes || []).join(" + ")}`;
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

function buildScenarioStepsFromBackendScenario(scenario, sourceVehicles = []) {
  if (!scenario || !Array.isArray(scenario.commands)) {
    return [];
  }

  const steps = [];
  const codeByVehicleId = buildVehicleCodeMap(sourceVehicles);

  for (const command of scenario.commands) {
    const type = normalizeScenarioStepType(command.type);
    const payload = command.payload || {};

    if (type === SCENARIO_STEP_MOVE) {
      const locoId = payload.locoId;
      const fromPathId = String(payload.fromPathId || "").trim();
      const fromIndex =
        payload.fromIndex == null || payload.fromIndex === ""
          ? Number.NaN
          : Number(payload.fromIndex);
      const toPathId = String(payload.toPathId || payload.targetPathId || "").trim();
      const toIndex = Number(payload.toIndex ?? payload.targetIndex ?? 0);

      steps.push({
        id: command.id || crypto.randomUUID(),
        type,
        payload: {
          unitCode: normalizeUnitCode(codeByVehicleId.get(locoId)) || "",
          fromPathId,
          fromIndex,
          toPathId,
          toIndex,
        },
      });
      continue;
    }

    steps.push({
      id: command.id || crypto.randomUUID(),
      type,
      payload: {
        unitCodes: [
          normalizeUnitCode(codeByVehicleId.get(payload.aId)) || "",
          normalizeUnitCode(codeByVehicleId.get(payload.bId)) || "",
        ].filter(Boolean),
      },
    });
  }

  return steps;
}

export default function EditorLayout({ activePanel, setActivePanel }) {
  const canvasRef = useRef(null);
  const canvasWrapRef = useRef(null);
  const panStateRef = useRef(null);
  const movementTimerRef = useRef(null);
  const movementRunIdRef = useRef(0);
  const skipAutoResolvePassesRef = useRef(0);
  const scenarioStopRequestedRef = useRef(false);

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
  const [scenarioStepType, setScenarioStepType] = useState(SCENARIO_STEP_MOVE);
  const [scenarioSteps, setScenarioSteps] = useState([]);
  const [scenarioInitialState, setScenarioInitialState] = useState(null);
  const [scenarioLayoutId, setScenarioLayoutId] = useState("");
  const [currentScenarioStep, setCurrentScenarioStep] = useState(0);
  const [scenarioStateHistory, setScenarioStateHistory] = useState([]);
  const [scenarioViewMode, setScenarioViewMode] = useState("start");
  const [scenarioExecutingStep, setScenarioExecutingStep] = useState(null);
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
    if (skipAutoResolvePassesRef.current > 0) {
      skipAutoResolvePassesRef.current -= 1;
      return;
    }

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
    movementRunIdRef.current += 1;
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
      setScenarioInitialState(null);
      setScenarioViewMode("start");
      setScenarioExecutingStep(null);
      scenarioStopRequestedRef.current = false;
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
      if (!selectedLayoutId) {
        setMovementHint("Перед сохранением сценария выбери сохраненную схему.");
        return;
      }
      const layoutID = Number.parseInt(selectedLayoutId, 10);
      if (Number.isNaN(layoutID) || layoutID <= 0) {
        setMovementHint("Некорректный layout_id для сценария.");
        return;
      }

      const createResp = await createScenario({
        name: scenarioName.trim() || "Сценарий",
        layoutId: layoutID,
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

      for (const rawStep of scenarioSteps) {
        const step = normalizeScenarioStep(rawStep);

        if (step.type === SCENARIO_STEP_MOVE) {
          const code = normalizeUnitCode(step.payload.unitCode);
          const locoId = idByCode.get(code);
          if (!locoId) {
            throw new Error(`Локомотив ${code || step.payload.unitCode} не найден для сохранения.`);
          }

          await addScenarioCommand(createResp.scenario.id, {
            type: SCENARIO_STEP_MOVE,
            payload: {
              locoId,
              fromPathId: step.payload.fromPathId,
              fromIndex: step.payload.fromIndex,
              toPathId: step.payload.toPathId,
              toIndex: step.payload.toIndex,
              targetPathId: step.payload.toPathId,
              targetIndex: step.payload.toIndex,
            },
          });
          continue;
        }

        const [aCode, bCode] = step.payload.unitCodes || [];
        const aId = idByCode.get(aCode);
        const bId = idByCode.get(bCode);
        if (!aId || !bId) {
          throw new Error(`Объекты ${aCode || "?"} и ${bCode || "?"} не найдены для сохранения.`);
        }

        await addScenarioCommand(createResp.scenario.id, {
          type: step.type,
          payload: {
            aId,
            bId,
          },
        });
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
      console.log("SCENARIO INITIAL STATE", scenario.initialState);
      console.log(
        "INITIAL VEHICLE POSITIONS",
        scenario.initialState?.vehicles?.map((v) => ({
          id: v.id,
          code: v.code,
          pathId: v.pathId,
          pathIndex: v.pathIndex,
          x: v.x,
          y: v.y,
        }))
      );
      stopMovement(true);
      scenarioStopRequestedRef.current = false;
      setScenarioName(scenario.name || "Сценарий");
      setScenarioLayoutId(
        scenario.layoutId == null || scenario.layoutId === ""
          ? ""
          : String(scenario.layoutId)
      );
      const loadedSteps = buildScenarioStepsFromBackendScenario(scenario, vehicles);
      setScenarioSteps(loadedSteps);
      setCurrentScenarioStep(0);
      setScenarioStateHistory([]);
      setScenarioInitialState(null);
      setScenarioViewMode("start");
      setScenarioExecutingStep(null);

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

  async function buildMovementTimeline(
    locoId,
    targetPathIdValue,
    targetIndexValue,
    sourceVehicles,
    sourceCouplings = couplings
  ) {
    const response = await planMovement({
      gridSize: GRID_SIZE,
      segments,
      vehicles: sourceVehicles,
      couplings: sourceCouplings,
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

  async function playTimeline(
    timeline,
    startMessage = "Движение запущено.",
    sourceVehicles = vehicles,
    sourceCouplings = couplings
  ) {
    if (!timeline.length) {
      return { ok: false, vehicles: sourceVehicles };
    }
    const runId = movementRunIdRef.current + 1;
    movementRunIdRef.current = runId;

    setMovementHint(startMessage);
    setMovementCellsPassed(0);
    setIsMoving(true);

    return new Promise((resolve) => {
      let stepIndex = 0;
      let currentVehicles = sourceVehicles.map((vehicle) => ({ ...vehicle }));

      movementTimerRef.current = setInterval(async () => {
        if (runId !== movementRunIdRef.current) {
          clearInterval(movementTimerRef.current);
          movementTimerRef.current = null;
          setIsMoving(false);
          resolve({ ok: false, vehicles: sourceVehicles });
          return;
        }

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
              couplings: sourceCouplings,
              movedVehicleIds: currentVehicles.map((v) => v.id),
              strictCouplings: true,
            });

            if (resolved.ok && Array.isArray(resolved.vehicles)) {
              if (runId !== movementRunIdRef.current) {
                resolve({ ok: false, vehicles: sourceVehicles });
                return;
              }
              currentVehicles = resolved.vehicles;
              setVehicles(resolved.vehicles);
            }
          } catch (error) {
            console.error("resolveVehicles failed", error);
          }

          setMovementHint("Движение завершено.");
          resolve({ ok: true, vehicles: currentVehicles });
          return;
        }

        if (runId !== movementRunIdRef.current) {
          resolve({ ok: false, vehicles: sourceVehicles });
          return;
        }
        currentVehicles = applyTimelineStepToVehicles(currentVehicles, step);
        setVehicles(currentVehicles);
        setMovementCellsPassed((prev) => prev + 1);
        stepIndex += 1;
      }, 180);
    });
  }

  async function handleDeleteScenario() {
    if (!selectedScenarioId) {
      setMovementHint("Выбери сценарий для удаления.");
      return;
    }
    try {
      await deleteScenario(selectedScenarioId);
      setSelectedScenarioId("");
      setScenarioSteps([]);
      setCurrentScenarioStep(0);
      setScenarioStateHistory([]);
      setScenarioInitialState(null);
      setScenarioLayoutId("");
      setScenarioViewMode("start");
      setScenarioExecutingStep(null);
      scenarioStopRequestedRef.current = false;
      await refreshSavedScenarios();
      setMovementHint("Сценарий удален.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось удалить сценарий.");
    }
  }

  async function executeMovement(locoId) {
    const timeline = await buildMovementTimeline(locoId, targetPathId, targetPathIndex, vehicles);
    return playTimeline(timeline, "Движение запущено.");
  }

  function buildDraftScenarioStep() {
    const type = normalizeScenarioStepType(scenarioStepType);
    if (type === SCENARIO_STEP_MOVE) {
      const unitCode = normalizeUnitCode(scenarioUnitCode);
      const fromPathId = scenarioFromPathId.trim();
      const fromIndex = Number.parseInt(scenarioFromIndex, 10);
      const toPathId = scenarioToPathId.trim();
      const toIndex = Number.parseInt(scenarioToIndex, 10);

      if (!unitCode || !fromPathId || Number.isNaN(fromIndex) || !toPathId || Number.isNaN(toIndex)) {
        setMovementHint("Заполни номер объекта, путь и индекс откуда/куда.");
        return null;
      }

      return normalizeScenarioStep({
        id: crypto.randomUUID(),
        type: SCENARIO_STEP_MOVE,
        payload: {
          unitCode,
          fromPathId,
          fromIndex,
          toPathId,
          toIndex,
        },
      });
    }

    const selectedCodes = selectedVehicleIds
      .map((id) => normalizeUnitCode(vehicleCodeById.get(id)))
      .filter(Boolean)
      .slice(0, 2);
    if (selectedCodes.length < 2) {
      setMovementHint("Для сцепки/расцепки выбери два объекта на схеме.");
      return null;
    }
    return normalizeScenarioStep({
      id: crypto.randomUUID(),
      type,
      payload: {
        unitCodes: selectedCodes,
      },
    });
  }

  function addScenarioStep() {
    const step = buildDraftScenarioStep();
    if (!step) {
      return;
    }
    setScenarioSteps((prev) => [...prev, step]);
    setScenarioInitialState(null);
    setCurrentScenarioStep(0);
    setScenarioStateHistory([]);
    setScenarioViewMode("start");
    setScenarioExecutingStep(null);
    setMovementHint("Шаг добавлен в сценарий.");
  }

  function removeScenarioStep(stepId) {
    setScenarioSteps((prev) => prev.filter((step) => step.id !== stepId));
    setScenarioInitialState(null);
    setCurrentScenarioStep(0);
    setScenarioStateHistory([]);
    setScenarioViewMode("start");
    setScenarioExecutingStep(null);
  }

  function clearScenarioSteps() {
    setScenarioSteps([]);
    setScenarioInitialState(null);
    setCurrentScenarioStep(0);
    setScenarioStateHistory([]);
    setScenarioViewMode("start");
    setScenarioExecutingStep(null);
  }

  function cloneLayoutState(state) {
    return JSON.parse(JSON.stringify(state));
  }

  function applyLayoutSnapshot(snapshot, skipResolvePasses = 2) {
    const safeSnapshot = cloneLayoutState(snapshot || { segments: [], vehicles: [], couplings: [] });
    skipAutoResolvePassesRef.current = skipResolvePasses;
    setSegments(safeSnapshot.segments || []);
    setVehicles(safeSnapshot.vehicles || []);
    setCouplings(safeSnapshot.couplings || []);
    return safeSnapshot;
  }

  function ensureScenarioInitialState() {
    if (scenarioInitialState) {
      return scenarioInitialState;
    }
    const snapshot = cloneLayoutState({
      segments,
      vehicles,
      couplings,
    });
    setScenarioInitialState(snapshot);
    return snapshot;
  }

  async function loadScenarioStartLayoutState() {
    const layoutId = Number.parseInt(String(scenarioLayoutId || selectedLayoutId || ""), 10);
    if (Number.isNaN(layoutId) || layoutId <= 0) {
      throw new Error("Для сценария не выбрана стартовая схема.");
    }

    const response = await getLayout(layoutId);
    const state = response.layout?.state || {};
    const snapshot = cloneLayoutState({
      segments: state.segments || [],
      vehicles: state.vehicles || [],
      couplings: state.couplings || [],
    });
    setSelectedLayoutId(String(layoutId));
    return snapshot;
  }

  function cloneTimeline(timeline) {
    if (!Array.isArray(timeline)) {
      return [];
    }
    return timeline.map((frame) =>
      Array.isArray(frame) ? frame.map((item) => ({ ...item })) : []
    );
  }

  async function executeSingleScenarioStep(
    step,
    stepIndex,
    totalSteps,
    sourceVehicles,
    sourceCouplings,
    animateMovement = true
  ) {
    const codeMap = buildVehicleCodeMap(sourceVehicles);

    if (step.type === SCENARIO_STEP_MOVE) {
      const stepCode = normalizeUnitCode(step.payload.unitCode);
      if (!stepCode) {
        throw new Error(`Некорректный номер объекта в шаге: ${step.payload.unitCode}.`);
      }
      const target = sourceVehicles.find(
        (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === stepCode
      );
      if (!target) {
        throw new Error(`Объект с номером ${stepCode} не найден.`);
      }
      if (target.type !== "locomotive") {
        throw new Error(`Объект ${stepCode} не локомотив.`);
      }
      if (
        target.pathId !== step.payload.fromPathId ||
        Number(target.pathIndex) !== step.payload.fromIndex
      ) {
        throw new Error(
          `Локомотив ${codeMap.get(target.id) || target.id} сейчас в ${target.pathId}:${target.pathIndex}, а не в ${step.payload.fromPathId}:${step.payload.fromIndex}.`
        );
      }

      const timeline = await buildMovementTimeline(
        target.id,
        step.payload.toPathId,
        step.payload.toIndex,
        sourceVehicles,
        sourceCouplings
      );

      if (!animateMovement) {
        const lastFrame = timeline[timeline.length - 1] || [];
        let instantVehicles = applyTimelineStepToVehicles(
          sourceVehicles.map((vehicle) => ({ ...vehicle })),
          lastFrame
        );

        try {
          const resolved = await resolveVehicles({
            gridSize: GRID_SIZE,
            segments,
            vehicles: instantVehicles,
            couplings: sourceCouplings,
            movedVehicleIds: instantVehicles.map((v) => v.id),
            strictCouplings: true,
          });
          if (resolved.ok && Array.isArray(resolved.vehicles)) {
            instantVehicles = resolved.vehicles;
          }
        } catch {
          // Keep instantVehicles as-is if resolve is temporarily unavailable.
        }

        return {
          vehicles: instantVehicles,
          couplings: sourceCouplings,
          timeline: [],
          lastLocomotiveId: target.id,
          lastTargetPathId: step.payload.toPathId,
          lastTargetIndex: step.payload.toIndex,
        };
      }

      const animationResult = await playTimeline(
        timeline,
        `Шаг ${stepIndex + 1}/${totalSteps}: движение`,
        sourceVehicles,
        sourceCouplings
      );
      if (!animationResult.ok) {
        throw new Error("Не удалось выполнить шаг движения.");
      }

      return {
        vehicles: animationResult.vehicles,
        couplings: sourceCouplings,
        timeline: cloneTimeline(timeline),
        lastLocomotiveId: target.id,
        lastTargetPathId: step.payload.toPathId,
        lastTargetIndex: step.payload.toIndex,
      };
    }

    const unitCodes = (step.payload.unitCodes || [])
      .map((value) => normalizeUnitCode(value))
      .filter(Boolean);
    if (unitCodes.length < 2) {
      throw new Error("Шаг сцепки/расцепки должен содержать два объекта.");
    }

    const a = sourceVehicles.find(
      (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === unitCodes[0]
    );
    const b = sourceVehicles.find(
      (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === unitCodes[1]
    );
    if (!a || !b) {
      throw new Error(`Не найдены объекты ${unitCodes[0]} и ${unitCodes[1]}.`);
    }

    const response = await applyLayoutOperation({
      gridSize: GRID_SIZE,
      state: {
        segments,
        vehicles: sourceVehicles,
        couplings: sourceCouplings,
      },
      action: step.type === SCENARIO_STEP_COUPLE ? "couple" : "decouple",
      selectedVehicleIds: [a.id, b.id],
    });
    if (!response.ok) {
      throw new Error(response.message || "Не удалось выполнить шаг сцепки/расцепки.");
    }

    const nextState = response.state || {};
    return {
      vehicles: nextState.vehicles || [],
      couplings: nextState.couplings || [],
      timeline: [],
      lastLocomotiveId: null,
      lastTargetPathId: null,
      lastTargetIndex: null,
    };
  }

  async function handleNextScenarioStep() {
    if (isMoving) {
      return;
    }

    const steps = scenarioSteps.map((step) => normalizeScenarioStep(step)).filter(Boolean);
    if (!steps.length) {
      setMovementHint("Сценарий не содержит шагов.");
      return;
    }
    if (currentScenarioStep >= steps.length) {
      setScenarioViewMode("final");
      setMovementHint("Все шаги сценария уже выполнены.");
      return;
    }
    ensureScenarioInitialState();
    setScenarioViewMode("step");
    setScenarioExecutingStep(currentScenarioStep);

    const cachedEntry = scenarioStateHistory[currentScenarioStep];
    if (cachedEntry?.afterState) {
      try {
        if (
          cachedEntry.stepType === SCENARIO_STEP_MOVE &&
          Array.isArray(cachedEntry.timeline) &&
          cachedEntry.timeline.length > 0
        ) {
          const playResult = await playTimeline(
            cloneTimeline(cachedEntry.timeline),
            `Шаг ${currentScenarioStep + 1}/${steps.length}: движение`,
            vehicles.map((vehicle) => ({ ...vehicle })),
            couplings.map((coupling) => ({ ...coupling }))
          );
          if (!playResult.ok) {
            throw new Error("Не удалось выполнить шаг движения.");
          }
        }

        applyLayoutSnapshot(cachedEntry.afterState);
        setCurrentScenarioStep((prev) => prev + 1);
        setScenarioExecutingStep(null);
        setScenarioViewMode("step");
        setMovementHint(`Выполнен шаг ${currentScenarioStep + 1}/${steps.length}.`);
        return;
      } catch (error) {
        setScenarioExecutingStep(null);
        setMovementHint(error.message || "Не удалось выполнить шаг сценария.");
        return;
      }
    }

    const step = steps[currentScenarioStep];
    const beforeState = cloneLayoutState({ segments, vehicles, couplings });

    try {
      const result = await executeSingleScenarioStep(
        step,
        currentScenarioStep,
        steps.length,
        beforeState.vehicles.map((vehicle) => ({ ...vehicle })),
        beforeState.couplings.map((coupling) => ({ ...coupling }))
      );

      const afterState = cloneLayoutState({
        segments: beforeState.segments,
        vehicles: result.vehicles || [],
        couplings: result.couplings || [],
      });

      setScenarioStateHistory((prev) => [
        ...prev.slice(0, currentScenarioStep),
        {
          beforeState,
          afterState,
          stepType: step.type,
          timeline: result.timeline || [],
        },
      ]);
      applyLayoutSnapshot(afterState);
      setCurrentScenarioStep((prev) => prev + 1);
      if (result.lastLocomotiveId) {
        setSelectedLocomotiveId(result.lastLocomotiveId);
      }
      if (result.lastTargetPathId) {
        setTargetPathId(result.lastTargetPathId);
        setTargetPathIndex(result.lastTargetIndex);
      }
      setScenarioExecutingStep(null);
      setScenarioViewMode("step");
      setMovementHint(`Выполнен шаг ${currentScenarioStep + 1}/${steps.length}.`);
    } catch (error) {
      setScenarioExecutingStep(null);
      setMovementHint(error.message || "Не удалось выполнить шаг сценария.");
    }
  }

  async function handlePrevScenarioStep() {
    if (isMoving) {
      return;
    }
    if (currentScenarioStep <= 0 || !scenarioStateHistory.length) {
      setMovementHint("Нет предыдущего шага для отката.");
      return;
    }

    const entryIndex = currentScenarioStep - 1;
    const historyEntry = scenarioStateHistory[entryIndex];
    const previousSnapshot = historyEntry?.beforeState || scenarioInitialState || {};
    const shouldAnimateReverseMove =
      historyEntry?.stepType === SCENARIO_STEP_MOVE &&
      Array.isArray(historyEntry?.timeline) &&
      historyEntry.timeline.length > 0;

    if (shouldAnimateReverseMove) {
      const reverseTimeline = cloneTimeline(historyEntry.timeline).reverse();
      const reverseResult = await playTimeline(
        reverseTimeline,
        `Откат шага ${currentScenarioStep}/${scenarioSteps.length}: движение`,
        vehicles.map((vehicle) => ({ ...vehicle })),
        couplings.map((coupling) => ({ ...coupling }))
      );
      if (!reverseResult.ok) {
        setMovementHint("Не удалось выполнить обратную анимацию шага.");
        return;
      }
    }

    applyLayoutSnapshot(previousSnapshot);
    setCurrentScenarioStep((prev) => Math.max(0, prev - 1));
    setScenarioExecutingStep(null);
    setScenarioViewMode(currentScenarioStep - 1 <= 0 ? "start" : "step");
    setMovementHint(
      shouldAnimateReverseMove
        ? "Шаг откачен с обратной анимацией."
        : "Выполнен откат на предыдущий шаг."
    );
  }

  async function handleShowScenarioStart() {
    if (isMoving) {
      stopMovement(false);
    }
    try {
      const layoutStartState = await loadScenarioStartLayoutState();
      scenarioStopRequestedRef.current = false;
      setScenarioInitialState(layoutStartState);
      setScenarioStateHistory([]);
      setCurrentScenarioStep(0);
      setScenarioExecutingStep(null);
      applyLayoutSnapshot(layoutStartState);
      setScenarioViewMode("start");
      setMovementHint("Показано стартовое состояние сценария.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось показать стартовое состояние.");
    }
  }

  function stopScenarioPlayback() {
    scenarioStopRequestedRef.current = true;
    stopMovement(false);
    setScenarioExecutingStep(null);
    setScenarioViewMode("paused");
    setMovementHint("Выполнение сценария остановлено.");
  }

  async function runSimpleScenario() {
    if (isMoving) {
      return;
    }

    const fallbackStep = scenarioSteps.length ? null : buildDraftScenarioStep();
    const steps = (scenarioSteps.length ? scenarioSteps : fallbackStep ? [fallbackStep] : [])
      .map((step) => normalizeScenarioStep(step))
      .filter(Boolean);

    try {
      if (!steps.length) {
        setMovementHint("Добавь хотя бы один корректный шаг сценария.");
        return;
      }
      const startState =
        scenarioInitialState ||
        cloneLayoutState({
          segments,
          vehicles,
          couplings,
        });
      setScenarioInitialState(startState);
      scenarioStopRequestedRef.current = false;
      setScenarioViewMode("play");
      setScenarioExecutingStep(0);

      let workingVehicles = vehicles.map((vehicle) => ({ ...vehicle }));
      let workingCouplings = couplings.map((coupling) => ({ ...coupling }));
      let lastLocomotiveId = null;
      let lastTargetPathId = null;
      let lastTargetIndex = null;
      const historyDraft = [...scenarioStateHistory.slice(0, currentScenarioStep)];
      if (currentScenarioStep === 0) {
        historyDraft.length = 0;
      }

      for (let index = 0; index < steps.length; index += 1) {
        if (scenarioStopRequestedRef.current) {
          setScenarioStateHistory(historyDraft.filter(Boolean));
          setScenarioExecutingStep(null);
          setScenarioViewMode("paused");
          setMovementHint("Выполнение сценария остановлено.");
          return;
        }

        setCurrentScenarioStep(index);
        setScenarioExecutingStep(index);
        const step = steps[index];
        const beforeState = cloneLayoutState({
          segments,
          vehicles: workingVehicles,
          couplings: workingCouplings,
        });
        const codeMap = buildVehicleCodeMap(workingVehicles);
        if (step.type === SCENARIO_STEP_MOVE) {
          const stepCode = normalizeUnitCode(step.payload.unitCode);
          if (!stepCode) {
            setScenarioExecutingStep(null);
            setScenarioViewMode("paused");
            setMovementHint(`Некорректный номер объекта в шаге: ${step.payload.unitCode}.`);
            return;
          }
          const target = workingVehicles.find(
            (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === stepCode
          );
          if (!target) {
            setScenarioExecutingStep(null);
            setScenarioViewMode("paused");
            setMovementHint(`Объект с номером ${stepCode} не найден.`);
            return;
          }
          if (target.type !== "locomotive") {
            setScenarioExecutingStep(null);
            setScenarioViewMode("paused");
            setMovementHint(`Объект ${stepCode} не локомотив.`);
            return;
          }

          if (
            target.pathId !== step.payload.fromPathId ||
            Number(target.pathIndex) !== step.payload.fromIndex
          ) {
            setScenarioExecutingStep(null);
            setScenarioViewMode("paused");
            setMovementHint(
              `Локомотив ${codeMap.get(target.id) || target.id} сейчас в ${target.pathId}:${target.pathIndex}, а не в ${step.payload.fromPathId}:${step.payload.fromIndex}.`
            );
            return;
          }

          const timeline = await buildMovementTimeline(
            target.id,
            step.payload.toPathId,
            step.payload.toIndex,
            workingVehicles,
            workingCouplings
          );

          const animationResult = await playTimeline(
            timeline,
            `Шаг ${index + 1}/${steps.length}: движение`,
            workingVehicles,
            workingCouplings
          );
          if (!animationResult.ok) {
            setScenarioStateHistory(historyDraft.filter(Boolean));
            setScenarioExecutingStep(null);
            setScenarioViewMode("paused");
            setMovementHint(
              scenarioStopRequestedRef.current
                ? "Выполнение сценария остановлено."
                : "Не удалось выполнить шаг движения."
            );
            return;
          }
          workingVehicles = animationResult.vehicles;
          historyDraft[index] = {
            beforeState,
            afterState: cloneLayoutState({
              segments,
              vehicles: workingVehicles,
              couplings: workingCouplings,
            }),
            stepType: step.type,
            timeline: cloneTimeline(timeline),
          };
          setScenarioStateHistory(historyDraft.slice(0, index + 1));

          lastLocomotiveId = target.id;
          lastTargetPathId = step.payload.toPathId;
          lastTargetIndex = step.payload.toIndex;
          continue;
        }

        const unitCodes = (step.payload.unitCodes || [])
          .map((value) => normalizeUnitCode(value))
          .filter(Boolean);
        if (unitCodes.length < 2) {
          setScenarioExecutingStep(null);
          setScenarioViewMode("paused");
          setMovementHint("Шаг сцепки/расцепки должен содержать два объекта.");
          return;
        }
        const a = workingVehicles.find(
          (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === unitCodes[0]
        );
        const b = workingVehicles.find(
          (vehicle) => normalizeUnitCode(codeMap.get(vehicle.id)) === unitCodes[1]
        );
        if (!a || !b) {
          setScenarioExecutingStep(null);
          setScenarioViewMode("paused");
          setMovementHint(`Не найдены объекты ${unitCodes[0]} и ${unitCodes[1]}.`);
          return;
        }

        const response = await applyLayoutOperation({
          gridSize: GRID_SIZE,
          state: {
            segments,
            vehicles: workingVehicles,
            couplings: workingCouplings,
          },
          action: step.type === SCENARIO_STEP_COUPLE ? "couple" : "decouple",
          selectedVehicleIds: [a.id, b.id],
        });
        if (!response.ok) {
          throw new Error(response.message || "Не удалось выполнить шаг сцепки/расцепки.");
        }
        const nextState = response.state || {};
        workingVehicles = nextState.vehicles || [];
        workingCouplings = nextState.couplings || [];
        setVehicles(workingVehicles);
        setCouplings(workingCouplings);
        historyDraft[index] = {
          beforeState,
          afterState: cloneLayoutState({
            segments,
            vehicles: workingVehicles,
            couplings: workingCouplings,
          }),
          stepType: step.type,
          timeline: [],
        };
        setScenarioStateHistory(historyDraft.slice(0, index + 1));
        setMovementHint(
          `Шаг ${index + 1}/${steps.length}: ${step.type === SCENARIO_STEP_COUPLE ? "сцепка" : "расцепка"} выполнен.`
        );
      }

      setScenarioStateHistory(historyDraft.slice(0, steps.length));
      if (lastLocomotiveId) {
        setSelectedLocomotiveId(lastLocomotiveId);
      }
      if (lastTargetPathId) {
        setTargetPathId(lastTargetPathId);
        setTargetPathIndex(lastTargetIndex);
      }
      setVehicles(workingVehicles);
      setCouplings(workingCouplings);
      setScenarioExecutingStep(null);
      setCurrentScenarioStep(steps.length);
      setScenarioViewMode("final");
      setMovementHint("Сценарий выполнен.");
    } catch (error) {
      setScenarioExecutingStep(null);
      setScenarioViewMode("paused");
      setMovementHint(error.message || "Ошибка связи с backend.");
    }
  }

  async function handleShowScenarioFinal() {
    if (isMoving) {
      return;
    }

    const steps = scenarioSteps.map((step) => normalizeScenarioStep(step)).filter(Boolean);
    if (!steps.length) {
      setMovementHint("Сценарий не содержит шагов.");
      return;
    }

    if (!scenarioInitialState) {
      try {
        const startState = await loadScenarioStartLayoutState();
        setScenarioInitialState(startState);
        applyLayoutSnapshot(startState);
        setCurrentScenarioStep(0);
      } catch (error) {
        setMovementHint(error.message || "Не удалось показать финал сценария.");
        return;
      }
    }

    if (scenarioStateHistory.length >= steps.length) {
      const finalEntry = scenarioStateHistory[steps.length - 1];
      if (finalEntry?.afterState) {
        applyLayoutSnapshot(finalEntry.afterState);
        setCurrentScenarioStep(steps.length);
        setScenarioExecutingStep(null);
        setScenarioViewMode("final");
        setMovementHint("Показано финальное состояние сценария.");
        return;
      }
    }

    try {
      scenarioStopRequestedRef.current = false;
      let workingSegments = (scenarioInitialState?.segments || segments).map((segment) => ({ ...segment }));
      let workingVehicles = (scenarioInitialState?.vehicles || vehicles).map((vehicle) => ({ ...vehicle }));
      let workingCouplings = (scenarioInitialState?.couplings || couplings).map((coupling) => ({ ...coupling }));
      const historyDraft = [];
      let lastLocomotiveId = null;
      let lastTargetPathId = null;
      let lastTargetIndex = null;

      for (let index = 0; index < steps.length; index += 1) {
        const step = steps[index];
        const beforeState = cloneLayoutState({
          segments: workingSegments,
          vehicles: workingVehicles,
          couplings: workingCouplings,
        });
        const result = await executeSingleScenarioStep(
          step,
          index,
          steps.length,
          beforeState.vehicles.map((vehicle) => ({ ...vehicle })),
          beforeState.couplings.map((coupling) => ({ ...coupling })),
          false
        );

        workingVehicles = (result.vehicles || []).map((vehicle) => ({ ...vehicle }));
        workingCouplings = (result.couplings || []).map((coupling) => ({ ...coupling }));
        if (result.lastLocomotiveId) {
          lastLocomotiveId = result.lastLocomotiveId;
        }
        if (result.lastTargetPathId) {
          lastTargetPathId = result.lastTargetPathId;
          lastTargetIndex = result.lastTargetIndex;
        }
        historyDraft[index] = {
          beforeState,
          afterState: cloneLayoutState({
            segments: workingSegments,
            vehicles: workingVehicles,
            couplings: workingCouplings,
          }),
          stepType: step.type,
          timeline: [],
        };
      }

      setScenarioStateHistory(historyDraft);
      applyLayoutSnapshot({
        segments: workingSegments,
        vehicles: workingVehicles,
        couplings: workingCouplings,
      });
      if (lastLocomotiveId) {
        setSelectedLocomotiveId(lastLocomotiveId);
      }
      if (lastTargetPathId) {
        setTargetPathId(lastTargetPathId);
        setTargetPathIndex(lastTargetIndex);
      }
      setCurrentScenarioStep(steps.length);
      setScenarioExecutingStep(null);
      setScenarioViewMode("final");
      setMovementHint("Показано финальное состояние сценария.");
    } catch (error) {
      setScenarioExecutingStep(null);
      setScenarioViewMode("paused");
      setMovementHint(error.message || "Не удалось показать финальное состояние.");
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
  const scenarioStepDisplay = scenarioSteps.length
    ? Math.min(Math.max(currentScenarioStep, 0), scenarioSteps.length)
    : 0;
  const scenarioActiveStepIndex =
    scenarioViewMode === "play" && scenarioExecutingStep != null
      ? scenarioExecutingStep
      : scenarioViewMode === "step" && currentScenarioStep > 0
        ? currentScenarioStep - 1
        : null;
  const isScenarioStartHighlighted =
    scenarioViewMode === "start" ||
    (scenarioViewMode !== "play" && scenarioActiveStepIndex == null && currentScenarioStep <= 0);
  const isScenarioFinalHighlighted =
    scenarioViewMode === "final" ||
    (scenarioViewMode !== "play" && scenarioSteps.length > 0 && currentScenarioStep >= scenarioSteps.length);

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
              <button type="button" className="toolButton" onClick={handleDeleteScenario}>
                Удалить сценарий
              </button>
              <select
                className="toolInput"
                value={scenarioStepType}
                onChange={(event) => setScenarioStepType(normalizeScenarioStepType(event.target.value))}
              >
                <option value={SCENARIO_STEP_MOVE}>движение</option>
                <option value={SCENARIO_STEP_COUPLE}>сцепка</option>
                <option value={SCENARIO_STEP_DECOUPLE}>расцепка</option>
              </select>
              {scenarioStepType === SCENARIO_STEP_MOVE ? (
                <>
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
                </>
              ) : (
                <p className="counter">Выбери на схеме 2 объекта и добавь шаг.</p>
              )}
              <button type="button" className="toolButton" onClick={addScenarioStep}>
                Добавить шаг
              </button>
              <button type="button" className="toolButton" onClick={clearScenarioSteps}>
                Очистить шаги
              </button>
              <button type="button" className="toolButton" onClick={handleShowScenarioStart}>
                Показать старт
              </button>
              <button type="button" className="toolButton" onClick={handlePrevScenarioStep}>
                Предыдущий шаг
              </button>
              <button type="button" className="toolButton" onClick={handleNextScenarioStep}>
                Следующий шаг
              </button>
              <button type="button" className="toolButton" onClick={handleShowScenarioFinal}>
                Показать финал
              </button>
              <button
                type="button"
                className="toolButton"
                onClick={scenarioViewMode === "play" ? stopScenarioPlayback : runSimpleScenario}
              >
                {scenarioViewMode === "play" ? "Стоп" : "Выполнить шаги"}
              </button>
              <p className="counter">
                Текущий шаг:{" "}
                {scenarioViewMode === "play" && scenarioExecutingStep != null
                  ? `${scenarioExecutingStep + 1}/${scenarioSteps.length}`
                  : `${scenarioStepDisplay}/${scenarioSteps.length}`}
              </p>
              <p className="counter">{movementHint || "-"}</p>
              {scenarioSteps.length > 0 && (
                <div className="scenarioSteps">
                  <div className={`scenarioStepRow ${isScenarioStartHighlighted ? "current" : ""}`}>
                    <span className="scenarioStepText">Старт</span>
                  </div>
                  {scenarioSteps.map((step, index) => (
                    <div
                      key={step.id}
                      className={`scenarioStepRow ${index === scenarioActiveStepIndex ? "current" : ""}`}
                    >
                      <span className="scenarioStepText">
                        {formatScenarioStepText(normalizeScenarioStep(step), index)}
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
                  <div className={`scenarioStepRow ${isScenarioFinalHighlighted ? "current" : ""}`}>
                    <span className="scenarioStepText">Финал</span>
                  </div>
                </div>
              )}
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

