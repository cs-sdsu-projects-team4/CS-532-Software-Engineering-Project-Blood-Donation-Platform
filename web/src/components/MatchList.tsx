import { useCallback } from "react";
import { useAsync } from "../hooks/useAsync";
import { formatDateTime } from "../lib/format";
import { listRequestMatches } from "../lib/requests-api";
import type { MatchStatus } from "../types/match";

const MATCH_STYLES: Record<MatchStatus, string> = {
  notified: "bg-amber-100 text-amber-800",
  accepted: "bg-green-100 text-green-800",
  declined: "bg-red-100 text-red-700",
  expired: "bg-gray-100 text-gray-500",
};

function MatchStatusBadge({ status }: { status: MatchStatus }) {
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium capitalize ${MATCH_STYLES[status]}`}
    >
      {status}
    </span>
  );
}

function formatDistance(meters: number | null): string {
  if (meters == null) {
    return "—";
  }
  if (meters >= 1000) {
    return `${(meters / 1000).toFixed(1)} km`;
  }
  return `${Math.round(meters)} m`;
}

export default function MatchList({ requestId }: { requestId: number }) {
  const load = useCallback(
    () => listRequestMatches(requestId),
    [requestId],
  );
  const { data, error, loading, reload } = useAsync(load);

  return (
    <section className="mt-8">
      <h2 className="text-lg font-semibold">Matched donors</h2>

      {loading && <p className="mt-3 text-sm text-gray-500">Loading…</p>}

      {error && !loading && (
        <div className="mt-3 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">
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
        <p className="mt-3 text-sm text-gray-500">
          No donors were matched for this request.
        </p>
      )}

      {!loading && !error && data && data.length > 0 && (
        <div className="mt-3 overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-gray-200 text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-3 font-medium">Donor</th>
                <th className="px-4 py-3 font-medium">Contact</th>
                <th className="px-4 py-3 font-medium">Distance</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Responded</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {data.map((match) => (
                <tr key={match.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium">{match.full_name}</td>
                  <td className="px-4 py-3">
                    <a
                      href={`mailto:${match.email}`}
                      className="text-red-700 hover:underline"
                    >
                      {match.email}
                    </a>
                    {match.phone && (
                      <span className="block text-gray-500">{match.phone}</span>
                    )}
                  </td>
                  <td className="px-4 py-3">{formatDistance(match.distance_m)}</td>
                  <td className="px-4 py-3">
                    <MatchStatusBadge status={match.status} />
                  </td>
                  <td className="px-4 py-3">
                    {formatDateTime(match.responded_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
