package pg

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

type IdempotencyRepo struct{ q *sqlcdb.Queries }

func NewIdempotencyRepo(q *sqlcdb.Queries) *IdempotencyRepo { return &IdempotencyRepo{q: q} }

// Acquire executes the upsert and translates the three possible outcomes
// (fresh claim, completed-previously, in-flight) into a domain result.
//
//   - pgx returns no row (ErrNoRows) → another caller holds an unexpired lease.
//     We then SELECT the existing row to confirm; if it's still in-flight, we
//     report InFlight. If by then the other caller already completed and the
//     UPDATE-WHERE clause didn't reclaim, we report Completed.
//   - acquired=true → caller owns the lease, proceed.
//   - acquired=false + status != 0 → completed previously, replay.
func (r *IdempotencyRepo) Acquire(ctx context.Context, userID, key uuid.UUID, requestHash string) (domain.IdempotencyAcquireResult, error) {
	row, err := r.q.AcquireIdempotency(ctx, sqlcdb.AcquireIdempotencyParams{
		UserID:         userID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
	})
	if err == nil {
		if row.Acquired {
			return domain.IdempotencyAcquireResult{Acquired: true}, nil
		}
		// acquired=false: ON CONFLICT DO UPDATE WHERE matched & returned a row;
		// this happens if status_code != 0 (completed) — sqlc gave us the body.
		return domain.IdempotencyAcquireResult{
			Completed:    true,
			StatusCode:   int(row.StatusCode),
			ResponseBody: json.RawMessage(row.ResponseBody),
			StoredHash:   row.RequestHash,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.IdempotencyAcquireResult{}, err
	}
	// No row returned → the ON CONFLICT DO UPDATE WHERE didn't match; row exists
	// but is either in-flight (status=0, lease not expired) or already completed.
	got, gerr := r.q.GetIdempotency(ctx, sqlcdb.GetIdempotencyParams{
		UserID:         userID,
		IdempotencyKey: key,
	})
	if gerr != nil {
		return domain.IdempotencyAcquireResult{}, gerr
	}
	if got.StatusCode == 0 {
		return domain.IdempotencyAcquireResult{InFlight: true, StoredHash: got.RequestHash}, nil
	}
	return domain.IdempotencyAcquireResult{
		Completed:    true,
		StatusCode:   int(got.StatusCode),
		ResponseBody: json.RawMessage(got.ResponseBody),
		StoredHash:   got.RequestHash,
	}, nil
}

func (r *IdempotencyRepo) Complete(ctx context.Context, userID, key uuid.UUID, status int, body json.RawMessage) error {
	return mapErr(r.q.CompleteIdempotency(ctx, sqlcdb.CompleteIdempotencyParams{
		UserID:         userID,
		IdempotencyKey: key,
		StatusCode:     int32(status),
		ResponseBody:   []byte(body),
	}))
}
