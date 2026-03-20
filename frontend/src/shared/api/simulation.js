const API_BASE_URL = "http://localhost:8080/api";

function getAuthHeaders() {
  const token = localStorage.getItem("authToken");
  const headers = {
    "Content-Type": "application/json",
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  return headers;
}

async function postJSON(path, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    headers: getAuthHeaders(),
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw await buildHTTPError(response, path);
  }

  return response.json();
}

async function putJSON(path, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "PUT",
    headers: getAuthHeaders(),
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw await buildHTTPError(response, path);
  }

  return response.json();
}

async function deleteJSON(path) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "DELETE",
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await buildHTTPError(response, path);
  }

  return response.json();
}

async function getJSON(path) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "GET",
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await buildHTTPError(response, path);
  }

  return response.json();
}

async function buildHTTPError(response, path) {
  let message = `Ошибка HTTP ${response.status} (${path})`;

  try {
    const contentType = response.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      const data = await response.json();
      const detailed =
        data?.message ||
        data?.error ||
        (typeof data === "string" ? data : "");
      if (detailed) {
        message = detailed;
      }
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch {
    // Keep fallback HTTP status message.
  }

  return new Error(message);
}

export async function validateCoupling(payload) {
  return postJSON("/couplings/validate", payload);
}

export async function planMovement(payload) {
  return postJSON("/movement/plan", payload);
}

export async function placeVehicle(payload) {
  return postJSON("/vehicles/place", payload);
}

export async function resolveVehicles(payload) {
  return postJSON("/vehicles/resolve", payload);
}

export async function applyLayoutOperation(payload) {
  return postJSON("/layout/apply", payload);
}

export async function getCurrentUser() {
  return getJSON("/auth/me");
}

export async function createScheme(payload) {
  return postJSON("/normalized/schemes", payload);
}

export async function listSchemes() {
  return getJSON("/normalized/schemes");
}

export async function getSchemeDetails(schemeId) {
  return getJSON(`/normalized/schemes/${schemeId}/details`);
}

export async function updateScheme(schemeId, payload) {
  return putJSON(`/normalized/schemes/${schemeId}`, payload);
}

export async function deleteScheme(schemeId) {
  return deleteJSON(`/normalized/schemes/${schemeId}`);
}

export async function createNormalizedScenario(payload) {
  return postJSON("/normalized/scenarios", payload);
}

export async function listNormalizedScenarios() {
  return getJSON("/normalized/scenarios");
}

export async function getNormalizedScenarioDetails(scenarioId) {
  return getJSON(`/normalized/scenarios/${scenarioId}/details`);
}

export async function updateNormalizedScenario(scenarioId, payload) {
  return putJSON(`/normalized/scenarios/${scenarioId}`, payload);
}

export async function deleteNormalizedScenario(scenarioId) {
  return deleteJSON(`/normalized/scenarios/${scenarioId}`);
}

export async function runScenario(scenarioId) {
  return postJSON(`/normalized/scenarios/${scenarioId}/run`, {});
}

export async function generateDraftHeuristicScenario(payload) {
  return postJSON("/normalized/heuristic/generate", payload);
}

export async function generateAndSaveDraftHeuristicScenario(payload) {
  return postJSON("/normalized/heuristic/generate-and-save", payload);
}

export async function generateFullHeuristicScenario(payload) {
  return postJSON("/normalized/heuristic/generate-full-scenario", payload);
}

export async function listHeuristicScenarios() {
  return getJSON("/normalized/heuristic/scenarios");
}

export async function getHeuristicScenarioDetails(heuristicScenarioId) {
  return getJSON(`/normalized/heuristic/scenarios/${heuristicScenarioId}`);
}

export async function saveHeuristicDraftAsScenario(payload) {
  return postJSON("/normalized/heuristic/save-as-scenario", payload);
}

export async function getExecution(executionId) {
  return getJSON(`/executions/${executionId}`);
}

export async function stepExecution(executionId) {
  return postJSON(`/executions/${executionId}/step`, {});
}

