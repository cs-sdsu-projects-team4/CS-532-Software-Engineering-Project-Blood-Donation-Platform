// Shapes mirror the Go backend auth contract (see backend/internal/http/auth.go).

export type Role =
  | "donor"
  | "clinic_employee"
  | "clinic_admin"
  | "system_admin";

export interface User {
  id: number;
  email: string;
  role: Role;
  full_name: string;
  phone: string | null;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
}

/** Response from POST /auth/login (and /auth/register): user + token pair. */
export interface AuthResponse {
  user: User;
  tokens: TokenPair;
}
