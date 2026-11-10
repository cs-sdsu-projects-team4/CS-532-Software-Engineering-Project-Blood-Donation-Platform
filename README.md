# Blood Donation Coordination App

A platform that helps clinics reach compatible blood donors faster during shortages. Donors use a
cross-platform mobile app; clinic and administrative staff use a web dashboard. Both clients share a
single Go REST API backed by PostgreSQL + PostGIS.

> **Course:** CS 532 Software Engineering · Fall 2026
> **Team:** Davit Sibashvili, Nino Sengelaia, Rafik Papoyan, Arshak Abajyan, Beka Kutsurua

The application coordinates outreach, donor responses, and appointment scheduling. Identity
verification, medical screening, testing, collection, and the final donation-eligibility decision
remain the responsibility of the clinic and happen in person, outside the app.

## Core workflow

1. A clinic employee creates an urgent blood request (type, amount, urgency, deadline, location,
   contact, appointment slots).
2. A clinic administrator reviews and approves it before any donor is notified.
3. The backend matches potentially compatible donors by blood compatibility, screening state,
   availability, recent donation history, and distance.
4. Matched donors are notified and can view the request.
5. A donor accepts or declines; an accepting donor selects an appointment slot.
6. Clinic staff verify identity, repeat required screening/testing, and make the final decision.

## Tech stack

| Layer | Technology |
|---|---|
| Web frontend | React + Vite + Tailwind CSS |
| Mobile frontend | Flutter + Dart |
| Backend | Go + Chi (REST/JSON over HTTPS) |
| Database | PostgreSQL + PostGIS |
| Database access | pgx + sqlc |
| Migrations | Goose |
| Auth | JWT access/refresh tokens + Argon2id |
| Notifications | Firebase Cloud Messaging |
| Containers | Docker + Docker Compose |
| Web server | Nginx (HTTPS, static web, reverse proxy) |
| CI/CD | GitHub Actions + GitHub Container Registry |
| Hosting | AWS EC2 |

## Repository structure

```
.
├── backend/                 # Go + Chi REST API and background worker
│   ├── cmd/api/             #   API server entrypoint
│   ├── cmd/worker/          #   background worker (matching + notification dispatch)
│   ├── internal/
│   │   ├── config/          #   configuration loading
│   │   ├── http/            #   router, middleware, handlers
│   │   ├── auth/            #   JWT, Argon2id, role-based access control
│   │   ├── store/           #   generated database access (sqlc)
│   │   ├── domain/          #   entities and business rules
│   │   ├── matching/        #   donor matching pipeline
│   │   └── notify/          #   push-notification client
│   ├── migrations/          # Goose database migrations
│   ├── queries/             # SQL source for sqlc
│   └── sqlc.yaml
├── web/                     # React web dashboard (clinic / admin)
├── mobile/                  # Flutter mobile app (donors)
├── deploy/                  # docker-compose, nginx config, env example
├── docs/                    # API contract and design notes
└── .github/workflows/       # CI and deployment pipelines
```

## Getting started

> Prerequisites: Go, Docker + Docker Compose, Node.js, and the Flutter SDK.

```bash
# 1. Copy the environment template and fill in local values (never commit real secrets)
cp deploy/.env.example deploy/.env

# 2. Start the local stack (API, worker, PostgreSQL/PostGIS)
cd deploy && docker compose up -d

# 3. Backend development
cd backend
go run ./cmd/api        # run the API locally
go test ./...           # run tests

# 4. Web development
cd web && npm install && npm run dev

# 5. Mobile development
cd mobile && flutter run
```

## Development process

- Two-week Agile iterations after the requirements baseline.
- Work is tracked with GitHub Issues and GitHub Projects; each item links to a functional requirement.
- The `main` branch is protected. Develop on short `feature/*` or `fix/*` branches and merge via pull
  request after at least one peer review and passing checks.
- Use semantic commit messages (e.g. `feat(backend): add donor registration`).
- Configuration examples are committed without secrets, private keys, or real sensitive data.

## Scope boundaries

The system does **not** make final medical eligibility decisions, test/collect/transport/store blood
products, store national ID numbers or clinical test records, or integrate with national identity,
hospital, or blood-bank systems. It is a course prototype, not a certified production service.

## License

Coursework for CS 532. Not licensed for production or clinical use.
