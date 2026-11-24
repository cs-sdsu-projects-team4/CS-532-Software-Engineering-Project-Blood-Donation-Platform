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

  const requestHeaders = new Headers(headers);
  requestHeaders.set("Accept", "application/json");

  if (auth) {
    const token = getToken();
    if (token) {
      requestHeaders.set("Authorization", `Bearer ${token}`);
    }
  }

  let requestBody: BodyInit | undefined;
  if (body !== undefined) {
    requestHeaders.set("Content-Type", "application/json");
    requestBody = JSON.stringify(body);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...rest,
    headers: requestHeaders,
    body: requestBody,
  });

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
