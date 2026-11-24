import { API_BASE_URL } from "./env";

/** Structured error envelope returned by the Go backend: {error:{code,message}}. */
interface ApiErrorBody {
  error?: {
    code?: string;
    message?: string;
  };
}

/** Error thrown for any non-2xx API response. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** Returns the current bearer token, or null when unauthenticated. */
type TokenProvider = () => string | null;

// The auth layer (WEB-PR2) registers a real provider; until then no token is
// attached. Keeping it injectable avoids a hard dependency on storage here.
let getToken: TokenProvider = () => null;

export function setAuthTokenProvider(provider: TokenProvider): void {
  getToken = provider;
}

/**
 * Attempts to refresh the access token. Resolves true if a new token is now
 * available (so the request should be retried), false otherwise.
 */
type TokenRefresher = () => Promise<boolean>;

// Registered by the auth layer (WEB-PR2). Null means no refresh is possible, so
// a 401 propagates to the caller unchanged.
let refreshToken: TokenRefresher | null = null;

export function setTokenRefresher(refresher: TokenRefresher | null): void {
  refreshToken = refresher;
}

export interface RequestOptions extends Omit<RequestInit, "body"> {
  /** JSON-serialisable request body; sets Content-Type automatically. */
  body?: unknown;
  /** Attach the bearer token when available. Defaults to true. */
  auth?: boolean;
}

/**
 * Typed fetch wrapper around the backend API. Resolves with the parsed JSON
 * body (or undefined for 204), and throws ApiError on any non-2xx response.
 */
export async function apiFetch<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { body, auth = true, headers, ...rest } = options;

  let requestBody: BodyInit | undefined;
  if (body !== undefined) {
    requestBody = JSON.stringify(body);
  }

  // Build the headers fresh on each send so a retry picks up a refreshed token.
  const send = (): Promise<Response> => {
    const requestHeaders = new Headers(headers);
    requestHeaders.set("Accept", "application/json");
    if (body !== undefined) {
      requestHeaders.set("Content-Type", "application/json");
    }
    if (auth) {
      const token = getToken();
      if (token) {
        requestHeaders.set("Authorization", `Bearer ${token}`);
      }
    }
    return fetch(`${API_BASE_URL}${path}`, {
      ...rest,
      headers: requestHeaders,
      body: requestBody,
    });
  };

  let response = await send();

  // On an expired/invalid access token, try a single refresh-and-retry so the
  // caller never sees a transient 401. auth===false requests (login/refresh)
  // skip this to avoid recursion.
  if (response.status === 401 && auth && refreshToken) {
    const refreshed = await refreshToken();
    if (refreshed) {
      response = await send();
    }
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const data = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    const errBody = data as ApiErrorBody | undefined;
    throw new ApiError(
      response.status,
      errBody?.error?.code ?? "unknown",
      errBody?.error?.message ?? response.statusText,
    );
  }

  return data as T;
}
