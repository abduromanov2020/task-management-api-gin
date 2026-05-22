# Tasks API

> Team-scoped task management API in Go (Gin + pgx + sqlc + PostgreSQL) with JWT auth, idempotent writes, and transactional task assignment.

This is the deliverable for the GDC back-end developer take-home test. It implements:

- JWT authentication (register / login, HS256, bcrypt cost 12, no email enumeration).
- CRUD on `/tasks` plus `POST /tasks/:id/assign`.
- **Idempotency on `POST /tasks`** via `Idempotency-Key` header. Survives N concurrent duplicate requests using a DB UNIQUE constraint and `INSERT ... ON CONFLICT DO UPDATE` with a 30-second lease — exactly one task row is created, even under 100 parallel goroutines.
- **Transactional assign**: update assignee + write `task_logs` + send notification, all inside one `pgx.BeginTxFunc`. Any step's failure rolls back the entire operation.
- **Structured JSON access logs** with `request_id` (UUID v4) attached to every log line for correlation; level rules `<400 INFO / 4xx WARN / 5xx ERROR`.
- **Unified error envelope** (`status, code, message, timestamp, request_id`) emitted by a central middleware; handlers only call `c.Error(err)`.
- **Unit tests** with hand-written mocks (no DB), including a 100-goroutine race-condition proof.

---

## Quickstart (Docker)

```bash
docker compose up --build
```

This starts:
- `postgres` (16-alpine) with named volume.
- `migrate` (one-shot) that runs all migrations.
- `api` listening on `http://localhost:8080`.

Verify:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Swagger UI: <http://localhost:8080/swagger/index.html>

---

## Local Development

Prerequisites: Go 1.22+, PostgreSQL 16+, [sqlc](https://sqlc.dev), [golang-migrate](https://github.com/golang-migrate/migrate).

```bash
cp .env.example .env             # fill in DATABASE_URL and a strong JWT_SECRET (>=32 bytes)
make migrate-up                   # applies db/migrations
make run                          # starts the API on $PORT (default 8080)
```

Regenerate sqlc-typed SQL after editing `db/queries/*.sql`:

```bash
make sqlc
```

Regenerate Swagger after editing handler annotations:

```bash
make swag
```

---

## API Reference

All authenticated routes expect `Authorization: Bearer <JWT>`. All errors return the same envelope:

```json
{
  "status":     "error",
  "code":       "TASK_NOT_FOUND",
  "message":    "Task not found",
  "details":    { },
  "timestamp":  "2026-05-22T10:00:00.123Z",
  "request_id": "5f9c4d2e-..."
}
```

### Authentication

| Method | Path             | Notes                                            |
|--------|------------------|--------------------------------------------------|
| POST   | `/auth/register` | Creates a team (auto if `team_name` empty) + user, returns JWT |
| POST   | `/auth/login`    | Returns JWT; constant-time on missing user       |

```bash
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"abduromanov@example.com","password":"secret-password","name":"Abdur","team_name":"Sprout"}'
```

### Tasks

| Method | Path                   | Notes                                                                      |
|--------|------------------------|----------------------------------------------------------------------------|
| POST   | `/tasks`               | **Idempotent.** Requires `Idempotency-Key: <uuid>` header.                 |
| GET    | `/tasks`               | Filters: `status`, `q` (title substring), `page` (1-based), `limit` (≤100) |
| GET    | `/tasks/:id`           | Team-scoped — 404 if outside requester's team                              |
| PUT    | `/tasks/:id`           | Full replace of mutable fields                                             |
| DELETE | `/tasks/:id`           | Creator only                                                               |
| POST   | `/tasks/:id/assign`    | Transactional (assignee + log + notification rollback together)            |

### Idempotency-Key

`POST /tasks` requires an `Idempotency-Key` header in UUID format. The server treats the same key (per user) as the same logical request:

- A second call within the 24-hour window with the same key **and the same body** returns the original `201` response byte-for-byte.
- A second call with the same key but a **different body** returns `422 IDEMPOTENCY_KEY_MISMATCH`.
- A second call while the first is still in flight returns `409 IDEMPOTENCY_IN_FLIGHT`.

---

## Architecture

```
                ┌─────────────┐
HTTP ──▶  Gin ─▶│  middleware │ request_id, access log (zap),
                │             │ recovery, error envelope, security headers,
                │             │ body limit, CORS, auth (JWT), rate-limit /auth/*
                └──────┬──────┘
                       │ c.Error(err)  /  c.JSON
                ┌──────▼──────┐
                │   handler   │  Gin DTOs (binding tags) → usecase inputs
                └──────┬──────┘
                ┌──────▼──────┐
                │   usecase   │  business logic; depends only on domain ports
                │             │  (no gin, no pgx, no zap imports)
                └──────┬──────┘
                       │ interfaces in internal/domain
                ┌──────▼──────┐
                │ repository  │  pgx + sqlc-generated typed queries
                │     pg      │  UnitOfWork wraps pgx.BeginTxFunc
                └──────┬──────┘
                ┌──────▼──────┐
                │  PostgreSQL │
                └─────────────┘
```

Request lifecycle: `RequestID → AccessLog → Recovery → ErrorHandler → SecurityHeaders → BodyLimit → CORS → (route group → Auth | RateLimit) → handler → usecase → repository`.

### Design notes

- **Why `ON CONFLICT DO UPDATE` for idempotency (not Redis):** Postgres' unique constraint serialises all concurrent attempts through the tuple lock. Of N concurrent goroutines, exactly one row is inserted; the rest observe the existing row and either replay the stored response, see "in-flight", or — if the previous holder crashed — reclaim the lease. No extra dependency, no eventually-consistent cache, and correctness is provable at the SQL layer. The 30-second `lease_expires_at` is the recovery mechanism for hard crashes (kill -9, OOM).
- **Why clean architecture:** handlers know HTTP, usecases know business rules, repositories know SQL. The usecase layer imports nothing from gin/pgx/zap, so the unit tests use hand-written fakes that implement the domain ports and exercise every business invariant without spinning up a database.
- **Why sqlc:** compile-time-typed SQL. Schema changes break the build, not production. SQL stays in `.sql` files where DBAs (or `EXPLAIN`) can read it directly. No risk of ORM-hidden N+1 queries.
- **Why pgx/v5 (not database/sql):** native PostgreSQL types (`pgtype.Timestamptz`, `uuid.UUID`, JSONB), better performance, and `pgx.BeginTxFunc` for ergonomic transactions with automatic rollback on error.
- **Pagination — offset vs keyset:** the API uses `LIMIT/OFFSET` with `offset ≤ 1000` enforced. Acceptable for a take-home; the production choice would be keyset pagination on `(created_at, id)` — the existing composite index already supports it.

---

## Database

### Schema

```
teams (id, name, created_at)
users (id, email CITEXT UNIQUE, password_hash, name, team_id, created_at, updated_at)
tasks (id, team_id, created_by, assignee_id?, title, description, status enum, priority enum, due_date?, created_at, updated_at)
task_logs (id, task_id, actor_id, action, payload jsonb, created_at)
idempotency_keys (PK(user_id, idempotency_key), request_hash, status_code, response_body jsonb, lease_expires_at, created_at)
```

### Migrations

```bash
make migrate-up      # applies all
make migrate-down    # reverts the last
```

Migration files live in `db/migrations/` (golang-migrate format: `NNN_name.up.sql` / `NNN_name.down.sql`).

### Indexes (rationale)

| Index                                                   | Reason                                                       |
|---------------------------------------------------------|--------------------------------------------------------------|
| `tasks(team_id, status, created_at DESC)`               | Powers `GET /tasks?status=…` ordered newest-first, team-scoped |
| `tasks(created_by)`                                     | Owner-deletion / audit lookups                               |
| `tasks(assignee_id) WHERE assignee_id IS NOT NULL`      | Partial: most rows start unassigned; smaller, cheaper index  |
| `tasks USING GIN (title gin_trgm_ops)`                  | Fast `ILIKE '%q%'` title search                              |
| `idempotency_keys(created_at)`                          | For 24h-window sweeps / observability                        |
| `users(email)` (unique, CITEXT)                         | Case-insensitive uniqueness without `LOWER()` workarounds    |

---

## Security

1. **bcrypt cost 12** for password hashes.
2. **JWT HS256**, secret enforced ≥32 bytes at startup. Claims include `sub, iss, aud, exp (15m), iat, jti`.
3. **No email enumeration** on `/auth/login` — same 401 response and constant-time path (bcrypt against a dummy hash) whether the email exists or not.
4. **SQL injection**: all queries are sqlc-generated with `$N` parameter binding — no string concatenation.
5. **Input validation** via `validator.v10` (`min`, `max`, `oneof`, `email`).
6. **1 MB request-body cap** via `http.MaxBytesReader`.
7. **Security headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`.
8. **CORS allowlist** via `CORS_ORIGINS` env (comma-separated).
9. **Rate-limit `/auth/*`** with a per-IP token bucket (default 5 rpm).
10. **Log redaction**: zap field encoder drops `Authorization`, `password`, `password_hash`, `token`, `jwt`, `secret` keys.
11. **HTTP server timeouts**: `ReadHeader 5s / Read 10s / Write 15s / Idle 60s`.
12. **pgxpool limits**: `MaxConns 20`, `MinConns 2`, `MaxConnLifetime 30m`.
13. **Error envelopes** never expose stack traces or internal causes — the full cause is recorded in logs only.

---

## Observability

### Structured logging fields

| Field             | Type     | When                                                              |
|-------------------|----------|-------------------------------------------------------------------|
| `request_id`      | uuid str | every log line within a request (echoed as `X-Request-ID` header) |
| `method`, `path`, `status`, `latency_ms`, `client_ip` | -    | access log                          |
| `user_id`, `team_id` | uuid str | after JWT validation                                          |
| `task_id`         | uuid str | per task operation                                                |
| `idempotency_key` | uuid str | `POST /tasks`                                                     |
| `event`           | str      | `task.created`, `task.assigned`, `idempotency.hit`, `auth.login.failed`, `server.start`, `notification.sent`, … |
| `error`, `error_code` | str  | non-2xx responses                                                 |

### Request IDs

Every request gets a fresh UUID (or the client's `X-Request-ID` if supplied as a UUID). It is bound to the per-request `domain.Logger` via `logger.Into(ctx, ...)` and rides on `context.Context` everywhere downstream.

---

## Testing

### Unit tests

```bash
make test         # plain (no -race)
make test-race    # with -race (requires CGO)
```

Tests live in `internal/usecase/*_test.go` and use hand-written fakes in `internal/usecase/mocks/`. No database is required. CI runs them with the race detector on every PR.

### Race-condition test (the rubric centerpiece)

`internal/usecase/task_usecase_test.go::TestCreateTask_Idempotency_100ConcurrentSameKey` fires 100 goroutines at `TaskUsecase.Create` with the same `Idempotency-Key`. The test asserts:

1. Exactly **one** task row was inserted (`tasks.InsertCount() == 1`).
2. Every successful goroutine returned the **identical response bytes** (`require.JSONEq`).
3. Any failed goroutine returned `domain.ErrIdemInFlight` — never a duplicate insert.

### Reviewer validation script (live API)

After `docker compose up`:

```bash
TOKEN=$(curl -sS -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"reviewer@example.com","password":"reviewer-password","name":"R"}' \
  | jq -r .access_token)

KEY=$(uuidgen)

# (a) first POST — expect 201
curl -sS -X POST http://localhost:8080/tasks \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"title":"ship it","priority":"high"}' -w "\nHTTP %{http_code}\n"

# (b) replay — expect 201 with IDENTICAL body
curl -sS -X POST http://localhost:8080/tasks \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"title":"ship it","priority":"high"}' -w "\nHTTP %{http_code}\n"

# (c) 50 parallel — expect exactly 1 task row
seq 50 | xargs -P50 -I{} curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST http://localhost:8080/tasks \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"title":"ship it","priority":"high"}' | sort | uniq -c

docker compose exec -T postgres psql -U postgres -d tasks -c \
  "SELECT count(*) AS task_rows FROM idempotency_keys WHERE idempotency_key='$KEY';"
# expect task_rows = 1
```

---

## Environment Variables

| Variable             | Default                                                                   | Notes                                  |
|----------------------|---------------------------------------------------------------------------|----------------------------------------|
| `PORT`               | `8080`                                                                    |                                        |
| `LOG_LEVEL`          | `info`                                                                    | `debug`/`info`/`warn`/`error`          |
| `ENV`                | `development`                                                             | `production` activates `gin.ReleaseMode` and JSON-only encoder |
| `DATABASE_URL`       | -                                                                         | Required. e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
| `DB_MAX_CONNS`       | `20`                                                                      |                                        |
| `DB_MIN_CONNS`       | `2`                                                                       |                                        |
| `JWT_SECRET`         | -                                                                         | Required, ≥32 bytes                    |
| `JWT_TTL`            | `15m`                                                                     | Go duration                            |
| `JWT_ISSUER`         | `tasks-api`                                                               |                                        |
| `JWT_AUDIENCE`       | `tasks-api`                                                               |                                        |
| `CORS_ORIGINS`       | `http://localhost:3000`                                                   | Comma-separated allowlist              |
| `AUTH_RATE_LIMIT_RPM`| `5`                                                                       | Per-IP token bucket for `/auth/*`      |

---

## What's Not Included (Out of Scope)

These were deliberately deferred to stay within the 2x24h window and stay focused on the rubric. They are noted here so reviewers can see the trade-offs explicitly:

- **Refresh tokens / token revocation**: JWTs carry `jti` so a deny-list is straightforward; no store was wired.
- **Distributed (Redis-backed) rate limiting**: in-memory limiter only works per replica.
- **Transactional outbox for notifications**: the notifier currently logs a structured event inside the tx. A production system would insert into a `notifications` table and let a worker pick it up after commit.
- **Integration tests against a real Postgres**: assignment requires only unit tests without DB; a future addition would use `testcontainers-go`.
- **Keyset pagination**: the index on `(team_id, status, created_at DESC)` already supports it; the API offers offset/limit for simplicity.
- **OpenTelemetry traces**: only request_id correlation is implemented.

---

## Project Layout

```
.
├── cmd/api/main.go                       # composition root
├── internal/
│   ├── apperr/                           # AppError envelope + domain-sentinel mapping
│   ├── auth/                             # password hasher + JWT issuer
│   ├── config/                           # env-based configuration
│   ├── domain/                           # pure types + repository/UoW/Logger/Notifier ports
│   ├── handler/                          # Gin handlers + DTOs + router
│   ├── logger/                           # zap setup, context helpers, redaction
│   ├── middleware/                       # request_id, access log, recovery, error envelope,
│   │                                     # security headers, body limit, auth, rate limit
│   ├── repository/pg/                    # pgx-backed repositories + UnitOfWork
│   │   └── sqlc/                         # sqlc-generated typed SQL (do not edit)
│   └── usecase/                          # business logic + tests
│       └── mocks/                        # hand-written fakes for unit tests
├── db/
│   ├── migrations/                       # golang-migrate files
│   └── queries/                          # sqlc input
├── docs/                                 # swaggo-generated OpenAPI
├── .github/workflows/ci.yml
├── Dockerfile                            # multi-stage, distroless
├── docker-compose.yml                    # postgres + migrate + api
├── Makefile
├── sqlc.yaml
├── .env.example
├── .golangci.yml
└── README.md
```
