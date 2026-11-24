import { apiFetch } from "./api";
import type { AuthResponse, TokenPair, User } from "../types/auth";

/** POST /auth/login — exchange credentials for a user + token pair. */
export function login(email: string, password: string): Promise<AuthResponse> {
  return apiFetch<AuthResponse>("/v1/auth/login", {
    method: "POST",
    auth: false,
    body: { email, password },
  });
}

/** POST /auth/refresh — exchange a refresh token for a fresh token pair. */
export function refreshTokens(refreshToken: string): Promise<TokenPair> {
  return apiFetch<TokenPair>("/v1/auth/refresh", {
    method: "POST",
    auth: false,
    body: { refresh_token: refreshToken },
  });
}

/** GET /auth/me — the authenticated caller's own account. */
export function fetchCurrentUser(): Promise<User> {
  return apiFetch<User>("/v1/auth/me");
}
