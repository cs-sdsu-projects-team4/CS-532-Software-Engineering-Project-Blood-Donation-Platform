import { useCallback, useState } from "react";
import { Link, useParams } from "react-router-dom";
import MatchList from "../components/MatchList";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../auth/useAuth";
import { useAsync } from "../hooks/useAsync";
import { formatDateTime } from "../lib/format";
import {
  approveRequest,
  cancelRequest,
  fulfillRequest,
  getRequest,
} from "../lib/requests-api";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="border-b border-gray-100 py-3 last:border-0 sm:grid sm:grid-cols-3 sm:gap-4">
      <dt className="text-sm font-medium text-gray-500">{label}</dt>
      <dd className="mt-1 text-sm text-gray-900 sm:col-span-2 sm:mt-0">
        {children}
      </dd>
    </div>
  );
}

export default function RequestDetailPage() {
  const { id } = useParams();
  const requestId = Number(id);
  const { user } = useAuth();

  const load = useCallback(() => getRequest(requestId), [requestId]);
  const { data: req, error, loading, reload } = useAsync(load);

  const [actionBusy, setActionBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  async function runAction(
    action: () => Promise<unknown>,
    onSuccess?: (result: unknown) => string | void,
  ) {
    setActionBusy(true);
    setActionError(null);
    setActionMessage(null);
    try {
      const result = await action();
      const message = onSuccess?.(result);
      if (message) {
        setActionMessage(message);
      }
      reload();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setActionBusy(false);
    }
  }

  const canApprove =
    req?.status === "pending_approval" && user?.role === "clinic_admin";
  const canCancel =
    req?.status === "pending_approval" || req?.status === "approved";
  const canFulfill = req?.status === "approved";
  const hasActions = canApprove || canCancel || canFulfill;

  return (
    <section className="max-w-2xl">
      <Link to="/requests" className="text-sm text-gray-600 hover:text-gray-900">
        ← Back to requests
      </Link>

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

      {!loading && !error && req && (
        <>
          <div className="mt-4 flex items-center gap-3">
            <h1 className="text-2xl font-semibold">Request #{req.id}</h1>
            <StatusBadge status={req.status} />
          </div>

          <dl className="mt-6 rounded-lg border border-gray-200 bg-white px-5 py-2">
            <Field label="Blood type">
              <span className="font-medium">{req.blood_type}</span>
            </Field>
            <Field label="Units needed">{req.units_needed}</Field>
            <Field label="Urgency">
              <span className="capitalize">{req.urgency}</span>
            </Field>
            <Field label="Search radius">
              {req.radius_m.toLocaleString()} m
            </Field>
            <Field label="Needed by">{formatDateTime(req.needed_by)}</Field>
            <Field label="Notes">{req.notes ?? "—"}</Field>
            <Field label="Location">
              {req.latitude != null && req.longitude != null
                ? `${req.latitude}, ${req.longitude}`
                : "Clinic default"}
            </Field>
            <Field label="Approved">
              {req.approved_at
                ? `by #${req.approved_by} on ${formatDateTime(req.approved_at)}`
                : "—"}
            </Field>
            <Field label="Created">
              {formatDateTime(req.created_at)} (by #{req.created_by})
            </Field>
            <Field label="Last updated">{formatDateTime(req.updated_at)}</Field>
          </dl>

          {actionMessage && (
            <p className="mt-4 rounded-md bg-green-50 px-4 py-3 text-sm text-green-700">
              {actionMessage}
            </p>
          )}
          {actionError && (
            <p
              role="alert"
              className="mt-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              {actionError}
            </p>
          )}

          {hasActions && (
            <div className="mt-6 flex flex-wrap gap-3">
              {canApprove && (
                <button
                  type="button"
                  disabled={actionBusy}
                  onClick={() =>
                    runAction(
                      () => approveRequest(req.id),
                      (result) => {
                        const n = (result as { matched_count: number })
                          .matched_count;
                        return `Approved — ${n} donor${n === 1 ? "" : "s"} matched.`;
                      },
                    )
                  }
                  className="rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white hover:bg-red-800 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Approve & match
                </button>
              )}
              {canFulfill && (
                <button
                  type="button"
                  disabled={actionBusy}
                  onClick={() =>
                    runAction(
                      () => fulfillRequest(req.id),
                      () => "Request marked fulfilled.",
                    )
                  }
                  className="rounded-md bg-green-700 px-4 py-2 text-sm font-medium text-white hover:bg-green-800 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Mark fulfilled
                </button>
              )}
              {canCancel && (
                <button
                  type="button"
                  disabled={actionBusy}
                  onClick={() =>
                    runAction(
                      () => cancelRequest(req.id),
                      () => "Request cancelled.",
                    )
                  }
                  className="rounded-md border border-red-300 px-4 py-2 text-sm font-medium text-red-700 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Cancel request
                </button>
              )}
            </div>
          )}

          {(req.status === "approved" || req.status === "fulfilled") && (
            <MatchList requestId={req.id} />
          )}
        </>
      )}
    </section>
  );
}
