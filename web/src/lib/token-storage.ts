import type { TokenPair } from "../types/auth";

// Persist the token pair in localStorage so a session survives page reloads.
// (A hardened deployment might prefer httpOnly cookies; localStorage keeps the
// MVP simple and matches the stateless-JWT backend.)
const STORAGE_KEY = "bd.auth.tokens";

export function loadTokens(): TokenPair | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as TokenPair;
  } catch {
    // Corrupt value — drop it so the app falls back to logged-out.
    localStorage.removeItem(STORAGE_KEY);
    return null;
  }
}

export function saveTokens(tokens: TokenPair): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(tokens));
}

export function clearTokens(): void {
  localStorage.removeItem(STORAGE_KEY);
}
