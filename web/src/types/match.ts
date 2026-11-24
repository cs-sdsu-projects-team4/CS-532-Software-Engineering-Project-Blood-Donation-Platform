// Shapes mirror the Go backend match contract (backend/internal/http/requests.go,
// requestMatchResponse).

export type MatchStatus = "notified" | "accepted" | "declined" | "expired";

export interface RequestMatch {
  id: number;
  donor_user_id: number;
  distance_m: number | null;
  status: MatchStatus;
  notified_at: string | null;
  responded_at: string | null;
  full_name: string;
  email: string;
  phone: string | null;
}
