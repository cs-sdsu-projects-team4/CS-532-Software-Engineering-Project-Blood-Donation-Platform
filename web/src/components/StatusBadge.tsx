import type { RequestStatus } from "../types/request";

const STYLES: Record<RequestStatus, string> = {
  pending_approval: "bg-amber-100 text-amber-800",
  approved: "bg-green-100 text-green-800",
  fulfilled: "bg-blue-100 text-blue-800",
  cancelled: "bg-gray-100 text-gray-700",
  expired: "bg-gray-100 text-gray-500",
};

const LABELS: Record<RequestStatus, string> = {
  pending_approval: "Pending approval",
  approved: "Approved",
  fulfilled: "Fulfilled",
  cancelled: "Cancelled",
  expired: "Expired",
};

export default function StatusBadge({ status }: { status: RequestStatus }) {
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${STYLES[status]}`}
    >
      {LABELS[status]}
    </span>
  );
}
