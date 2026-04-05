const AUTH_KEY = "skillroom.auth";
const PREVIEW_KEY = "skillroom.preview";
const REGION_KEY = "skillroom.region";

export function loadAuth() {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(AUTH_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

export function saveAuth(auth) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(AUTH_KEY, JSON.stringify(auth));
}

export function clearAuth() {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(AUTH_KEY);
}

export function loadPreview() {
  if (typeof window === "undefined") {
    return false;
  }
  return window.localStorage.getItem(PREVIEW_KEY) === "true";
}

export function setPreview(enabled) {
  if (typeof window === "undefined") {
    return;
  }
  if (enabled) {
    window.localStorage.setItem(PREVIEW_KEY, "true");
    return;
  }
  window.localStorage.removeItem(PREVIEW_KEY);
}

export function loadRegionId() {
  if (typeof window === "undefined") {
    return "americas";
  }
  return window.localStorage.getItem(REGION_KEY) || "americas";
}

export function saveRegionId(regionId) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(REGION_KEY, regionId);
}

export async function apiFetch(path, options = {}) {
  const { token, body, headers, method = "GET", cache = "no-store" } = options;
  const requestHeaders = new Headers(headers || {});
  if (body !== undefined) {
    requestHeaders.set("Content-Type", "application/json");
  }
  if (token) {
    requestHeaders.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`/backend${path}`, {
    method,
    cache,
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();

  if (!response.ok) {
    if (typeof payload === "string") {
      throw new Error(payload || "request failed");
    }
    throw new Error(payload.error || "request failed");
  }

  return payload;
}
