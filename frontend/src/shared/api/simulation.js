const API_BASE_URL = "http://localhost:8080/api";

async function postJSON(path, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
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
