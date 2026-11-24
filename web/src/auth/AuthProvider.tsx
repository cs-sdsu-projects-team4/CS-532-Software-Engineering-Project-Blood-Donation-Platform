import { useCallback, useEffect, useMemo, useState } from "react";
import { setAuthTokenProvider, setTokenRefresher } from "../lib/api";
import * as authApi from "../lib/auth-api";
import { clearTokens, loadTokens, saveTokens } from "../lib/token-storage";
import type { User } from "../types/auth";
import { AuthContext, type AuthStatus } from "./auth-context";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [status, setStatus] = useState<AuthStatus>("loading");

  // Refresh the access token using the stored refresh token. Returns true when
  // a fresh token is saved; on failure it tears the session down and returns
  // false. Registered with the API client so 401s refresh-and-retry.
  const refresh = useCallback(async (): Promise<boolean> => {
    const tokens = loadTokens();
    if (!tokens) {
      return false;
    }
    try {
      const next = await authApi.refreshTokens(tokens.refresh_token);
      saveTokens(next);
      return true;
    } catch {
      clearTokens();
      setUser(null);
      setStatus("unauthenticated");
      return false;
    }
  }, []);

  // Wire the API client to our token storage + refresh, then restore any
  // existing session by reading the caller's own account.
  useEffect(() => {
    setAuthTokenProvider(() => loadTokens()?.access_token ?? null);
    setTokenRefresher(refresh);

    let active = true;
    (async () => {
      if (!loadTokens()) {
        setStatus("unauthenticated");
        return;
      }
      try {
        const me = await authApi.fetchCurrentUser();
        if (active) {
          setUser(me);
          setStatus("authenticated");
        }
      } catch {
        clearTokens();
        if (active) {
          setUser(null);
          setStatus("unauthenticated");
        }
      }
    })();

    return () => {
      active = false;
    };
  }, [refresh]);

  const login = useCallback(async (email: string, password: string) => {
    const res = await authApi.login(email, password);
    saveTokens(res.tokens);
    setUser(res.user);
    setStatus("authenticated");
  }, []);

  const logout = useCallback(() => {
    clearTokens();
    setUser(null);
    setStatus("unauthenticated");
  }, []);

  const value = useMemo(
    () => ({ user, status, login, logout }),
    [user, status, login, logout],
  );

  return <AuthContext value={value}>{children}</AuthContext>;
}
