const REGION_KEY = "skillroom.region";
const AUTH_UPDATED_EVENT = "skillroom:auth-updated";
const AUTH_CLEARED_EVENT = "skillroom:auth-cleared";

let authState = null;
let refreshPromise = null;

export function loadAuth() {
  return authState;
}

export function saveAuth(auth) {
  if (!auth) {
    authState = null;
  } else {
    const nextAuth = { ...auth };
    delete nextAuth.refresh_token;
    authState = nextAuth;
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(AUTH_UPDATED_EVENT, { detail: authState }));
  }
}

export function clearAuth() {
  authState = null;
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(AUTH_CLEARED_EVENT));
  }
}

export async function signOut() {
  try {
    await performRequest("/v1/auth/logout", {
      method: "POST",
      allowAuth: false,
    });
  } catch {
    // Best-effort logout. The local auth state still needs to be cleared.
  } finally {
    clearAuth();
  }
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

export async function restoreAuth() {
  if (loadAuth()) {
    return loadAuth();
  }
  try {
    return await refreshStoredAuth();
  } catch {
    return null;
  }
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

  if (response.status === 401 && allowRefresh && shouldRefresh(path)) {
    const nextAuth = await refreshStoredAuth();
    response = await performRequest(path, {
      token: nextAuth?.access_token || token,
      body,
      headers,
      method,
      cache,
    });
  }

  return unwrapResponse(response);
}

async function refreshStoredAuth() {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const response = await performRequest("/v1/auth/refresh", {
        method: "POST",
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

function shouldRefresh(path) {
  return ![
    "/v1/auth/login",
    "/v1/auth/register",
    "/v1/auth/refresh",
    "/v1/auth/logout",
  ].includes(path);
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
    credentials: "same-origin",
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
