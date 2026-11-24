import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/useAuth";
import type { Role } from "../types/auth";

interface NavItem {
  to: string;
  label: string;
  end?: boolean;
  roles: Role[];
}

const navItems: NavItem[] = [
  {
    to: "/",
    label: "Dashboard",
    end: true,
    roles: ["clinic_employee", "clinic_admin", "system_admin", "donor"],
  },
  {
    to: "/requests",
    label: "Requests",
    roles: ["clinic_employee", "clinic_admin"],
  },
];

/** "clinic_admin" -> "Clinic admin" for display. */
function roleLabel(role: Role): string {
  const spaced = role.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

export default function RootLayout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const visibleItems = user
    ? navItems.filter((item) => item.roles.includes(user.role))
    : [];

  function handleLogout() {
    logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center gap-6 px-6 py-4">
          <span className="text-lg font-semibold text-red-700">
            Blood Donation
          </span>
          <nav className="flex gap-4 text-sm">
            {visibleItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  isActive
                    ? "font-medium text-red-700"
                    : "text-gray-600 hover:text-gray-900"
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          {user && (
            <div className="ml-auto flex items-center gap-4 text-sm">
              <span className="text-right leading-tight">
                <span className="block font-medium text-gray-900">
                  {user.full_name}
                </span>
                <span className="block text-xs text-gray-500">
                  {roleLabel(user.role)}
                </span>
              </span>
              <button
                type="button"
                onClick={handleLogout}
                className="rounded-md border border-gray-300 px-3 py-1.5 text-gray-700 hover:bg-gray-50"
              >
                Sign out
              </button>
            </div>
          )}
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}
