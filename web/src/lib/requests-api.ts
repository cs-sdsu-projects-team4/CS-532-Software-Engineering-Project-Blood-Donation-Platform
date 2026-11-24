import { apiFetch } from "./api";
import type {
  ApproveResult,
  BloodRequest,
  CreateRequestInput,
} from "../types/request";
import type { RequestMatch } from "../types/match";

/** GET /requests — all blood requests for the caller's clinic. */
export function listRequests(): Promise<BloodRequest[]> {
  return apiFetch<BloodRequest[]>("/v1/requests");
}

/** GET /requests/{id} — one request the caller's clinic owns. */
export function getRequest(id: number): Promise<BloodRequest> {
  return apiFetch<BloodRequest>(`/v1/requests/${id}`);
}

/** POST /requests — create a new request (starts at pending_approval). */
export function createRequest(input: CreateRequestInput): Promise<BloodRequest> {
  return apiFetch<BloodRequest>("/v1/requests", {
    method: "POST",
    body: input,
  });
}

/** POST /requests/{id}/approve — clinic_admin approves and runs matching. */
export function approveRequest(id: number): Promise<ApproveResult> {
  return apiFetch<ApproveResult>(`/v1/requests/${id}/approve`, {
    method: "POST",
  });
}

/** POST /requests/{id}/cancel — cancel a pending or approved request. */
export function cancelRequest(id: number): Promise<BloodRequest> {
  return apiFetch<BloodRequest>(`/v1/requests/${id}/cancel`, {
    method: "POST",
  });
}

/** POST /requests/{id}/fulfill — mark an approved request fulfilled. */
export function fulfillRequest(id: number): Promise<BloodRequest> {
  return apiFetch<BloodRequest>(`/v1/requests/${id}/fulfill`, {
    method: "POST",
  });
}

/** GET /requests/{id}/matches — matched donors with contact details. */
export function listRequestMatches(id: number): Promise<RequestMatch[]> {
  return apiFetch<RequestMatch[]>(`/v1/requests/${id}/matches`);
}
