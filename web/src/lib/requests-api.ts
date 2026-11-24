import { apiFetch } from "./api";
import type { BloodRequest, CreateRequestInput } from "../types/request";

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
