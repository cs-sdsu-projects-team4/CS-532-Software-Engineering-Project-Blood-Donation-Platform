/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL for the Go backend REST API (e.g. "/api"). */
  readonly VITE_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
