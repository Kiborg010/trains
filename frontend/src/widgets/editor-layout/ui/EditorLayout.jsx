import { useEffect, useMemo, useRef, useState } from "react";
import { GRID_SIZE, getSegmentSlots, keyOf, snap } from "../../../shared/lib/geometry.js";
import {
  applyLayoutOperation,
  createNormalizedScenario,
  createScheme,
  deleteNormalizedScenario,
  deleteScheme,
  generateFullHeuristicScenario,
  getNormalizedScenarioDetails,
  getScenarioMetrics,
  getSchemeDetails,
  listNormalizedScenarios,
  listSchemes,
  planMovement,
  resolveVehicles,
  updateNormalizedScenario,
  updateScheme,
} from "../../../shared/api/simulation.js";

const DEFAULT_VIEWPORT_WIDTH = 1200;
const DEFAULT_VIEWPORT_HEIGHT = 700;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2.5;
const ZOOM_STEP = 0.1;
const SCENARIO_STEP_MOVE = "MOVE_LOCO";
const SCENARIO_STEP_MOVE_GROUP = "MOVE_GROUP";
const SCENARIO_STEP_COUPLE = "COUPLE";
const SCENARIO_STEP_DECOUPLE = "DECOUPLE";
const PATH_TYPE_MAIN = "main";
const PATH_TYPE_BYPASS = "bypass";
const PATH_TYPE_SORTING = "sorting";
const PATH_TYPE_LEAD = "lead";
const PATH_TYPE_NORMAL = "normal";
const DEFAULT_WAGON_COLOR = "#0ea5e9";
const WAGON_COLOR_PALETTE = [
  DEFAULT_WAGON_COLOR,
  "#22c55e",
  "#f59e0b",
  "#f97316",
  "#ef4444",
  "#a855f7",
  "#14b8a6",
];
const PATH_TYPE_OPTIONS = [
  { value: PATH_TYPE_MAIN, label: "Главный" },
  { value: PATH_TYPE_BYPASS, label: "Объездной" },
  { value: PATH_TYPE_SORTING, label: "Сортировочный" },
  { value: PATH_TYPE_LEAD, label: "Вытяжной" },
  { value: PATH_TYPE_NORMAL, label: "Прочий" },
];
const PATH_TYPE_LABELS = {
  [PATH_TYPE_MAIN]: "Главный",
  [PATH_TYPE_BYPASS]: "Объездной",
  [PATH_TYPE_SORTING]: "Сортировочный",
  [PATH_TYPE_LEAD]: "Вытяжной",
  [PATH_TYPE_NORMAL]: "Прочий",
};
const PATH_TYPE_COLORS = {
  [PATH_TYPE_MAIN]: "#f59e0b",
  [PATH_TYPE_BYPASS]: "#e11d48",
  [PATH_TYPE_SORTING]: "#22c55e",
  [PATH_TYPE_LEAD]: "#38bdf8",
  [PATH_TYPE_NORMAL]: "#334155",
};
const PATH_TYPE_COLORS_DARK = {
  [PATH_TYPE_MAIN]: "#fbbf24",
  [PATH_TYPE_BYPASS]: "#fb7185",
  [PATH_TYPE_SORTING]: "#4ade80",
  [PATH_TYPE_LEAD]: "#7dd3fc",
  [PATH_TYPE_NORMAL]: "#cbd5e1",
};
const SIDEBAR_MIN_WIDTH = 280;
const SIDEBAR_DEFAULT_WIDTH = 320;
const SIDEBAR_MAX_WIDTH = 560;
const CANVAS_THEME_LIGHT = "light";
const CANVAS_THEME_DARK = "dark";
const CANVAS_THEME_COLORS = {
  [CANVAS_THEME_LIGHT]: {
    background: "#ffffff",
    gridMinor: "#d8e2ee",
    gridMajor: "#b6c7db",
    pathLabel: "#0f172a",
    emptySlot: "#cbd5e1",
    occupiedSlot: "#94a3b8",
    node: "#cbd5e1",
    selectionFill: "rgba(37, 99, 235, 0.18)",
    selectionStroke: "#2563eb",
  },
  [CANVAS_THEME_DARK]: {
    background: "#0b1220",
    gridMinor: "#223045",
    gridMajor: "#314760",
    pathLabel: "#e2e8f0",
    emptySlot: "#475569",
    occupiedSlot: "#94a3b8",
    node: "#64748b",
    selectionFill: "rgba(96, 165, 250, 0.18)",
    selectionStroke: "#60a5fa",
  },
};

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function roundGridValue(value) {
  return Math.round(value * 100) / 100;
}

function computeCanvasGridSize(segments) {
  const frequencies = new Map();
  for (const segment of segments || []) {
    const slots = getSegmentSlots(segment);
    if (!Array.isArray(slots) || slots.length < 2) {
      continue;
    }
    for (let index = 1; index < slots.length; index += 1) {
      const prev = slots[index - 1];
      const current = slots[index];
      const distance = Math.hypot(current.x - prev.x, current.y - prev.y);
      if (!Number.isFinite(distance) || distance <= 0) {
        continue;
      }
      const key = roundGridValue(distance);
      frequencies.set(key, (frequencies.get(key) || 0) + 1);
    }
  }

  let bestValue = GRID_SIZE;
  let bestCount = -1;
  for (const [value, count] of frequencies.entries()) {
    if (count > bestCount) {
      bestValue = Number(value);
      bestCount = count;
    }
  }
  return bestValue;
}

function buildGridLines(start, end, step) {
  if (!Number.isFinite(step) || step <= 0 || !Number.isFinite(start) || !Number.isFinite(end)) {
    return [];
  }
  const first = Math.floor(start / step) * step;
  const lines = [];
  for (let value = first; value <= end + step; value += step) {
    lines.push(roundGridValue(value));
  }
  return lines;
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
  if (normalized === SCENARIO_STEP_MOVE_GROUP) {
    return normalized;
  }
  if (normalized === SCENARIO_STEP_COUPLE || normalized === SCENARIO_STEP_DECOUPLE) {
    return normalized;
  }
  return SCENARIO_STEP_MOVE;
}

function normalizePathType(type) {
  const normalized = String(type || "").trim().toLowerCase();
  if (
    normalized === PATH_TYPE_MAIN ||
    normalized === PATH_TYPE_BYPASS ||
    normalized === PATH_TYPE_SORTING ||
    normalized === PATH_TYPE_LEAD ||
    normalized === PATH_TYPE_NORMAL
  ) {
    return normalized;
  }
  return PATH_TYPE_NORMAL;
}

function getSegmentStrokeColor(segmentType, isSelected) {
  if (isSelected) {
    return "#2563eb";
  }
  return PATH_TYPE_COLORS[normalizePathType(segmentType)] || PATH_TYPE_COLORS[PATH_TYPE_NORMAL];
}

function getCanvasSegmentStrokeColor(segmentType, isSelected, canvasTheme) {
  if (isSelected) {
    return "#2563eb";
  }
  const palette = canvasTheme === CANVAS_THEME_DARK ? PATH_TYPE_COLORS_DARK : PATH_TYPE_COLORS;
  return palette[normalizePathType(segmentType)] || palette[PATH_TYPE_NORMAL];
}

function normalizeScenarioStep(step) {
  const type = normalizeScenarioStepType(step?.type);
  const payload = step?.payload && typeof step.payload === "object" ? step.payload : {};
  const id = step?.id || crypto.randomUUID();

  if (type === SCENARIO_STEP_MOVE_GROUP) {
    return {
      id,
      type,
      payload: {
        fromPathId: String(payload.fromPathId ?? step?.fromPathId ?? "").trim(),
        toPathId: String(payload.toPathId ?? step?.toPathId ?? "").trim(),
        heuristicStepType: String(payload.heuristicStepType ?? step?.heuristicStepType ?? "").trim(),
        sourceSide: String(payload.sourceSide ?? step?.sourceSide ?? "").trim(),
        wagonCount: Number(payload.wagonCount ?? step?.wagonCount ?? 0),
        targetColor: String(payload.targetColor ?? step?.targetColor ?? "").trim(),
        formationTrackId: String(payload.formationTrackId ?? step?.formationTrackId ?? "").trim(),
        bufferTrackId: String(payload.bufferTrackId ?? step?.bufferTrackId ?? "").trim(),
        mainTrackId: String(payload.mainTrackId ?? step?.mainTrackId ?? "").trim(),
      },
    };
  }

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

function getSegmentDisplayName(segment, index) {
  const explicitName = String(segment?.name || "").trim();
  if (explicitName) {
    return explicitName;
  }
  return String(index + 1);
}

function mergeSegmentNames(previousSegments, nextSegments) {
  const prevNameById = new Map(
    (previousSegments || []).map((segment) => [segment.id, String(segment?.name || "").trim()])
  );
  const normalizedNextSegments = Array.isArray(nextSegments) ? nextSegments : [];
  const usedNames = new Set();
  let maxNumericName = 0;

  for (const segment of normalizedNextSegments) {
    const explicitName = String(segment?.name || "").trim();
    const preservedName = prevNameById.get(segment?.id) || "";
    const candidate = explicitName || preservedName;
    if (!candidate) {
      continue;
    }
    usedNames.add(candidate);
    const numericValue = Number.parseInt(candidate, 10);
    if (String(numericValue) === candidate && !Number.isNaN(numericValue)) {
      maxNumericName = Math.max(maxNumericName, numericValue);
    }
  }

  return normalizedNextSegments.map((segment) => {
    const explicitName = String(segment?.name || "").trim();
    if (explicitName) {
      return { ...segment, name: explicitName };
    }

    const preservedName = prevNameById.get(segment?.id) || "";
    if (preservedName) {
      return { ...segment, name: preservedName };
    }

    let nextName = "";
    do {
      maxNumericName += 1;
      nextName = String(maxNumericName);
    } while (usedNames.has(nextName));
    usedNames.add(nextName);
    return { ...segment, name: nextName };
  });
}

function getPathDisplayName(pathId, pathNameById) {
  const normalizedId = String(pathId || "").trim();
  if (!normalizedId) {
    return "?";
  }
  return pathNameById.get(normalizedId) || normalizedId;
}

function formatPathReference(pathId, pathIndex, pathNameById) {
  return `${getPathDisplayName(pathId, pathNameById)}:${pathIndex}`;
}

function formatHumanReadableTrackLabel(pathId, pathNameById) {
  const normalizedId = String(pathId || "").trim();
  if (!normalizedId) {
    return "-";
  }

  const explicitName = pathNameById?.get?.(normalizedId);
  if (explicitName) {
    return `${explicitName} (${normalizedId})`;
  }

  const ordinalCandidate = extractTrackOrdinalCandidate(normalizedId);
  if (ordinalCandidate) {
    return `Путь ${ordinalCandidate} (${normalizedId})`;
  }

  return normalizedId;
}

function formatHeuristicDraftStepText(step, pathNameById) {
  const count = Number(step?.wagon_count || 0);
  const sourceTrackId = String(step?.source_track_id || step?.source_track_id || step?.sourceTrackID || "").trim();
  const destinationTrackId = String(
    step?.destination_track_id || step?.destinationTrackID || ""
  ).trim();
  const sourceSide = String(step?.source_side || step?.sourceSide || "").trim();
  const type = String(step?.step_type || step?.stepType || "").trim();
  const sourceLabel = formatHumanReadableTrackLabel(sourceTrackId, pathNameById);
  const destinationLabel = formatHumanReadableTrackLabel(destinationTrackId, pathNameById);

  if (type === "buffer_blockers") {
    return `Перенести ${count} блокирующих вагонов с пути ${sourceLabel} в буферный путь ${destinationLabel}`;
  }
  if (type === "transfer_targets_to_formation") {
    const sideText = sourceSide ? ` (со стороны ${sourceSide})` : "";
    return `Перенести ${count} целевых вагонов с пути ${sourceLabel}${sideText} на путь формирования ${destinationLabel}`;
  }
  if (type === "transfer_formation_to_main") {
    return `Перевести сформированный состав с пути ${sourceLabel} на главный путь ${destinationLabel}`;
  }
  return `${type}: ${sourceLabel} -> ${destinationLabel}`;
}

function extractTrackOrdinalCandidate(pathId) {
  const raw = String(pathId || "").trim();
  if (!raw) {
    return "";
  }
  const directDigits = raw.match(/^\d+$/);
  if (directDigits) {
    return directDigits[0];
  }
  const suffix = raw.match(/track-(\d+)$/i);
  if (suffix) {
    return suffix[1];
  }
  return "";
}

function buildSegmentTrackIdAliasMap(segments) {
  const map = new Map();
  (segments || []).forEach((segment) => {
    const segmentId = String(segment?.id || "").trim();
    const ordinalCandidate = extractTrackOrdinalCandidate(segmentId);
    if (segmentId) {
      map.set(segmentId, segmentId);
    }
    if (ordinalCandidate) {
      map.set(ordinalCandidate, segmentId);
    }
  });
  return map;
}

function canonicalizeTrackIdForSegments(pathId, segments) {
  const raw = String(pathId || "").trim();
  if (!raw) {
    return "";
  }
  const aliasMap = buildSegmentTrackIdAliasMap(segments);
  const ordinalCandidate = extractTrackOrdinalCandidate(raw);
  return aliasMap.get(raw) || aliasMap.get(ordinalCandidate) || raw;
}

function remapScenarioStepsToCurrentSegments(rawSteps, segments) {
  return (rawSteps || []).map((rawStep) => {
    const step = normalizeScenarioStep(rawStep);
    if (step.type !== SCENARIO_STEP_MOVE) {
      return step;
    }
    return {
      ...step,
      payload: {
        ...step.payload,
        fromPathId: canonicalizeTrackIdForSegments(step.payload.fromPathId, segments),
        toPathId: canonicalizeTrackIdForSegments(step.payload.toPathId, segments),
      },
    };
  });
}

function buildPersistedTrackIdMap(tracks) {
  const map = new Map();
  (tracks || []).forEach((track) => {
    const trackId = String(track?.track_id || "").trim();
    const ordinalCandidate = extractTrackOrdinalCandidate(trackId);
    if (trackId) {
      map.set(trackId, trackId);
    }
    if (ordinalCandidate) {
      map.set(ordinalCandidate, trackId);
    }
  });
  return map;
}

function remapScenarioStepsToPersistedTracks(rawSteps, tracks) {
  const persistedTrackIdMap = buildPersistedTrackIdMap(tracks);
  return (rawSteps || []).map((rawStep) => {
    const step = normalizeScenarioStep(rawStep);
    if (step.type !== SCENARIO_STEP_MOVE) {
      return step;
    }
    const fromCandidate = extractTrackOrdinalCandidate(step.payload.fromPathId);
    const toCandidate = extractTrackOrdinalCandidate(step.payload.toPathId);
    return {
      ...step,
      payload: {
        ...step.payload,
        fromPathId:
          persistedTrackIdMap.get(step.payload.fromPathId) ||
          persistedTrackIdMap.get(fromCandidate) ||
          step.payload.fromPathId,
        toPathId:
          persistedTrackIdMap.get(step.payload.toPathId) ||
          persistedTrackIdMap.get(toCandidate) ||
          step.payload.toPathId,
      },
    };
  });
}

function formatScenarioStepText(step, index, pathNameById) {
  if (step.type === SCENARIO_STEP_MOVE_GROUP) {
    return `${index + 1}. ГРУППА: ${formatHeuristicDraftStepText(
      {
        step_type: step.payload.heuristicStepType,
        source_track_id: step.payload.fromPathId,
        destination_track_id: step.payload.toPathId,
        source_side: step.payload.sourceSide,
        wagon_count: step.payload.wagonCount,
      },
      pathNameById
    )}`;
  }
  if (step.type === SCENARIO_STEP_MOVE) {
    return `${index + 1}. ${step.payload.unitCode}: ${formatPathReference(
      step.payload.fromPathId,
      step.payload.fromIndex,
      pathNameById
    )} -> ${formatPathReference(step.payload.toPathId, step.payload.toIndex, pathNameById)}`;
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

function mergeVehicleColors(previousVehicles, nextVehicles, rememberedColors) {
  const nextList = Array.isArray(nextVehicles) ? nextVehicles : [];
  const prevById = new Map((Array.isArray(previousVehicles) ? previousVehicles : []).map((v) => [v.id, v]));
  return nextList.map((vehicle) => {
    if (!vehicle || typeof vehicle !== "object") {
      return vehicle;
    }
    if (vehicle.color) {
      return vehicle;
    }
    const prev = prevById.get(vehicle.id);
    const rememberedColor =
      rememberedColors && typeof rememberedColors.get === "function"
        ? rememberedColors.get(vehicle.id)
        : "";
    if (!prev?.color && !rememberedColor) {
      return vehicle;
    }
    return { ...vehicle, color: prev?.color || rememberedColor };
  });
}

function getTrackStorageAllowed(type) {
  const normalizedType = normalizePathType(type);
  return normalizedType === PATH_TYPE_SORTING || normalizedType === PATH_TYPE_LEAD;
}

function buildTrackConnectionsFromSegments(segments) {
  const byNode = new Map();

  for (const segment of segments) {
    const endpoints = [
      { key: keyOf(segment.from.x, segment.from.y), side: "start" },
      { key: keyOf(segment.to.x, segment.to.y), side: "end" },
    ];
    for (const endpoint of endpoints) {
      const list = byNode.get(endpoint.key) || [];
      list.push({ segment, side: endpoint.side });
      byNode.set(endpoint.key, list);
    }
  }

  const seen = new Set();
  const connections = [];
  for (const entries of byNode.values()) {
    if (entries.length < 2) {
      continue;
    }
    const connectionType = entries.length > 2 ? "switch" : "serial";
    for (let i = 0; i < entries.length; i += 1) {
      for (let j = i + 1; j < entries.length; j += 1) {
        const a = entries[i];
        const b = entries[j];
        const id = `${a.segment.id}:${b.segment.id}:${a.side}:${b.side}`;
        if (seen.has(id)) {
          continue;
        }
        seen.add(id);
        connections.push({
          connection_id: id,
          track1_id: a.segment.id,
          track2_id: b.segment.id,
          track1_side: a.side,
          track2_side: b.side,
          connection_type: connectionType,
        });
      }
    }
  }
  return connections;
}

function endpointRefKey(segmentId, side) {
  return `${segmentId}:${side}`;
}

function getSegmentSlotsWithEndpointOverrides(segment, endpointOverrides) {
  const startOverride = endpointOverrides.get(endpointRefKey(segment.id, "start"));
  const endOverride = endpointOverrides.get(endpointRefKey(segment.id, "end"));
  if (!startOverride && !endOverride) {
    return getSegmentSlots(segment, GRID_SIZE);
  }
  return getSegmentSlots(
    {
      ...segment,
      from: startOverride || segment.from,
      to: endOverride || segment.to,
    },
    GRID_SIZE
  );
}

function getPathSlotPoint(segments, endpointOverrides, pathId, pathIndex) {
  const segment = (segments || []).find((item) => item.id === pathId);
  if (!segment) {
    return null;
  }
  const points = getSegmentSlotsWithEndpointOverrides(segment, endpointOverrides);
  const index = Number(pathIndex);
  if (!Number.isInteger(index) || index < 0 || index >= points.length) {
    return null;
  }
  return points[index];
}

function arePathPositionsEquivalent(segments, endpointOverrides, leftPathId, leftPathIndex, rightPathId, rightPathIndex) {
  const left = getPathSlotPoint(segments, endpointOverrides, leftPathId, leftPathIndex);
  const right = getPathSlotPoint(segments, endpointOverrides, rightPathId, rightPathIndex);
  if (!left || !right) {
    return false;
  }
  return Math.abs(left.x - right.x) < 1e-9 && Math.abs(left.y - right.y) < 1e-9;
}

function buildSharedEndpointModel(segments) {
  const connections = buildTrackConnectionsFromSegments(segments);
  const parent = new Map();
  const endpointPoints = new Map();

  const addEndpoint = (segmentId, side, point) => {
    const key = endpointRefKey(segmentId, side);
    if (!parent.has(key)) {
      parent.set(key, key);
    }
    if (point) {
      endpointPoints.set(key, point);
    }
  };

  for (const segment of segments) {
    addEndpoint(segment.id, "start", segment.from);
    addEndpoint(segment.id, "end", segment.to);
  }

  const find = (key) => {
    const root = parent.get(key);
    if (!root || root === key) {
      return key;
    }
    const compressed = find(root);
    parent.set(key, compressed);
    return compressed;
  };

  const union = (a, b) => {
    const rootA = find(a);
    const rootB = find(b);
    if (rootA !== rootB) {
      parent.set(rootB, rootA);
    }
  };

  for (const connection of connections) {
    const a = endpointRefKey(connection.track1_id, connection.track1_side);
    const b = endpointRefKey(connection.track2_id, connection.track2_side);
    if (parent.has(a) && parent.has(b)) {
      union(a, b);
    }
  }

  const groupedRefs = new Map();
  for (const key of parent.keys()) {
    const root = find(key);
    const list = groupedRefs.get(root) || [];
    list.push(key);
    groupedRefs.set(root, list);
  }

  const endpointOverrides = new Map();
  const nodes = [];
  for (const refs of groupedRefs.values()) {
    let sumX = 0;
    let sumY = 0;
    let count = 0;
    const endpoints = [];

    for (const ref of refs) {
      const point = endpointPoints.get(ref);
      if (!point) {
        continue;
      }
      const [segmentId, endpoint] = ref.split(":");
      sumX += point.x;
      sumY += point.y;
      count += 1;
      endpoints.push({ segmentId, endpoint });
    }

    if (!count) {
      continue;
    }

    const sharedPoint = { x: sumX / count, y: sumY / count };
    for (const ref of refs) {
      endpointOverrides.set(ref, sharedPoint);
    }

    nodes.push({
      x: sharedPoint.x,
      y: sharedPoint.y,
      endpoints,
    });
  }

  return { endpointOverrides, nodes };
}

function normalizeEditorLayoutForSave(
  segments,
  vehicles,
  couplings,
  scenarioSteps = [],
  schemeKey = ""
) {
  const prefix = schemeKey ? `scheme-${schemeKey}` : `draft-${crypto.randomUUID()}`;
  const nextSegments = (segments || []).map((segment, index) => ({
    ...segment,
    id: `${prefix}-track-${index + 1}`,
  }));

  const findNearestSlot = (vehicle) => {
    let best = null;
    for (const segment of nextSegments) {
      const slots = getSegmentSlots(segment, GRID_SIZE);
      for (let index = 0; index < slots.length; index += 1) {
        const slot = slots[index];
        const dx = slot.x - Number(vehicle.x || 0);
        const dy = slot.y - Number(vehicle.y || 0);
        const distance = dx * dx + dy * dy;
        if (!best || distance < best.distance) {
          best = { pathId: segment.id, pathIndex: index, distance };
        }
      }
    }
    return best;
  };

  const nextVehicles = (vehicles || []).map((vehicle) => {
    const nearest = findNearestSlot(vehicle);
    if (!nearest) {
      return { ...vehicle };
    }
    return {
      ...vehicle,
      pathId: nearest.pathId,
      pathIndex: nearest.pathIndex,
    };
  });

  const fallbackPathMap = new Map();
  nextSegments.forEach((segment, index) => {
    fallbackPathMap.set((segments[index]?.id || "").trim(), segment.id);
  });

  const nextScenarioSteps = (scenarioSteps || []).map((step) => {
    const normalized = normalizeScenarioStep(step);
    if (normalized.type !== SCENARIO_STEP_MOVE) {
      return normalized;
    }
    return {
      ...normalized,
      payload: {
        ...normalized.payload,
        fromPathId: fallbackPathMap.get(normalized.payload.fromPathId) || normalized.payload.fromPathId,
        toPathId: fallbackPathMap.get(normalized.payload.toPathId) || normalized.payload.toPathId,
      },
    };
  });

  return {
    segments: nextSegments,
    vehicles: nextVehicles,
    couplings: (couplings || []).map((coupling) => ({ ...coupling })),
    scenarioSteps: nextScenarioSteps,
  };
}

function buildNormalizedSchemePayload(name, segments, vehicles, couplings) {
  return {
    name: name.trim() || "Схема",
    tracks: segments.map((segment, index) => ({
      track_id: segment.id,
      name: getSegmentDisplayName(segment, index),
      type: normalizePathType(segment.type),
      start_x: segment.from.x,
      start_y: segment.from.y,
      end_x: segment.to.x,
      end_y: segment.to.y,
      capacity: getSegmentSlots(segment, GRID_SIZE).length,
      storage_allowed: getTrackStorageAllowed(segment.type),
    })),
    track_connections: buildTrackConnectionsFromSegments(segments),
    wagons: vehicles
      .filter((vehicle) => vehicle.type === "wagon")
      .map((vehicle) => ({
        wagon_id: vehicle.id,
        name: vehicle.code || vehicle.id,
        color: vehicle.color || DEFAULT_WAGON_COLOR,
        track_id: vehicle.pathId || "",
        track_index: Number(vehicle.pathIndex ?? 0),
      })),
    locomotives: vehicles
      .filter((vehicle) => vehicle.type === "locomotive")
      .map((vehicle) => ({
        loco_id: vehicle.id,
        name: vehicle.code || vehicle.id,
        color: vehicle.color || "#dc2626",
        track_id: vehicle.pathId || "",
        track_index: Number(vehicle.pathIndex ?? 0),
      })),
    couplings: couplings.map((coupling) => ({
      coupling_id: coupling.id,
      object1_id: coupling.a,
      object2_id: coupling.b,
    })),
  };
}

function cloneNormalizedSchemePayloadWithNewIds(payload) {
  const trackIdMap = new Map();
  const objectIdMap = new Map();

  const nextTracks = (payload?.tracks || []).map((track) => {
    const nextId = `track-${crypto.randomUUID()}`;
    trackIdMap.set(track.track_id, nextId);
    return {
      ...track,
      track_id: nextId,
    };
  });

  const nextWagons = (payload?.wagons || []).map((wagon) => {
    const nextId = `wagon-${crypto.randomUUID()}`;
    objectIdMap.set(wagon.wagon_id, nextId);
    return {
      ...wagon,
      wagon_id: nextId,
      track_id: trackIdMap.get(wagon.track_id) || wagon.track_id,
    };
  });

  const nextLocomotives = (payload?.locomotives || []).map((locomotive) => {
    const nextId = `loco-${crypto.randomUUID()}`;
    objectIdMap.set(locomotive.loco_id, nextId);
    return {
      ...locomotive,
      loco_id: nextId,
      track_id: trackIdMap.get(locomotive.track_id) || locomotive.track_id,
    };
  });

  const nextConnections = (payload?.track_connections || []).map((connection) => ({
    ...connection,
    connection_id: `connection-${crypto.randomUUID()}`,
    track1_id: trackIdMap.get(connection.track1_id) || connection.track1_id,
    track2_id: trackIdMap.get(connection.track2_id) || connection.track2_id,
  }));

  const nextCouplings = (payload?.couplings || []).map((coupling) => ({
    ...coupling,
    coupling_id: `coupling-${crypto.randomUUID()}`,
    object1_id: objectIdMap.get(coupling.object1_id) || coupling.object1_id,
    object2_id: objectIdMap.get(coupling.object2_id) || coupling.object2_id,
  }));

  return {
    ...payload,
    tracks: nextTracks,
    track_connections: nextConnections,
    wagons: nextWagons,
    locomotives: nextLocomotives,
    couplings: nextCouplings,
  };
}

function buildEditorStateFromSchemeDetails(details) {
  const tracks = Array.isArray(details?.tracks) ? details.tracks : [];
  const segments = tracks.map((track, index) => ({
    id: track.track_id,
    name: String(track.name || "").trim() || String(index + 1),
    type: normalizePathType(track.type),
    from: { x: track.start_x, y: track.start_y },
    to: { x: track.end_x, y: track.end_y },
  }));
  const segmentById = new Map(segments.map((segment) => [segment.id, segment]));

  function buildVehicle(item, type) {
    const segment = segmentById.get(item.track_id);
    const slots = segment ? getSegmentSlots(segment, GRID_SIZE) : [];
    const point = slots[item.track_index] || segment?.from || { x: 0, y: 0 };
    return {
      id: type === "locomotive" ? item.loco_id : item.wagon_id,
      type,
      code: item.name,
      color: type === "wagon" ? item.color || DEFAULT_WAGON_COLOR : item.color || "#dc2626",
      pathId: item.track_id,
      pathIndex: item.track_index,
      x: point.x,
      y: point.y,
    };
  }

  return {
    segments,
    vehicles: [
      ...(details?.wagons || []).map((item) => buildVehicle(item, "wagon")),
      ...(details?.locomotives || []).map((item) => buildVehicle(item, "locomotive")),
    ],
    couplings: (details?.couplings || []).map((coupling) => ({
      id: coupling.coupling_id,
      a: coupling.object1_id,
      b: coupling.object2_id,
    })),
  };
}

function buildNormalizedScenarioPayload(name, schemeId, scenarioSteps, sourceVehicles) {
  const codeByVehicleId = buildVehicleCodeMap(sourceVehicles);
  const idByCode = new Map();
  for (const vehicle of sourceVehicles) {
    const code = normalizeUnitCode(codeByVehicleId.get(vehicle.id));
    if (code) {
      idByCode.set(code, vehicle.id);
    }
  }

  return {
    name: name.trim() || "Сценарий",
    scheme_id: schemeId,
    scenario_steps: scenarioSteps.map((rawStep, index) => {
      const step = normalizeScenarioStep(rawStep);
      if (step.type === SCENARIO_STEP_MOVE_GROUP) {
        return {
          step_id: step.id || crypto.randomUUID(),
          step_order: index,
          step_type: "move_group",
          from_track_id: step.payload.fromPathId,
          to_track_id: step.payload.toPathId,
          payload_json: {
            heuristic_step_type: step.payload.heuristicStepType,
            source_side: step.payload.sourceSide,
            wagon_count: step.payload.wagonCount,
            target_color: step.payload.targetColor,
            formation_track_id: step.payload.formationTrackId,
            buffer_track_id: step.payload.bufferTrackId,
            main_track_id: step.payload.mainTrackId,
          },
        };
      }
      if (step.type === SCENARIO_STEP_MOVE) {
        const locoId = idByCode.get(normalizeUnitCode(step.payload.unitCode));
        if (!locoId) {
          throw new Error(`Локомотив ${step.payload.unitCode || "?"} не найден для сохранения.`);
        }
        return {
          step_id: step.id || crypto.randomUUID(),
          step_order: index,
          step_type: "move_loco",
          from_track_id: step.payload.fromPathId,
          from_index: step.payload.fromIndex,
          to_track_id: step.payload.toPathId,
          to_index: step.payload.toIndex,
          object1_id: locoId,
        };
      }

      const [aCode, bCode] = step.payload.unitCodes || [];
      const aId = idByCode.get(normalizeUnitCode(aCode));
      const bId = idByCode.get(normalizeUnitCode(bCode));
      if (!aId || !bId) {
        throw new Error(`Объекты ${aCode || "?"} и ${bCode || "?"} не найдены для сохранения.`);
      }
      return {
        step_id: step.id || crypto.randomUUID(),
        step_order: index,
        step_type: step.type === SCENARIO_STEP_COUPLE ? "couple" : "decouple",
        object1_id: aId,
        object2_id: bId,
      };
    }),
  };
}

function buildScenarioStepsFromNormalizedScenario(details, sourceVehicles = []) {
  if (!details || !Array.isArray(details.scenario_steps)) {
    return [];
  }

  const steps = [];
  const codeByVehicleId = buildVehicleCodeMap(sourceVehicles);

  for (const step of details.scenario_steps) {
    const type =
      step.step_type === "move_group"
        ? SCENARIO_STEP_MOVE_GROUP
        : step.step_type === "couple"
          ? SCENARIO_STEP_COUPLE
          : step.step_type === "decouple"
            ? SCENARIO_STEP_DECOUPLE
            : SCENARIO_STEP_MOVE;

    if (type === SCENARIO_STEP_MOVE_GROUP) {
      const payload = step.payload_json && typeof step.payload_json === "object" ? step.payload_json : {};
      steps.push({
        id: step.step_id || crypto.randomUUID(),
        type,
        payload: {
          fromPathId: String(step.from_track_id || "").trim(),
          toPathId: String(step.to_track_id || "").trim(),
          heuristicStepType: String(payload.heuristic_step_type || "").trim(),
          sourceSide: String(payload.source_side || "").trim(),
          wagonCount: Number(payload.wagon_count || 0),
          targetColor: String(payload.target_color || "").trim(),
          formationTrackId: String(payload.formation_track_id || "").trim(),
          bufferTrackId: String(payload.buffer_track_id || "").trim(),
          mainTrackId: String(payload.main_track_id || "").trim(),
        },
      });
      continue;
    }

    if (type === SCENARIO_STEP_MOVE) {
      const locoId = step.object1_id;
      const fromPathId = String(step.from_track_id || "").trim();
      const fromIndex =
        step.from_index == null || step.from_index === ""
          ? Number.NaN
          : Number(step.from_index);
      const toPathId = String(step.to_track_id || "").trim();
      const toIndex = Number(step.to_index ?? 0);

      steps.push({
        id: step.step_id || crypto.randomUUID(),
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
      id: step.step_id || crypto.randomUUID(),
      type,
      payload: {
        unitCodes: [
          normalizeUnitCode(codeByVehicleId.get(step.object1_id)) || "",
          normalizeUnitCode(codeByVehicleId.get(step.object2_id)) || "",
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
  const scenarioStepEditorRef = useRef(null);
  const sidebarResizeActiveRef = useRef(false);
  const vehicleColorMemoryRef = useRef(new Map());

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
  const [heuristicLocomotiveId, setHeuristicLocomotiveId] = useState("");
  const [wagonPaintColor, setWagonPaintColor] = useState(DEFAULT_WAGON_COLOR);
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
  const [scenarioLinkObject1Code, setScenarioLinkObject1Code] = useState("");
  const [scenarioLinkObject2Code, setScenarioLinkObject2Code] = useState("");
  const [scenarioStepType, setScenarioStepType] = useState(SCENARIO_STEP_MOVE);
  const [scenarioSteps, setScenarioSteps] = useState([]);
  const [selectedScenarioListStepId, setSelectedScenarioListStepId] = useState("");
  const [selectedScenarioStepId, setSelectedScenarioStepId] = useState("");
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
  const [sidebarWidth, setSidebarWidth] = useState(SIDEBAR_DEFAULT_WIDTH);
  const [canvasTheme, setCanvasTheme] = useState(CANVAS_THEME_LIGHT);
  const [savedScenarios, setSavedScenarios] = useState([]);
  const [selectedScenarioId, setSelectedScenarioId] = useState("");
  const [selectedMetricsScenarioId, setSelectedMetricsScenarioId] = useState("");
  const [scenarioMetrics, setScenarioMetrics] = useState(null);
  const [scenarioMetricsError, setScenarioMetricsError] = useState("");
  const [isScenarioMetricsLoading, setIsScenarioMetricsLoading] = useState(false);
  const [heuristicTargetColor, setHeuristicTargetColor] = useState("");
  const [heuristicRequiredTargetCount, setHeuristicRequiredTargetCount] = useState("1");
  const [heuristicFormationTrackId, setHeuristicFormationTrackId] = useState("");
  const [heuristicDraftName, setHeuristicDraftName] = useState("");
  const [heuristicDraftResult, setHeuristicDraftResult] = useState(null);
  const [heuristicDraftError, setHeuristicDraftError] = useState("");
  const [isGeneratingHeuristicDraft, setIsGeneratingHeuristicDraft] = useState(false);
  const [isHeuristicSectionCollapsed, setIsHeuristicSectionCollapsed] = useState(false);
  const [isScenarioSectionCollapsed, setIsScenarioSectionCollapsed] = useState(false);
  const [isScenarioStepEditorCollapsed, setIsScenarioStepEditorCollapsed] = useState(false);
  const [isScenarioPlaybackSectionCollapsed, setIsScenarioPlaybackSectionCollapsed] = useState(false);
  const [isScenarioStepNavigationCollapsed, setIsScenarioStepNavigationCollapsed] = useState(false);
  const [isScenarioStepsSectionCollapsed, setIsScenarioStepsSectionCollapsed] = useState(false);
  const [isLayoutSchemesSectionCollapsed, setIsLayoutSchemesSectionCollapsed] = useState(false);
  const [isLayoutAddSectionCollapsed, setIsLayoutAddSectionCollapsed] = useState(false);
  const [isLayoutObjectsSectionCollapsed, setIsLayoutObjectsSectionCollapsed] = useState(false);
  const [isCouplingActionsSectionCollapsed, setIsCouplingActionsSectionCollapsed] = useState(false);
  const [isMovementActionsSectionCollapsed, setIsMovementActionsSectionCollapsed] = useState(false);
  const [isMetricsSectionCollapsed, setIsMetricsSectionCollapsed] = useState(false);

  useEffect(() => {
    function handlePointerMove(event) {
      if (!sidebarResizeActiveRef.current) {
        return;
      }
      const nextWidth = clamp(event.clientX, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH);
      setSidebarWidth(nextWidth);
    }

    function handlePointerUp() {
      sidebarResizeActiveRef.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    }

    window.addEventListener("mousemove", handlePointerMove);
    window.addEventListener("mouseup", handlePointerUp);
    return () => {
      window.removeEventListener("mousemove", handlePointerMove);
      window.removeEventListener("mouseup", handlePointerUp);
    };
  }, []);

  function handleSidebarResizeStart(event) {
    event.preventDefault();
    sidebarResizeActiveRef.current = true;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }

  const viewWidth = viewport.width / zoom;
  const viewHeight = viewport.height / zoom;
  const isManeuversPanel = activePanel === "maneuvers";
  const isCouplingPanel = activePanel === "coupling";
  const isMovementPanel = activePanel === "movement";
  const isEditMode = isManeuversPanel && mode === "edit";
  const isPlaceMode =
    isManeuversPanel && (mode === "placeWagon" || mode === "placeLocomotive");
  const isMoveMode = isMovementPanel && mode === "move";
  const isPaintMode = isManeuversPanel && mode === "paintWagon";

  const selectedSegmentSet = useMemo(() => new Set(selectedSegmentIds), [selectedSegmentIds]);
  const selectedVehicleSet = useMemo(() => new Set(selectedVehicleIds), [selectedVehicleIds]);
  const vehicleById = useMemo(() => new Map(vehicles.map((v) => [v.id, v])), [vehicles]);
  const vehicleCodeById = useMemo(() => buildVehicleCodeMap(vehicles), [vehicles]);
  const segmentDisplayNameById = useMemo(() => {
    const map = new Map();
    segments.forEach((segment, index) => {
      map.set(segment.id, getSegmentDisplayName(segment, index));
    });
    return map;
  }, [segments]);
  const locomotiveOptions = useMemo(
    () => vehicles.filter((vehicle) => vehicle.type === "locomotive"),
    [vehicles]
  );
  const selectedSegmentsType = useMemo(() => {
    if (!selectedSegmentIds.length) {
      return "";
    }
    const selected = segments.filter((segment) => selectedSegmentSet.has(segment.id));
    if (!selected.length) {
      return "";
    }
    const firstType = normalizePathType(selected[0].type);
    return selected.every((segment) => normalizePathType(segment.type) === firstType)
      ? firstType
      : "";
  }, [segments, selectedSegmentIds, selectedSegmentSet]);

  const sharedEndpointModel = useMemo(() => buildSharedEndpointModel(segments), [segments]);
  const nodes = useMemo(() => sharedEndpointModel.nodes, [sharedEndpointModel]);

  const railSlots = useMemo(() => {
    const map = new Map();
    for (const segment of segments) {
      const points = getSegmentSlotsWithEndpointOverrides(segment, sharedEndpointModel.endpointOverrides);
      points.forEach((point, index) => {
        const id = `${segment.id}:${index}`;
        map.set(id, { id, pathId: segment.id, index, x: point.x, y: point.y });
      });
    }
    return [...map.values()];
  }, [segments, sharedEndpointModel]);

  const occupiedSlots = useMemo(
    () =>
      new Set(
        vehicles
          .filter((vehicle) => vehicle.pathId != null)
          .map((vehicle) => `${vehicle.pathId}:${vehicle.pathIndex}`)
      ),
    [vehicles]
  );
  const pathOptions = useMemo(
    () =>
      segments.map((segment, index) => ({
        value: segment.id,
        label: getSegmentDisplayName(segment, index),
      })),
    [segments]
  );
  const leadPathOptions = useMemo(
    () =>
      segments
        .filter((segment) => normalizePathType(segment.type) === PATH_TYPE_LEAD)
        .map((segment, index) => ({
          value: segment.id,
          label: getSegmentDisplayName(segment, index),
        })),
    [segments]
  );

  useEffect(() => {
    const memory = vehicleColorMemoryRef.current;
    const liveIds = new Set();
    for (const vehicle of vehicles) {
      liveIds.add(vehicle.id);
      if (vehicle?.color) {
        memory.set(vehicle.id, vehicle.color);
      }
    }
    for (const id of [...memory.keys()]) {
      if (!liveIds.has(id)) {
        memory.delete(id);
      }
    }
  }, [vehicles]);

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
          hasVehiclePositionChanges(prev, response.vehicles)
            ? mergeVehicleColors(prev, response.vehicles, vehicleColorMemoryRef.current)
            : prev
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
          listSchemes(),
          listNormalizedScenarios(),
        ]);
        if (!cancelled) {
          setSavedLayouts(layoutsResp.schemes || []);
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

  useEffect(() => {
    if (!savedScenarios.length) {
      setScenarioMetrics(null);
      setScenarioMetricsError("");
      return;
    }
    setSelectedMetricsScenarioId((prev) => {
      if (prev && savedScenarios.some((scenario) => String(scenario.scenario_id) === String(prev))) {
        return prev;
      }
      if (selectedScenarioId && savedScenarios.some((scenario) => String(scenario.scenario_id) === String(selectedScenarioId))) {
        return String(selectedScenarioId);
      }
      return String(savedScenarios[0].scenario_id);
    });
  }, [savedScenarios, selectedScenarioId]);

  useEffect(() => {
    let cancelled = false;

    async function loadScenarioMetrics() {
      if (!selectedMetricsScenarioId) {
        setScenarioMetrics(null);
        setScenarioMetricsError("");
        return;
      }

      setIsScenarioMetricsLoading(true);
      setScenarioMetricsError("");
      try {
        const response = await getScenarioMetrics(selectedMetricsScenarioId);
        if (!cancelled) {
          setScenarioMetrics(response.metrics || null);
        }
      } catch (error) {
        if (!cancelled) {
          setScenarioMetrics(null);
          setScenarioMetricsError(error.message || "Не удалось загрузить метрики сценария.");
        }
      } finally {
        if (!cancelled) {
          setIsScenarioMetricsLoading(false);
        }
      }
    }

    loadScenarioMetrics();
    return () => {
      cancelled = true;
    };
  }, [selectedMetricsScenarioId]);

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
      throw new Error(response.message || "Не удалось выполнить действие с расстановкой.");
    }

    const nextState = response.state || {};
    setSegments((prev) => mergeSegmentNames(prev, nextState.segments || []));
    setVehicles((prev) =>
      mergeVehicleColors(prev, nextState.vehicles || [], vehicleColorMemoryRef.current)
    );
    setCouplings(nextState.couplings || []);
    if (response.message) {
      setMovementHint(response.message);
    }
    return response;
  }

  async function refreshSavedLayouts() {
    const response = await listSchemes();
    setSavedLayouts(response.schemes || []);
  }

  async function refreshSavedScenarios() {
    const response = await listNormalizedScenarios();
    setSavedScenarios(response.scenarios || []);
  }

  async function handleSaveLayout() {
    try {
      const normalizedLayoutName = String(layoutName || "").trim();
      if (!normalizedLayoutName) {
        setMovementHint("Укажи название схемы перед сохранением.");
        return;
      }

      const normalizedState = normalizeEditorLayoutForSave(
        segments,
        vehicles,
        couplings,
        scenarioSteps,
        selectedLayoutId
      );
      const normalizedVehiclesForSave = normalizedState.vehicles.map((vehicle) =>
        vehicle.type === "wagon" ? { ...vehicle, color: vehicle.color || DEFAULT_WAGON_COLOR } : { ...vehicle }
      );

      const payload = buildNormalizedSchemePayload(
        normalizedLayoutName,
        normalizedState.segments,
        normalizedVehiclesForSave,
        normalizedState.couplings
      );

      setSegments((prev) => mergeSegmentNames(prev, normalizedState.segments));
      setVehicles((prev) =>
        mergeVehicleColors(prev, normalizedVehiclesForSave, vehicleColorMemoryRef.current)
      );
      setCouplings(normalizedState.couplings);
      setScenarioSteps(normalizedState.scenarioSteps);

      let response;
      const existingByName = savedLayouts.find(
        (layout) => String(layout?.name || "").trim() === normalizedLayoutName
      );

      if (existingByName?.scheme_id != null) {
        response = await updateScheme(existingByName.scheme_id, payload);
      } else {
        response = await createScheme(cloneNormalizedSchemePayloadWithNewIds(payload));
      }

      const saved = response.scheme;
      if (saved?.scheme_id != null) {
        setSelectedLayoutId(String(saved.scheme_id));
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
      const response = await getSchemeDetails(selectedLayoutId);
      const state = buildEditorStateFromSchemeDetails(response);
      stopMovement(true);
      setSegments((prev) => mergeSegmentNames(prev, state.segments));
      setVehicles((prev) =>
        mergeVehicleColors(prev, state.vehicles, vehicleColorMemoryRef.current)
      );
      setCouplings(state.couplings);
      setLayoutName(String(response?.scheme?.name || "").trim() || layoutName);
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
      await deleteScheme(selectedLayoutId);
      setSelectedLayoutId("");
      await refreshSavedLayouts();
      setMovementHint("Схема удалена.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось удалить схему.");
    }
  }

  function handleSelectedSegmentsTypeChange(nextType) {
    const normalizedType = normalizePathType(nextType);
    setSegments((prev) =>
      prev.map((segment) =>
        selectedSegmentSet.has(segment.id) ? { ...segment, type: normalizedType } : segment
      )
    );
    if (selectedSegmentIds.length > 0) {
      setMovementHint(
        `Тип ${selectedSegmentIds.length === 1 ? "пути" : "путей"}: ${PATH_TYPE_LABELS[normalizedType]}.`
      );
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
      const schemeId = Number.parseInt(selectedLayoutId, 10);
      if (Number.isNaN(schemeId) || schemeId <= 0) {
        setMovementHint("Некорректный scheme_id для сценария.");
        return;
      }
      const schemeDetails = await getSchemeDetails(schemeId);
      const persistedScenarioSteps = remapScenarioStepsToPersistedTracks(
        scenarioSteps,
        schemeDetails?.tracks || []
      );

      const payload = buildNormalizedScenarioPayload(
        scenarioName,
        schemeId,
        persistedScenarioSteps,
        vehicles
      );

      let response;
      if (selectedScenarioId) {
        response = await updateNormalizedScenario(selectedScenarioId, payload);
      } else {
        response = await createNormalizedScenario(payload);
      }

      if (!response.ok || !response.scenario?.scenario_id) {
        throw new Error(response.message || "Не удалось сохранить сценарий.");
      }

      setSelectedScenarioId(String(response.scenario.scenario_id));
      setScenarioSteps(persistedScenarioSteps);
      await refreshSavedScenarios();
      setMovementHint("Сценарий сохранен.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось сохранить сценарий.");
    }
  }

  async function loadScenarioById(scenarioId) {
    try {
      const scenarioDetails = await getNormalizedScenarioDetails(scenarioId);
      const schemeId = scenarioDetails.scenario?.scheme_id;
      let sourceVehicles = vehicles;
      let initialSnapshot = cloneLayoutState({ segments, vehicles, couplings });
      if (schemeId) {
        const schemeDetails = await getSchemeDetails(schemeId);
        const mappedState = buildEditorStateFromSchemeDetails(schemeDetails);
        sourceVehicles = mappedState.vehicles;
        initialSnapshot = cloneLayoutState(mappedState);
        setSegments((prev) => mergeSegmentNames(prev, mappedState.segments));
        setVehicles((prev) =>
          mergeVehicleColors(prev, mappedState.vehicles, vehicleColorMemoryRef.current)
        );
        setCouplings(mappedState.couplings);
      }
      applyLoadedScenarioDetails(scenarioDetails, sourceVehicles, initialSnapshot);
      setMovementHint("Сценарий загружен.");
    } catch (error) {
      setMovementHint(error.message || "Не удалось загрузить сценарий.");
    }
  }

  function applyLoadedScenarioDetails(scenarioDetails, sourceVehicles, initialSnapshot) {
    const schemeId = scenarioDetails?.scenario?.scheme_id;
    stopMovement(true);
    scenarioStopRequestedRef.current = false;
    resetScenarioEditorState(scenarioStepType);
    setSelectedScenarioListStepId("");
    setScenarioName(scenarioDetails?.scenario?.name || "Сценарий");
    setScenarioLayoutId(schemeId == null || schemeId === "" ? "" : String(schemeId));
    const loadedSteps = remapScenarioStepsToCurrentSegments(
      buildScenarioStepsFromNormalizedScenario(scenarioDetails, sourceVehicles),
      initialSnapshot?.segments || segments
    );
    setScenarioSteps(loadedSteps);
    setCurrentScenarioStep(0);
    setScenarioStateHistory([]);
    setScenarioInitialState(initialSnapshot);
    setScenarioViewMode("start");
    setScenarioExecutingStep(null);
  }

  async function handleLoadScenario() {
    if (!selectedScenarioId) {
      setMovementHint("Выбери сценарий для загрузки.");
      return;
    }
    await loadScenarioById(selectedScenarioId);
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
    if (selectedScenarioStepId || scenarioStepType === SCENARIO_STEP_MOVE) {
      return;
    }
    const selectedCodes = selectedVehicleIds
      .map((id) => normalizeUnitCode(vehicleCodeById.get(id)))
      .filter(Boolean)
      .slice(0, 2);
    setScenarioLinkObject1Code(selectedCodes[0] || "");
    setScenarioLinkObject2Code(selectedCodes[1] || "");
  }, [selectedScenarioStepId, scenarioStepType, selectedVehicleIds, vehicleCodeById]);

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
    const payload = {
      gridSize: GRID_SIZE,
      segments,
      trackConnections: buildTrackConnectionsFromSegments(segments),
      vehicles: sourceVehicles,
      couplings: sourceCouplings,
      selectedLocomotiveId: locoId,
      targetPathId: targetPathIdValue,
      targetIndex: targetIndexValue,
    };

    const response = await planMovement(payload);

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
              currentVehicles = mergeVehicleColors(
                currentVehicles,
                resolved.vehicles,
                vehicleColorMemoryRef.current
              );
              setVehicles((prev) =>
                mergeVehicleColors(prev, currentVehicles, vehicleColorMemoryRef.current)
              );
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
        setVehicles((prev) =>
          mergeVehicleColors(prev, currentVehicles, vehicleColorMemoryRef.current)
        );
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
      await deleteNormalizedScenario(selectedScenarioId);
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

  async function handleGenerateHeuristicDraft() {
    try {
      const schemeId = Number.parseInt(String(selectedLayoutId || scenarioLayoutId || "").trim(), 10);
      const requiredTargetCount = Number.parseInt(String(heuristicRequiredTargetCount || "").trim(), 10);
      const targetColor = String(heuristicTargetColor || "").trim();
      const formationTrackId = String(heuristicFormationTrackId || "").trim();
      const draftName = String(heuristicDraftName || "").trim();

      if (Number.isNaN(schemeId) || schemeId <= 0) {
        throw new Error("Для эвристики выбери сохраненную схему.");
      }
      if (!targetColor) {
        throw new Error("Укажи целевой цвет вагонов.");
      }
      if (Number.isNaN(requiredTargetCount) || requiredTargetCount <= 0) {
        throw new Error("Укажи корректное required_target_count.");
      }

      setIsGeneratingHeuristicDraft(true);
      setHeuristicDraftError("");

      const response = await generateFullHeuristicScenario({
        scheme_id: schemeId,
        target_color: targetColor,
        required_target_count: requiredTargetCount,
        ...(formationTrackId ? { formation_track_id: formationTrackId } : {}),
        ...(draftName ? { name: draftName } : {}),
      });

      setHeuristicDraftResult(response);
      if (!response?.feasible) {
        setMovementHint("Эвристический сценарий не построен: задача невыполнима.");
        return;
      }
      if (!response?.scenario || !Array.isArray(response?.scenario_steps)) {
        throw new Error("Сервер не вернул готовый обычный сценарий.");
      }

      const currentSnapshot = cloneLayoutState({ segments, vehicles, couplings });
      applyLoadedScenarioDetails(
        {
          ok: true,
          scenario: response.scenario,
          scenario_steps: response.scenario_steps,
        },
        vehicles,
        currentSnapshot
      );
      if (response?.created_scenario_id) {
        setSelectedScenarioId(String(response.created_scenario_id));
      }
      await refreshSavedScenarios();
      setMovementHint("Эвристический сценарий сгенерирован и сразу открыт как обычный сценарий.");
    } catch (error) {
      setHeuristicDraftResult(null);
      setHeuristicDraftError(error.message || "Не удалось сгенерировать эвристический сценарий.");
      setMovementHint(error.message || "Не удалось сгенерировать эвристический сценарий.");
    } finally {
      setIsGeneratingHeuristicDraft(false);
    }
  }

  async function executeMovement(locoId) {
    const timeline = await buildMovementTimeline(locoId, targetPathId, targetPathIndex, vehicles);
    return playTimeline(timeline, "Движение запущено.");
  }

  function resetScenarioEditorState(nextStepType = SCENARIO_STEP_MOVE) {
    setSelectedScenarioStepId("");
    setScenarioStepType(normalizeScenarioStepType(nextStepType));
    setScenarioUnitCode("");
    setScenarioFromPathId("");
    setScenarioFromIndex("");
    setScenarioToPathId("");
    setScenarioToIndex("");
    setScenarioLinkObject1Code("");
    setScenarioLinkObject2Code("");
  }

  function resetScenarioSelectionState() {
    setSelectedScenarioListStepId("");
    setSelectedScenarioStepId("");
  }

  function resetScenarioPlaybackState() {
    setScenarioInitialState(null);
    setCurrentScenarioStep(0);
    setScenarioStateHistory([]);
    setScenarioViewMode("start");
    setScenarioExecutingStep(null);
  }

  function loadScenarioStepIntoEditor(step) {
    const normalizedStep = normalizeScenarioStep(step);
    setSelectedScenarioListStepId(normalizedStep.id);
    setSelectedScenarioStepId(normalizedStep.id);
    setScenarioStepType(normalizedStep.type);

    if (normalizedStep.type === SCENARIO_STEP_MOVE) {
      setScenarioUnitCode(normalizedStep.payload.unitCode || "");
      setScenarioFromPathId(normalizedStep.payload.fromPathId || "");
      setScenarioFromIndex(
        Number.isFinite(normalizedStep.payload.fromIndex) ? String(normalizedStep.payload.fromIndex) : ""
      );
      setScenarioToPathId(normalizedStep.payload.toPathId || "");
      setScenarioToIndex(
        Number.isFinite(normalizedStep.payload.toIndex) ? String(normalizedStep.payload.toIndex) : ""
      );
      setScenarioLinkObject1Code("");
      setScenarioLinkObject2Code("");
      return;
    }

    const [object1Code = "", object2Code = ""] = normalizedStep.payload.unitCodes || [];
    setScenarioUnitCode("");
    setScenarioFromPathId("");
    setScenarioFromIndex("");
    setScenarioToPathId("");
    setScenarioToIndex("");
    setScenarioLinkObject1Code(object1Code);
    setScenarioLinkObject2Code(object2Code);
  }

  function handleSelectScenarioStep(stepId) {
    if (selectedScenarioStepId && selectedScenarioStepId !== stepId) {
      resetScenarioEditorState(scenarioStepType);
    }
    setSelectedScenarioListStepId(stepId);
  }

  function handleEditScenarioStep(step) {
    loadScenarioStepIntoEditor(step);
    scenarioStepEditorRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function buildDraftScenarioStep(stepId = crypto.randomUUID()) {
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
        id: stepId,
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

    const object1Code = normalizeUnitCode(scenarioLinkObject1Code);
    const object2Code = normalizeUnitCode(scenarioLinkObject2Code);
    if (!object1Code || !object2Code) {
      setMovementHint("Для сцепки/расцепки укажи два объекта.");
      return null;
    }
    return normalizeScenarioStep({
      id: stepId,
      type,
      payload: {
        unitCodes: [object1Code, object2Code],
      },
    });
  }

  function addScenarioStep() {
    const step = buildDraftScenarioStep();
    if (!step) {
      return;
    }
    const anchorStepId = selectedScenarioListStepId || selectedScenarioStepId;
    setScenarioSteps((prev) => {
      if (!anchorStepId) {
        return [...prev, step];
      }
      const selectedIndex = prev.findIndex((item) => item.id === anchorStepId);
      if (selectedIndex < 0) {
        return [...prev, step];
      }
      return [...prev.slice(0, selectedIndex + 1), step, ...prev.slice(selectedIndex + 1)];
    });
    resetScenarioPlaybackState();
    setSelectedScenarioListStepId(step.id);
    setSelectedScenarioStepId(step.id);
    setMovementHint(anchorStepId ? "Шаг вставлен после выбранного." : "Шаг добавлен в конец сценария.");
  }

  function saveScenarioStepEdits() {
    if (!selectedScenarioStepId) {
      setMovementHint("Сначала выбери шаг для редактирования.");
      return;
    }
    const step = buildDraftScenarioStep(selectedScenarioStepId);
    if (!step) {
      return;
    }
    setScenarioSteps((prev) => prev.map((item) => (item.id === selectedScenarioStepId ? step : item)));
    resetScenarioPlaybackState();
    setMovementHint("Изменения шага сохранены.");
  }

  function removeScenarioStep(stepId) {
    setScenarioSteps((prev) => prev.filter((step) => step.id !== stepId));
    if (selectedScenarioListStepId === stepId) {
      setSelectedScenarioListStepId("");
    }
    if (selectedScenarioStepId === stepId) {
      resetScenarioEditorState(scenarioStepType);
    }
    resetScenarioPlaybackState();
  }

  function clearScenarioSteps() {
    setScenarioSteps([]);
    resetScenarioSelectionState();
    resetScenarioEditorState(scenarioStepType);
    resetScenarioPlaybackState();
  }

  function cloneLayoutState(state) {
    return JSON.parse(JSON.stringify(state));
  }

  function applyLayoutSnapshot(snapshot, skipResolvePasses = 2) {
    const safeSnapshot = cloneLayoutState(snapshot || { segments: [], vehicles: [], couplings: [] });
    skipAutoResolvePassesRef.current = skipResolvePasses;
    setSegments((prev) => mergeSegmentNames(prev, safeSnapshot.segments || []));
    setVehicles((prev) =>
      mergeVehicleColors(prev, safeSnapshot.vehicles || [], vehicleColorMemoryRef.current)
    );
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
    const schemeId = Number.parseInt(String(scenarioLayoutId || selectedLayoutId || ""), 10);
    if (Number.isNaN(schemeId) || schemeId <= 0) {
      throw new Error("Для сценария не выбрана стартовая схема.");
    }

    const response = await getSchemeDetails(schemeId);
    const snapshot = cloneLayoutState(buildEditorStateFromSchemeDetails(response));
    setSelectedLayoutId(String(schemeId));
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
      const expectedFromPathId = canonicalizeTrackIdForSegments(step.payload.fromPathId, segments);
      const expectedToPathId = canonicalizeTrackIdForSegments(step.payload.toPathId, segments);
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
        !(
          target.pathId === expectedFromPathId &&
          Number(target.pathIndex) === step.payload.fromIndex
        ) &&
        !arePathPositionsEquivalent(
          segments,
          sharedEndpointModel.endpointOverrides,
          target.pathId,
          Number(target.pathIndex),
          expectedFromPathId,
          step.payload.fromIndex
        )
      ) {
        throw new Error(
          `Локомотив ${codeMap.get(target.id) || target.id} сейчас в ${target.pathId}:${target.pathIndex}, а не в ${expectedFromPathId}:${step.payload.fromIndex}.`
        );
      }

      const timeline = await buildMovementTimeline(
        target.id,
        expectedToPathId,
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
            instantVehicles = mergeVehicleColors(
              instantVehicles,
              resolved.vehicles,
              vehicleColorMemoryRef.current
            );
          }
        } catch {
          // Keep instantVehicles as-is if resolve is temporarily unavailable.
        }

        return {
          vehicles: instantVehicles,
          couplings: sourceCouplings,
          timeline: [],
          lastLocomotiveId: target.id,
          lastTargetPathId: expectedToPathId,
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
        lastTargetPathId: expectedToPathId,
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

    return {
      vehicles: sourceVehicles,
      couplings: (response.state || {}).couplings || [],
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

    const steps = remapScenarioStepsToCurrentSegments(scenarioSteps, segments)
      .map((step) => normalizeScenarioStep(step))
      .filter(Boolean);
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

  async function showScenarioStepById(stepId) {
    if (isMoving) {
      stopMovement(false);
    }

    if (!stepId) {
      setMovementHint("Сначала выбери шаг в списке.");
      return;
    }

    if (selectedScenarioStepId && selectedScenarioStepId !== stepId) {
      resetScenarioEditorState(scenarioStepType);
    }
    setSelectedScenarioListStepId(stepId);

    const steps = remapScenarioStepsToCurrentSegments(scenarioSteps, segments)
      .map((step) => normalizeScenarioStep(step))
      .filter(Boolean);
    if (!steps.length) {
      setMovementHint("Сценарий не содержит шагов.");
      return;
    }

    const targetIndex = steps.findIndex((step) => step.id === stepId);
    if (targetIndex < 0) {
      setMovementHint("Выбранный шаг не найден.");
      return;
    }

    try {
      const cachedEntry = scenarioStateHistory[targetIndex];
      if (cachedEntry?.afterState) {
        applyLayoutSnapshot(cachedEntry.afterState);
        setCurrentScenarioStep(targetIndex + 1);
        setScenarioExecutingStep(null);
        setScenarioViewMode("step");
        setMovementHint(`Показано состояние после шага ${targetIndex + 1}/${steps.length}.`);
        return;
      }

      const layoutStartState = scenarioInitialState || (await loadScenarioStartLayoutState());
      const historyDraft = [];
      let workingVehicles = (layoutStartState.vehicles || []).map((vehicle) => ({ ...vehicle }));
      let workingCouplings = (layoutStartState.couplings || []).map((coupling) => ({ ...coupling }));

      for (let index = 0; index <= targetIndex; index += 1) {
        const step = steps[index];
        const beforeState = cloneLayoutState({
          segments: layoutStartState.segments || segments,
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

        const afterState = cloneLayoutState({
          segments: beforeState.segments,
          vehicles: result.vehicles || [],
          couplings: result.couplings || [],
        });

        historyDraft.push({
          beforeState,
          afterState,
          stepType: step.type,
          timeline: result.timeline || [],
        });

        workingVehicles = afterState.vehicles.map((vehicle) => ({ ...vehicle }));
        workingCouplings = afterState.couplings.map((coupling) => ({ ...coupling }));
      }

      setScenarioInitialState(layoutStartState);
      setScenarioStateHistory(historyDraft);
      setCurrentScenarioStep(targetIndex + 1);
      setScenarioExecutingStep(null);
      setScenarioViewMode("step");
      applyLayoutSnapshot(historyDraft[targetIndex].afterState);
      setMovementHint(`Показано состояние после шага ${targetIndex + 1}/${steps.length}.`);
    } catch (error) {
      setScenarioExecutingStep(null);
      setMovementHint(error.message || "Не удалось перейти к выбранному шагу.");
    }
  }

  async function handleShowSelectedScenarioStep() {
    if (!selectedScenarioListStepId) {
      setMovementHint("Сначала выбери шаг в списке.");
      return;
    }

    const steps = remapScenarioStepsToCurrentSegments(scenarioSteps, segments)
      .map((step) => normalizeScenarioStep(step))
      .filter(Boolean);
    if (!steps.length) {
      setMovementHint("Сценарий не содержит шагов.");
      return;
    }

    const targetIndex = steps.findIndex((step) => step.id === selectedScenarioListStepId);
    if (targetIndex < 0) {
      setMovementHint("Выбранный шаг не найден.");
      return;
    }

    if (targetIndex + 1 <= currentScenarioStep) {
      await showScenarioStepById(selectedScenarioListStepId);
      return;
    }

    if (isMoving) {
      return;
    }

    scenarioStopRequestedRef.current = false;
    setScenarioViewMode("play");

    let workingVehicles = vehicles.map((vehicle) => ({ ...vehicle }));
    let workingCouplings = couplings.map((coupling) => ({ ...coupling }));
    const historyDraft = [...scenarioStateHistory.slice(0, currentScenarioStep)];

    try {
      for (let index = currentScenarioStep; index <= targetIndex; index += 1) {
        if (scenarioStopRequestedRef.current) {
          setScenarioStateHistory(historyDraft.filter(Boolean));
          setScenarioExecutingStep(null);
          setScenarioViewMode("paused");
          setMovementHint("Переход к выбранному шагу остановлен.");
          return;
        }

        setScenarioExecutingStep(index);
        const step = steps[index];
        const beforeState = cloneLayoutState({
          segments,
          vehicles: workingVehicles,
          couplings: workingCouplings,
        });

        const result = await executeSingleScenarioStep(
          step,
          index,
          steps.length,
          beforeState.vehicles.map((vehicle) => ({ ...vehicle })),
          beforeState.couplings.map((coupling) => ({ ...coupling })),
          true
        );

        const afterState = cloneLayoutState({
          segments: beforeState.segments,
          vehicles: result.vehicles || [],
          couplings: result.couplings || [],
        });

        historyDraft[index] = {
          beforeState,
          afterState,
          stepType: step.type,
          timeline: result.timeline || [],
        };

        workingVehicles = afterState.vehicles.map((vehicle) => ({ ...vehicle }));
        workingCouplings = afterState.couplings.map((coupling) => ({ ...coupling }));
        setScenarioStateHistory(historyDraft.slice(0, index + 1));
        applyLayoutSnapshot(afterState);
        setCurrentScenarioStep(index + 1);
      }

      setScenarioExecutingStep(null);
      setScenarioViewMode(targetIndex + 1 >= steps.length ? "final" : "step");
      setMovementHint(`Показано состояние после шага ${targetIndex + 1}/${steps.length}.`);
    } catch (error) {
      setScenarioExecutingStep(null);
      setScenarioViewMode("paused");
      setMovementHint(error.message || "Не удалось перейти к выбранному шагу.");
    }
  }

  function stopScenarioPlayback() {
    scenarioStopRequestedRef.current = true;
    setMovementHint("Дальнейшее выполнение сценария будет остановлено после текущего шага.");
  }

  async function handleShowFullScenario() {
    await runSimpleScenario(true);
  }

  async function runSimpleScenario(restartFromStart = false) {
    if (isMoving) {
      return;
    }

    const fallbackStep = scenarioSteps.length ? null : buildDraftScenarioStep();
    const steps = remapScenarioStepsToCurrentSegments(
      scenarioSteps.length ? scenarioSteps : fallbackStep ? [fallbackStep] : [],
      segments
    )
      .map((step) => normalizeScenarioStep(step))
      .filter(Boolean);

    try {
      if (!steps.length) {
        setMovementHint("Добавь хотя бы один корректный шаг сценария.");
        return;
      }
      const startState = restartFromStart
        ? await loadScenarioStartLayoutState()
        : scenarioInitialState ||
          cloneLayoutState({
            segments,
            vehicles,
            couplings,
          });
      setScenarioInitialState(startState);
      scenarioStopRequestedRef.current = false;
      setScenarioViewMode("play");
      const startIndex = restartFromStart ? 0 : currentScenarioStep;
      setScenarioExecutingStep(startIndex);

      if (restartFromStart) {
        applyLayoutSnapshot(startState);
        setCurrentScenarioStep(0);
        setScenarioStateHistory([]);
      }

      let workingVehicles = (restartFromStart ? startState.vehicles : vehicles).map((vehicle) => ({ ...vehicle }));
      let workingCouplings = (restartFromStart ? startState.couplings : couplings).map((coupling) => ({ ...coupling }));
      let lastLocomotiveId = null;
      let lastTargetPathId = null;
      let lastTargetIndex = null;
      const historyDraft = restartFromStart
        ? []
        : [...scenarioStateHistory.slice(0, currentScenarioStep)];
      if (restartFromStart || currentScenarioStep === 0) {
        historyDraft.length = 0;
      }

      for (let index = startIndex; index < steps.length; index += 1) {
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
            !(
              target.pathId === step.payload.fromPathId &&
              Number(target.pathIndex) === step.payload.fromIndex
            ) &&
            !arePathPositionsEquivalent(
              segments,
              sharedEndpointModel.endpointOverrides,
              target.pathId,
              Number(target.pathIndex),
              step.payload.fromPathId,
              step.payload.fromIndex
            )
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
        skipAutoResolvePassesRef.current = 2;
        workingCouplings = (response.state || {}).couplings || [];
        setVehicles((prev) =>
          mergeVehicleColors(prev, workingVehicles, vehicleColorMemoryRef.current)
        );
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
      setVehicles((prev) =>
        mergeVehicleColors(prev, workingVehicles, vehicleColorMemoryRef.current)
      );
      skipAutoResolvePassesRef.current = Math.max(skipAutoResolvePassesRef.current, 1);
      setCouplings(workingCouplings);
      setScenarioExecutingStep(null);
      setCurrentScenarioStep(steps.length);
      setScenarioViewMode("final");
      setMovementHint("Сценарий выполнен.");
    } catch (error) {
      setScenarioExecutingStep(null);
      setScenarioViewMode("paused");
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
        setMovementHint(error.message || "Ошибка связи с сервером.");
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
    const affectedEndpoints = (node.endpoints || []).map((item) => ({
      segmentId: item.segmentId,
      endpoint: item.endpoint === "start" ? "from" : "to",
    }));
    const affectedIds = affectedEndpoints.map((item) => item.segmentId);

    setSelectedSegmentIds([...new Set(affectedIds)]);
    setDragState({ type: "node", affectedEndpoints });
    setSelectionBox(null);
  }

  function handleNodeClick(event, node) {
    if (isMoving) {
      return;
    }
    if (!isMoveMode) {
      return;
    }
    event.stopPropagation();
    const preferred = (node.endpoints || [])[0];
    if (!preferred) {
      return;
    }
    const segment = segments.find((item) => item.id === preferred.segmentId);
    if (!segment) {
      return;
    }
    const points = getSegmentSlotsWithEndpointOverrides(segment, sharedEndpointModel.endpointOverrides);
    if (!points.length) {
      return;
    }
    const targetIndex = preferred.endpoint === "start" ? 0 : points.length - 1;
    setTargetPathId(segment.id);
    setTargetPathIndex(targetIndex);
    setMovementHint(`Цель: ${formatPathReference(segment.id, targetIndex, segmentDisplayNameById)}`);
  }

  async function handleSlotClick(event, slot) {
    if (isMoving) {
      return;
    }

    if (isMoveMode) {
      event.stopPropagation();
      setTargetPathId(slot.pathId);
      setTargetPathIndex(slot.index);
      setMovementHint(`Цель: ${formatPathReference(slot.pathId, slot.index, segmentDisplayNameById)}`);
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
        color: type === "wagon" ? wagonPaintColor || DEFAULT_WAGON_COLOR : undefined,
        targetPathId: slot.pathId,
        targetIndex: slot.index,
      });
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с сервером.");
    }
  }

  function handleVehicleClick(event, vehicleId) {
    event.stopPropagation();

    if (isPaintMode) {
      const vehicle = vehicleById.get(vehicleId);
      if (!vehicle || vehicle.type !== "wagon") {
        return;
      }
      setVehicles((prev) =>
        prev.map((item) =>
          item.id === vehicleId ? { ...item, color: wagonPaintColor } : item
        )
      );
      setSelectedVehicleIds([vehicleId]);
      setMovementHint(`Вагон покрашен в ${wagonPaintColor}.`);
      return;
    }

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
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
      setMovementHint(error.message || "Ошибка связи с сервером.");
    }
  }

  async function clearLayout() {
    stopMovement(true);
    try {
      await applyLayoutAction("clear");
    } catch (error) {
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
      setMovementHint(error.message || "Ошибка связи с сервером.");
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
      if (!["drawTrack", "placeWagon", "placeLocomotive", "paintWagon", "edit"].includes(mode)) {
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

  const canvasGridSize = useMemo(() => computeCanvasGridSize(segments), [segments]);
  const majorCanvasGrid = canvasGridSize * 5;
  const minorVerticalGridLines = useMemo(
    () => buildGridLines(camera.x, camera.x + viewWidth, canvasGridSize),
    [camera.x, viewWidth, canvasGridSize]
  );
  const minorHorizontalGridLines = useMemo(
    () => buildGridLines(camera.y, camera.y + viewHeight, canvasGridSize),
    [camera.y, viewHeight, canvasGridSize]
  );
  const majorVerticalGridLines = useMemo(
    () => buildGridLines(camera.x, camera.x + viewWidth, majorCanvasGrid),
    [camera.x, viewWidth, majorCanvasGrid]
  );
  const majorHorizontalGridLines = useMemo(
    () => buildGridLines(camera.y, camera.y + viewHeight, majorCanvasGrid),
    [camera.y, viewHeight, majorCanvasGrid]
  );
  const selectionRect = selectionBox ? normalizeRect(selectionBox.start, selectionBox.end) : null;
  const activeModeLabel =
    mode === "drawTrack"
      ? "Добавление пути"
      : mode === "placeWagon"
        ? "Добавление вагонов"
        : mode === "placeLocomotive"
          ? "Добавление локомотивов"
          : mode === "paintWagon"
            ? "Покраска вагонов"
          : mode === "move"
            ? "Движение"
            : mode === "edit"
              ? "Редактирование"
              : "Просмотр";
  const scenarioStepDisplay = scenarioSteps.length
    ? Math.min(Math.max(currentScenarioStep, 0), scenarioSteps.length)
    : 0;
  const canvasColors = CANVAS_THEME_COLORS[canvasTheme] || CANVAS_THEME_COLORS[CANVAS_THEME_LIGHT];
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
      <aside className="sidebar" style={{ width: sidebarWidth }}>
        <div className="panelContent">
          {activePanel === "maneuvers" && (
            <div className="tools">
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Схемы</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsLayoutSchemesSectionCollapsed((value) => !value)}
                    aria-label={isLayoutSchemesSectionCollapsed ? "Развернуть блок схем" : "Свернуть блок схем"}
                    title={isLayoutSchemesSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isLayoutSchemesSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isLayoutSchemesSectionCollapsed && (
                  <div className="toolSectionBody">
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
                        <option key={layout.scheme_id} value={String(layout.scheme_id)}>
                          {layout.name || `Схема ${layout.scheme_id}`}
                        </option>
                      ))}
                    </select>
                    <button type="button" className="toolButton" onClick={handleLoadLayout}>
                      Загрузить схему
                    </button>
                    <button type="button" className="toolButton" onClick={handleSaveLayout}>
                      Сохранить схему
                    </button>
                    <button type="button" className="toolButton toolButtonDanger" onClick={handleDeleteLayout}>
                      Удалить схему
                    </button>
                  </div>
                )}
              </section>

              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Добавление</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsLayoutAddSectionCollapsed((value) => !value)}
                    aria-label={isLayoutAddSectionCollapsed ? "Развернуть блок добавления" : "Свернуть блок добавления"}
                    title={isLayoutAddSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isLayoutAddSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isLayoutAddSectionCollapsed && (
                  <div className="toolSectionBody">
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
                    <div className="paintModeBlock">
                      <button
                        type="button"
                        className={`toolButton ${mode === "paintWagon" ? "active" : ""}`}
                        onClick={() => switchMode("paintWagon")}
                      >
                        Покраска
                      </button>
                      <div className="colorPaletteRow">
                        {WAGON_COLOR_PALETTE.map((color) => (
                          <button
                            key={color}
                            type="button"
                            className={`colorSwatch ${wagonPaintColor === color ? "active" : ""}`}
                            onClick={() => setWagonPaintColor(color)}
                            title={color}
                            style={{ background: color }}
                          />
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </section>

              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Работа с объектами</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsLayoutObjectsSectionCollapsed((value) => !value)}
                    aria-label={isLayoutObjectsSectionCollapsed ? "Развернуть блок работы с объектами" : "Свернуть блок работы с объектами"}
                    title={isLayoutObjectsSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isLayoutObjectsSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isLayoutObjectsSectionCollapsed && (
                  <div className="toolSectionBody">
                    <button
                      type="button"
                      className={`toolButton ${mode === "edit" ? "active" : ""}`}
                      onClick={() => switchMode("edit")}
                    >
                      Редактирование
                    </button>
                    <select
                      className="toolInput"
                      value={selectedSegmentsType}
                      onChange={(event) => handleSelectedSegmentsTypeChange(event.target.value)}
                      disabled={selectedSegmentIds.length === 0}
                    >
                      <option value="">Тип выбранных путей</option>
                      {PATH_TYPE_OPTIONS.map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                    <button type="button" className="toolButton toolButtonDanger" onClick={deleteSelectedSegments}>
                      Удалить выбранные пути
                    </button>
                    <button type="button" className="toolButton toolButtonDanger" onClick={deleteSelectedVehicles}>
                      Удалить выбранные составы
                    </button>
                    <button type="button" className="toolButton toolButtonDanger" onClick={deleteSelectedAll}>
                      Удалить всё выбранное
                    </button>
                    <button type="button" className="toolButton toolButtonDanger" onClick={clearLayout}>
                      Очистить всё
                    </button>
                  </div>
                )}
              </section>
            </div>
          )}

          {activePanel === "coupling" && (
            <div className="tools">
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Сцепка</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsCouplingActionsSectionCollapsed((value) => !value)}
                    aria-label={isCouplingActionsSectionCollapsed ? "Развернуть блок сцепки" : "Свернуть блок сцепки"}
                    title={isCouplingActionsSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isCouplingActionsSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isCouplingActionsSectionCollapsed && (
                  <div className="toolSectionBody">
                    <button type="button" className="toolButton" onClick={coupleSelectedVehicles}>
                      Сцепить выбранные
                    </button>
                    <button type="button" className="toolButton" onClick={decoupleSelectedVehicles}>
                      Расцепить выбранные
                    </button>
                  </div>
                )}
              </section>
            </div>
          )}

          {activePanel === "movement" && (
            <div className="tools">
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Движение</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsMovementActionsSectionCollapsed((value) => !value)}
                    aria-label={isMovementActionsSectionCollapsed ? "Развернуть блок движения" : "Свернуть блок движения"}
                    title={isMovementActionsSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isMovementActionsSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isMovementActionsSectionCollapsed && (
                  <div className="toolSectionBody">
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
                    <button type="button" className="toolButton toolButtonDanger" onClick={() => stopMovement(false)}>
                      Стоп
                    </button>
                  </div>
                )}
              </section>
            </div>
          )}

          {activePanel === "scenario" && (
            <div className="tools">
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Сценарии</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsScenarioSectionCollapsed((value) => !value)}
                    aria-label={isScenarioSectionCollapsed ? "Развернуть раздел сценариев" : "Свернуть раздел сценариев"}
                    title={isScenarioSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isScenarioSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isScenarioSectionCollapsed && (
                  <div className="toolSectionBody">
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
                        <option key={scenario.scenario_id} value={String(scenario.scenario_id)}>
                          {scenario.name || `Сценарий ${scenario.scenario_id}`}
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
                  </div>
                )}
              </section>
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible" style={{ paddingBottom: 0 }}>
                  <span>Эвристический сценарий</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsHeuristicSectionCollapsed((value) => !value)}
                    aria-label={
                      isHeuristicSectionCollapsed
                        ? "Развернуть эвристический сценарий"
                        : "Свернуть эвристический сценарий"
                    }
                    title={isHeuristicSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isHeuristicSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isHeuristicSectionCollapsed && (
                  <div className="toolSectionBody">
                    <p className="counter">
                      Схема: {selectedLayoutId || scenarioLayoutId || "-"}
                    </p>
                    <div className="paintModeBlock">
                      <p className="counter">Целевой цвет вагонов</p>
                      <div className="colorPaletteRow">
                        {WAGON_COLOR_PALETTE.map((color) => (
                          <button
                            key={`heuristic-color-${color}`}
                            type="button"
                            className={`colorSwatch ${heuristicTargetColor === color ? "active" : ""}`}
                            onClick={() => setHeuristicTargetColor(color)}
                            title={color}
                            style={{ background: color }}
                          />
                        ))}
                      </div>
                    </div>
                    <input
                      className="toolInput"
                      value={heuristicDraftName}
                      onChange={(event) => setHeuristicDraftName(event.target.value)}
                      placeholder="Название эвристического сценария"
                    />
                    <input
                      className="toolInput"
                      value={heuristicRequiredTargetCount}
                      onChange={(event) => setHeuristicRequiredTargetCount(event.target.value)}
                      placeholder="Количество целевых вагонов"
                    />
                    <select
                      className="toolInput"
                      value={heuristicFormationTrackId}
                      onChange={(event) => setHeuristicFormationTrackId(event.target.value)}
                    >
                      <option value="">Путь формирования (авто)</option>
                      {leadPathOptions.map((path) => (
                        <option key={`heuristic-lead-${path.value}`} value={path.value}>
                          Путь {path.label}
                        </option>
                      ))}
                    </select>
                    <button
                      type="button"
                      className="toolButton"
                      onClick={handleGenerateHeuristicDraft}
                      disabled={isGeneratingHeuristicDraft}
                    >
                      {isGeneratingHeuristicDraft ? "Генерация..." : "Сгенерировать эвристический сценарий"}
                    </button>
                    <p className="counter">
                      Выполнимость:{" "}
                      {heuristicDraftResult == null
                        ? "-"
                        : heuristicDraftResult.feasible
                          ? "да"
                          : "нет"}
                    </p>
                    {heuristicDraftError ? <p className="counter">{heuristicDraftError}</p> : null}
                    {heuristicDraftResult?.reasons?.length ? (
                      <div className="scenarioSteps">
                        {heuristicDraftResult.reasons.map((reason, index) => (
                          <div key={`heuristic-reason-${index}`} className="scenarioStepRow">
                            <span className="scenarioStepText">{reason}</span>
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                )}
              </section>
              <section className="toolSection" ref={scenarioStepEditorRef}>
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Добавление / изменение шагов</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsScenarioStepEditorCollapsed((value) => !value)}
                    aria-label={
                      isScenarioStepEditorCollapsed
                        ? "Развернуть блок редактирования шагов"
                        : "Свернуть блок редактирования шагов"
                    }
                    title={isScenarioStepEditorCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isScenarioStepEditorCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isScenarioStepEditorCollapsed && <div className="toolSectionBody">
                  <div className="scenarioStepEditor">
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
                        <select
                          className="toolInput"
                          value={scenarioFromPathId}
                          onChange={(event) => setScenarioFromPathId(event.target.value)}
                        >
                          <option value="">Откуда путь</option>
                          {pathOptions.map((path) => (
                            <option key={`from-${path.value}`} value={path.value}>
                              Путь {path.label}
                            </option>
                          ))}
                        </select>
                        <input
                          className="toolInput"
                          value={scenarioFromIndex}
                          onChange={(event) => setScenarioFromIndex(event.target.value)}
                          placeholder="Откуда индекс"
                        />
                        <select
                          className="toolInput"
                          value={scenarioToPathId}
                          onChange={(event) => setScenarioToPathId(event.target.value)}
                        >
                          <option value="">Куда путь</option>
                          {pathOptions.map((path) => (
                            <option key={`to-${path.value}`} value={path.value}>
                              Путь {path.label}
                            </option>
                          ))}
                        </select>
                        <input
                          className="toolInput"
                          value={scenarioToIndex}
                          onChange={(event) => setScenarioToIndex(event.target.value)}
                          placeholder="Куда индекс"
                        />
                      </>
                    ) : (
                      <>
                        <input
                          className="toolInput"
                          value={scenarioLinkObject1Code}
                          onChange={(event) => setScenarioLinkObject1Code(event.target.value)}
                          placeholder="Объект 1 (л1, в1)"
                        />
                        <input
                          className="toolInput"
                          value={scenarioLinkObject2Code}
                          onChange={(event) => setScenarioLinkObject2Code(event.target.value)}
                          placeholder="Объект 2 (л1, в1)"
                        />
                        <p className="counter">Можно ввести коды вручную или сначала выбрать 2 объекта на схеме.</p>
                      </>
                    )}
                  </div>
                  <button type="button" className="toolButton" onClick={addScenarioStep}>
                    Добавить шаг
                  </button>
                  <button
                    type="button"
                    className="toolButton"
                    onClick={saveScenarioStepEdits}
                    disabled={!selectedScenarioStepId}
                  >
                    Сохранить изменения шага
                  </button>
                  <button
                    type="button"
                    className="toolButton"
                    onClick={() => resetScenarioEditorState(scenarioStepType)}
                  >
                    {selectedScenarioStepId ? "Отменить изменение шага" : "Очистить форму шага"}
                  </button>
                  <button
                    type="button"
                    className="toolButton toolButtonDanger"
                    onClick={clearScenarioSteps}
                  >
                    Удалить все шаги
                  </button>
                </div>}
              </section>
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Просмотр сценария</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsScenarioPlaybackSectionCollapsed((value) => !value)}
                    aria-label={
                      isScenarioPlaybackSectionCollapsed
                        ? "Развернуть блок просмотра сценария"
                        : "Свернуть блок просмотра сценария"
                    }
                    title={isScenarioPlaybackSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isScenarioPlaybackSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isScenarioPlaybackSectionCollapsed && (
                  <div className="toolSectionBody">
                    <button type="button" className="toolButton" onClick={handleShowFullScenario}>
                      Показать сценарий полностью
                    </button>
                    <button
                      type="button"
                      className="toolButton"
                      onClick={handleShowSelectedScenarioStep}
                      disabled={!selectedScenarioListStepId}
                    >
                      Перейти к выбранному шагу
                    </button>
                    <button type="button" className="toolButton" onClick={handleShowScenarioStart}>
                      Показать старт
                    </button>
                    <button type="button" className="toolButton" onClick={handleShowScenarioFinal}>
                      Показать финал
                    </button>
                  </div>
                )}
              </section>

              <section className="toolSection scenarioPinnedSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Пошаговый просмотр</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsScenarioStepNavigationCollapsed((value) => !value)}
                    aria-label={
                      isScenarioStepNavigationCollapsed
                        ? "Развернуть блок пошагового просмотра"
                        : "Свернуть блок пошагового просмотра"
                    }
                    title={isScenarioStepNavigationCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isScenarioStepNavigationCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isScenarioStepNavigationCollapsed && (
                  <div className="toolSectionBody">
                    <button type="button" className="toolButton" onClick={handlePrevScenarioStep}>
                      Предыдущий шаг
                    </button>
                    <button type="button" className="toolButton" onClick={handleNextScenarioStep}>
                      Следующий шаг
                    </button>
                    <button type="button" className="toolButton toolButtonDanger" onClick={stopScenarioPlayback}>
                      Стоп
                    </button>
                  </div>
                )}
              </section>
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Шаги сценария</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsScenarioStepsSectionCollapsed((value) => !value)}
                    aria-label={
                      isScenarioStepsSectionCollapsed
                        ? "Развернуть шаги сценария"
                        : "Свернуть шаги сценария"
                    }
                    title={isScenarioStepsSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isScenarioStepsSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isScenarioStepsSectionCollapsed && <div className="toolSectionBody">
                  <p className="counter">
                    Текущий шаг:{" "}
                    {scenarioViewMode === "play" && scenarioExecutingStep != null
                      ? `${scenarioExecutingStep + 1}/${scenarioSteps.length}`
                      : `${scenarioStepDisplay}/${scenarioSteps.length}`}
                  </p>
                  {scenarioSteps.length > 0 && (
                    <div className="scenarioSteps">
                      <div
                        className={`scenarioStepRow ${isScenarioStartHighlighted ? "current" : ""}`}
                        onDoubleClick={handleShowScenarioStart}
                        style={{ cursor: "pointer" }}
                      >
                        <span className="scenarioStepText">Старт</span>
                      </div>
                      {scenarioSteps.map((step, index) => (
                        <div
                          key={step.id}
                          className={`scenarioStepRow ${index === scenarioActiveStepIndex ? "current" : ""} ${step.id === selectedScenarioListStepId ? "selected" : ""}`}
                          onClick={() => handleSelectScenarioStep(step.id)}
                          onDoubleClick={() => showScenarioStepById(step.id)}
                          style={{ cursor: "pointer" }}
                        >
                          <span className="scenarioStepText">
                            {formatScenarioStepText(
                              normalizeScenarioStep(step),
                              index,
                              segmentDisplayNameById
                            )}
                          </span>
                          <div className="scenarioStepActions">
                            <button
                              type="button"
                              className="scenarioStepEdit"
                              onClick={(event) => {
                                event.stopPropagation();
                                handleEditScenarioStep(step);
                              }}
                              title="Редактировать шаг"
                            >
                              ✎
                            </button>
                            <button
                              type="button"
                              className="scenarioStepRemove"
                              onClick={(event) => {
                                event.stopPropagation();
                                removeScenarioStep(step.id);
                              }}
                              title="Удалить шаг"
                            >
                              ×
                            </button>
                          </div>
                        </div>
                      ))}
                      <div
                        className={`scenarioStepRow ${isScenarioFinalHighlighted ? "current" : ""}`}
                        onDoubleClick={handleShowScenarioFinal}
                        style={{ cursor: "pointer" }}
                      >
                        <span className="scenarioStepText">Финал</span>
                      </div>
                    </div>
                  )}
                </div>}
              </section>
            </div>
          )}

          {activePanel === "metrics" && (
            <div className="tools">
              <section className="toolSection">
                <div className="toolSectionHeader toolSectionHeaderCollapsible">
                  <span>Подсчёт сценария</span>
                  <button
                    type="button"
                    className="toolSectionToggle"
                    onClick={() => setIsMetricsSectionCollapsed((value) => !value)}
                    aria-label={isMetricsSectionCollapsed ? "Развернуть блок подсчёта" : "Свернуть блок подсчёта"}
                    title={isMetricsSectionCollapsed ? "Развернуть" : "Свернуть"}
                  >
                    {isMetricsSectionCollapsed ? "▾" : "▴"}
                  </button>
                </div>
                {!isMetricsSectionCollapsed && (
                  <div className="toolSectionBody">
                    <select
                      className="toolInput"
                      value={selectedMetricsScenarioId}
                      onChange={(event) => setSelectedMetricsScenarioId(event.target.value)}
                    >
                      <option value="">Выбор сценария</option>
                      {savedScenarios.map((scenario) => (
                        <option key={`metrics-${scenario.scenario_id}`} value={String(scenario.scenario_id)}>
                          {scenario.name || `Сценарий ${scenario.scenario_id}`}
                        </option>
                      ))}
                    </select>

                    {isScenarioMetricsLoading ? (
                      <p className="counter">Загрузка метрик...</p>
                    ) : scenarioMetricsError ? (
                      <p className="counter">{scenarioMetricsError}</p>
                    ) : scenarioMetrics ? (
                      <>
                        <p className="counter">Пройдено звеньев: {scenarioMetrics.total_loco_distance}</p>
                        <p className="counter">Сцепок: {scenarioMetrics.total_couples}</p>
                        <p className="counter">Расцепок: {scenarioMetrics.total_decouples}</p>
                      </>
                    ) : (
                      <p className="counter">Выбери сценарий.</p>
                    )}
                  </div>
                )}
              </section>
            </div>
          )}
        </div>

        <p className="counter">{movementHint || "-"}</p>
      </aside>

      <div
        className="sidebarResizeHandle"
        onMouseDown={handleSidebarResizeStart}
        role="separator"
        aria-orientation="vertical"
        aria-label="Изменить ширину панели инструментов"
      />

      <main className={`workspace workspaceTheme-${canvasTheme}`}>
        <header className={`toolbar toolbarTheme-${canvasTheme}`}>
          <div className="toolbarActions">
            <button
              type="button"
              className={`zoomButton zoomButtonTheme ${canvasTheme === CANVAS_THEME_DARK ? "active" : ""}`}
              onClick={() =>
                setCanvasTheme((prev) =>
                  prev === CANVAS_THEME_DARK ? CANVAS_THEME_LIGHT : CANVAS_THEME_DARK
                )
              }
            >
              {canvasTheme === CANVAS_THEME_DARK ? "Светлый холст" : "Тёмный холст"}
            </button>
            <div className="zoomControls">
              <button type="button" className="zoomButton" onClick={zoomOut}>
                -
              </button>
              <button type="button" className="zoomButton zoomButtonValue" onClick={resetZoom}>
                {Math.round(zoom * 100)}%
              </button>
              <button type="button" className="zoomButton" onClick={zoomIn}>
                +
              </button>
            </div>
          </div>
        </header>

        <section
          ref={canvasWrapRef}
          className={`canvasWrap canvasTheme-${canvasTheme} ${isPanning ? "panning" : ""}`}
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
            <rect x={camera.x} y={camera.y} width={viewWidth} height={viewHeight} fill={canvasColors.background} />
            <g aria-hidden="true">
              {minorVerticalGridLines.map((x) => (
                <line
                  key={`grid-minor-v-${x}`}
                  x1={x}
                  y1={camera.y}
                  x2={x}
                  y2={camera.y + viewHeight}
                  stroke={canvasColors.gridMinor}
                  strokeWidth="1"
                  vectorEffect="non-scaling-stroke"
                  shapeRendering="crispEdges"
                />
              ))}
              {minorHorizontalGridLines.map((y) => (
                <line
                  key={`grid-minor-h-${y}`}
                  x1={camera.x}
                  y1={y}
                  x2={camera.x + viewWidth}
                  y2={y}
                  stroke={canvasColors.gridMinor}
                  strokeWidth="1"
                  vectorEffect="non-scaling-stroke"
                  shapeRendering="crispEdges"
                />
              ))}
              {majorVerticalGridLines.map((x) => (
                <line
                  key={`grid-major-v-${x}`}
                  x1={x}
                  y1={camera.y}
                  x2={x}
                  y2={camera.y + viewHeight}
                  stroke={canvasColors.gridMajor}
                  strokeWidth="1.2"
                  vectorEffect="non-scaling-stroke"
                  shapeRendering="crispEdges"
                />
              ))}
              {majorHorizontalGridLines.map((y) => (
                <line
                  key={`grid-major-h-${y}`}
                  x1={camera.x}
                  y1={y}
                  x2={camera.x + viewWidth}
                  y2={y}
                  stroke={canvasColors.gridMajor}
                  strokeWidth="1.2"
                  vectorEffect="non-scaling-stroke"
                  shapeRendering="crispEdges"
                />
              ))}
            </g>

            {segments.map((segment) => {
              const pathName = getPathDisplayName(segment.id, segmentDisplayNameById);
              const midpointX = (segment.from.x + segment.to.x) / 2;
              const midpointY = (segment.from.y + segment.to.y) / 2 - 24;
              return (
                <g key={segment.id}>
                  <line
                    x1={segment.from.x}
                    y1={segment.from.y}
                    x2={segment.to.x}
                    y2={segment.to.y}
                    stroke={getCanvasSegmentStrokeColor(segment.type, selectedSegmentSet.has(segment.id), canvasTheme)}
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
                    <title>
                      Путь {pathName} ({PATH_TYPE_LABELS[normalizePathType(segment.type)]})
                    </title>
                  </line>
                  <text
                    x={midpointX}
                    y={midpointY}
                    fill={canvasColors.pathLabel}
                    fontSize="14"
                    fontWeight="700"
                    textAnchor="middle"
                    pointerEvents="none"
                    style={{ userSelect: "none" }}
                  >
                    {pathName}
                  </text>
                </g>
              );
            })}

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
                      ? canvasColors.occupiedSlot
                      : canvasColors.emptySlot
                }
                className="slotPoint"
                onClick={(event) => handleSlotClick(event, slot)}
              >
                <title>
                  Путь {getPathDisplayName(slot.pathId, segmentDisplayNameById)}, звено: {slot.index}
                </title>
             </circle>
            ))}

            {vehicles.map((vehicle) => {
              const vehicleLabel = vehicleCodeById.get(vehicle.id) || vehicle.id;
              return (
                <g key={vehicle.id}>
                  <rect
                    x={vehicle.x - GRID_SIZE / 2 + 6}
                    y={vehicle.y - GRID_SIZE / 2 + 6}
                    width={GRID_SIZE - 12}
                    height={GRID_SIZE - 12}
                    rx="8"
                    fill={vehicle.type === "locomotive" ? "#dc2626" : vehicle.color || DEFAULT_WAGON_COLOR}
                    stroke={selectedVehicleSet.has(vehicle.id) ? "#facc15" : vehicle.type === "locomotive" ? "#7f1d1d" : "#0c4a6e"}
                    strokeWidth={selectedVehicleSet.has(vehicle.id) ? "4" : "2"}
                    className={isEditMode ? "slotPoint" : ""}
                    onMouseDown={(event) => startVehicleDrag(event, vehicle.id)}
                    onClick={(event) => handleVehicleClick(event, vehicle.id)}
                  >
                    <title>{vehicleLabel}</title>
                  </rect>
                  <text
                    x={vehicle.x}
                    y={vehicle.y + 4}
                    fill="#ffffff"
                    fontSize="12"
                    fontWeight="700"
                    textAnchor="middle"
                    pointerEvents="none"
                    style={{ userSelect: "none" }}
                    stroke="rgba(15,23,42,0.45)"
                    strokeWidth="0.6"
                    paintOrder="stroke"
                  >
                    {vehicleLabel}
                  </text>
                </g>
              );
            })}

            {nodes.map((node) => {
              const nodeKey = `${keyOf(node.x, node.y)}:${(node.endpoints || []).map((item) => `${item.segmentId}:${item.endpoint}`).join("|")}`;

              if (isEditMode) {
                return (
                  <circle
                    key={nodeKey}
                    cx={node.x}
                    cy={node.y}
                    r="7"
                    fill={canvasTheme === CANVAS_THEME_DARK ? "#93c5fd" : "#60a5fa"}
                    stroke={canvasTheme === CANVAS_THEME_DARK ? "#dbeafe" : "#1e3a8a"}
                    strokeWidth="2"
                    className="draggablePoint"
                    onMouseDown={(event) => startNodeDrag(event, node)}
                    onClick={(event) => handleNodeClick(event, node)}
                  />
                );
              }

              return (
                <g key={nodeKey}>
                  <circle
                    cx={node.x}
                    cy={node.y}
                    r="8"
                    fill="transparent"
                    className="slotPoint"
                    onClick={(event) => handleNodeClick(event, node)}
                  />
                  <circle
                    cx={node.x}
                    cy={node.y}
                    r="4.5"
                    fill={canvasColors.node}
                    pointerEvents="none"
                  />
                </g>
              );
            })}

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
                fill={canvasColors.selectionFill}
                stroke={canvasColors.selectionStroke}
              />
            )}
          </svg>
        </section>

        <footer className={`statusbar statusbarTheme-${canvasTheme}`}>
          X: {mousePoint.x} | Y: {mousePoint.y} | Zoom: {Math.round(zoom * 100)}% | Grid: {roundGridValue(canvasGridSize)}
        </footer>
      </main>
    </div>
  );
}

