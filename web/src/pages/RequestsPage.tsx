import { useCallback } from "react";
import { Link } from "react-router-dom";
import StatusBadge from "../components/StatusBadge";
import { useAsync } from "../hooks/useAsync";
import { formatDateTime } from "../lib/format";
import { listRequests } from "../lib/requests-api";

export default function RequestsPage() {
  const load = useCallback(() => listRequests(), []);
  const { data, error, loading, reload } = useAsync(load);

  return (
    <section>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Requests</h1>
        <Link
          to="/requests/new"
          className="rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white hover:bg-red-800"
        >
          New request
        </Link>
      </div>

      {loading && <p className="mt-6 text-gray-500">Loading…</p>}

      {error && !loading && (
        <div className="mt-6 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}{" "}
          <button
            type="button"
            onClick={reload}
            className="font-medium underline hover:no-underline"
          >
            Retry
          </button>
        </div>
      )}

      {!loading && !error && data && data.length === 0 && (
        <p className="mt-6 text-gray-500">
          No requests yet. Create one to get started.
        </p>
      )}

      {!loading && !error && data && data.length > 0 && (
        <div className="mt-6 overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-gray-200 text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-3 font-medium">ID</th>
                <th className="px-4 py-3 font-medium">Blood type</th>
                <th className="px-4 py-3 font-medium">Units</th>
                <th className="px-4 py-3 font-medium">Urgency</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Needed by</th>
                <th className="px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {data.map((req) => (
                <tr key={req.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <Link
                      to={`/requests/${req.id}`}
                      className="font-medium text-red-700 hover:underline"
                    >
                      #{req.id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-medium">{req.blood_type}</td>
                  <td className="px-4 py-3">{req.units_needed}</td>
                  <td className="px-4 py-3 capitalize">{req.urgency}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={req.status} />
                  </td>
                  <td className="px-4 py-3">{formatDateTime(req.needed_by)}</td>
                  <td className="px-4 py-3">{formatDateTime(req.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
