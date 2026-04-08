const AUTH_KEY = "skillroom.auth";
const REGION_KEY = "skillroom.region";
const AUTH_UPDATED_EVENT = "skillroom:auth-updated";
const AUTH_CLEARED_EVENT = "skillroom:auth-cleared";

let refreshPromise = null;

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
  window.dispatchEvent(new CustomEvent(AUTH_UPDATED_EVENT, { detail: auth }));
}

export function clearAuth() {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(AUTH_KEY);
  window.dispatchEvent(new Event(AUTH_CLEARED_EVENT));
}

export function subscribeAuth(onUpdated, onCleared) {
  if (typeof window === "undefined") {
    return () => {};
  }

  function handleUpdated(event) {
    onUpdated?.(event.detail || loadAuth());
  }

  function handleCleared() {
    onCleared?.();
  }

  window.addEventListener(AUTH_UPDATED_EVENT, handleUpdated);
  window.addEventListener(AUTH_CLEARED_EVENT, handleCleared);

  return () => {
    window.removeEventListener(AUTH_UPDATED_EVENT, handleUpdated);
    window.removeEventListener(AUTH_CLEARED_EVENT, handleCleared);
  };
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

export function isUnauthorizedError(error) {
  return Boolean(error && typeof error === "object" && error.status === 401);
}

export async function apiFetch(path, options = {}) {
  const { token, body, headers, method = "GET", cache = "no-store", allowRefresh = true } = options;
  const currentAuth = loadAuth();
  let response = await performRequest(path, {
    token: currentAuth?.access_token || token,
    body,
    headers,
    method,
    cache,
  });

  if (response.status === 401 && allowRefresh && shouldRefresh(path, currentAuth)) {
    const nextAuth = await refreshStoredAuth(currentAuth);
    response = await performRequest(path, {
      token: nextAuth.access_token,
      body,
      headers,
      method,
      cache,
    });
  }

  return unwrapResponse(response);
}

async function refreshStoredAuth(currentAuth) {
  if (!currentAuth?.refresh_token) {
    clearAuth();
    throw createAPIError("session expired", 401);
  }
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const response = await performRequest("/v1/auth/refresh", {
        method: "POST",
        body: { refresh_token: currentAuth.refresh_token },
        allowAuth: false,
      });
      const payload = await unwrapResponse(response);
      saveAuth(payload);
      return payload;
    })().catch((error) => {
      clearAuth();
      throw error;
    }).finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

function shouldRefresh(path, currentAuth) {
  return Boolean(
    currentAuth?.refresh_token &&
      path !== "/v1/auth/login" &&
      path !== "/v1/auth/register" &&
      path !== "/v1/auth/refresh",
  );
}

async function performRequest(path, options = {}) {
  const { token, body, headers, method = "GET", cache = "no-store", allowAuth = true } = options;
  const requestHeaders = new Headers(headers || {});
  if (body !== undefined) {
    requestHeaders.set("Content-Type", "application/json");
  }
  if (allowAuth && token) {
    requestHeaders.set("Authorization", `Bearer ${token}`);
  }

  return fetch(`/backend${path}`, {
    method,
    cache,
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

async function unwrapResponse(response) {
  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();

  if (!response.ok) {
    if (typeof payload === "string") {
      throw createAPIError(payload || "request failed", response.status, payload);
    }
    throw createAPIError(payload.error || "request failed", response.status, payload);
  }

  return payload;
}

function createAPIError(message, status, payload) {
  const error = new Error(message);
  error.status = status;
  error.payload = payload;
  return error;
}
