// Centralised, typed access to build-time configuration. Vite exposes only
// variables prefixed with VITE_ to the client bundle.

/** Base URL for the backend REST API. Defaults to the proxied "/api" path. */
export const API_BASE_URL: string =
  import.meta.env.VITE_API_BASE_URL?.replace(/\/+$/, "") || "/api";
