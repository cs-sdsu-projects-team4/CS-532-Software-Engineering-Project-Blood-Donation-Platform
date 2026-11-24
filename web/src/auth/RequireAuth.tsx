import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "./useAuth";

/**
 * Route guard for authenticated areas. While the session is being restored it
 * shows a placeholder (so we don't flash the login page); once resolved it
 * either renders the nested routes or redirects to /login, remembering where
 * the user was headed.
 */
export default function RequireAuth() {
  const { status } = useAuth();
  const location = useLocation();

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center text-gray-500">
        Loading…
      </div>
    );
  }

  if (status === "unauthenticated") {
    return (
      <Navigate to="/login" replace state={{ from: location.pathname }} />
    );
  }

  return <Outlet />;
}
