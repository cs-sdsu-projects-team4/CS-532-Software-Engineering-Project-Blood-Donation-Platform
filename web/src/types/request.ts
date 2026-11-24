// Shapes mirror the Go backend request contract (backend/internal/http/requests.go).

export type BloodType =
  | "O-"
  | "O+"
  | "A-"
  | "A+"
  | "B-"
  | "B+"
  | "AB-"
  | "AB+";

export const BLOOD_TYPES: BloodType[] = [
  "O-",
  "O+",
  "A-",
  "A+",
  "B-",
  "B+",
  "AB-",
  "AB+",
];

export type Urgency = "routine" | "urgent" | "critical";

export const URGENCIES: Urgency[] = ["routine", "urgent", "critical"];

export type RequestStatus =
  | "pending_approval"
  | "approved"
  | "fulfilled"
  | "cancelled"
  | "expired";

export interface BloodRequest {
  id: number;
  clinic_id: number;
  created_by: number;
  blood_type: BloodType;
  units_needed: number;
  urgency: Urgency;
  status: RequestStatus;
  approved_by: number | null;
  approved_at: string | null;
  radius_m: number;
  latitude: number | null;
  longitude: number | null;
  notes: string | null;
  needed_by: string | null;
  created_at: string;
  updated_at: string;
}

/** Result of approving a request: the updated request + how many donors matched. */
export interface ApproveResult {
  request: BloodRequest;
  matched_count: number;
}

export interface CreateRequestInput {
  blood_type: BloodType;
  units_needed: number;
  urgency?: Urgency;
  radius_m?: number;
  latitude?: number | null;
  longitude?: number | null;
  notes?: string;
  needed_by?: string | null;
}
