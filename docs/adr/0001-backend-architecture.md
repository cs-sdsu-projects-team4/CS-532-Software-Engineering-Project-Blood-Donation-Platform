# ADR 0001 — Backend architecture & key decisions

Status: Accepted · Scope: Phase 1 (Go backend)

Short record of the decisions behind the backend so future work (and reviewers)
have the "why", not just the code.

## Stack
- **Go + Chi** for a small, explicit REST/JSON API over HTTPS.
- **PostgreSQL + PostGIS** — PostGIS gives geography types and `ST_DWithin` radius
  queries, which the donor-matching step relies on.
- **pgx + sqlc** — we write explicit SQL in `queries/` and generate type-safe Go
  into `internal/store`; generated code is never hand-edited.
- **Goose** migrations in `migrations/` are the source of truth for the schema.
- **JWT access/refresh + Argon2id** for auth; refresh tokens are stateless in the
  MVP to avoid extra schema (token-type separation prevents cross-use).

## Structure
- `cmd/` + `internal/` per golang-standards/project-layout. One handler file per
  resource under `internal/http/`; business rules live in `domain/`, `matching/`,
  and `notify/`, not in handlers.
- Input is validated server-side; errors are structured JSON (`{error:{code,message}}`).

## Security / RBAC
- Authorization is enforced **only in the Go backend** (never in clients), via JWT
  + role middleware. A clinic request must be **approved by a `clinic_admin` before
  any donor is matched/notified** — enforced by request status, not just by role.
- `system_admin` provisions clinics and staff; no account self-registers as staff
  or admin. The first `system_admin` is seeded at startup from env (`bootstrap`).

## Matching (deterministic, MVP)
- No ML, no Redis (decisions D-014/D-015). The pipeline is: blood compatibility
  (resolved in Go from the authoritative matrix) → screening/eligibility →
  availability + recent-donation window → PostGIS radius. Compatibility stays in
  one place (`domain`); the data-level filters run in SQL where the PostGIS index is.

## Notifications
- `notify.Notifier` abstracts push delivery. The default `LogNotifier` logs, so the
  whole flow runs locally and in tests without Firebase; a real FCM implementation
  slots in behind the same interface. Dispatch happens synchronously at approval
  (best-effort); a background worker was deliberately deferred as unnecessary for
  the MVP.

## Privacy (NFR-02)
- We store only what matching, contact, and scheduling need — no national IDs, no
  clinic medical test records. Age (18+) and blood type are self-reported; screening
  guidance is non-diagnostic and defers the final decision to the clinic (FR-04).
