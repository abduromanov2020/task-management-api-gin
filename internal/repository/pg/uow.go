package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

// UoW is the pgx-backed UnitOfWork. InTx delegates to pgx.BeginTxFunc so
// commit/rollback semantics are handled by the driver: the closure's error
// (including panics, after Recovery middleware) rolls everything back.
type UoW struct {
	pool *pgxpool.Pool
}

func NewUoW(pool *pgxpool.Pool) *UoW { return &UoW{pool: pool} }

func (u *UoW) reposFor(q *sqlcdb.Queries) domain.TxRepos {
	return domain.TxRepos{
		Users:    NewUserRepo(q),
		Teams:    NewTeamRepo(q),
		Tasks:    NewTaskRepo(q),
		TaskLogs: NewTaskLogRepo(q),
		Idem:     NewIdempotencyRepo(q),
		Notifier: NewLogNotifier(),
	}
}

func (u *UoW) Repos() domain.TxRepos {
	return u.reposFor(sqlcdb.New(u.pool))
}

func (u *UoW) InTx(ctx context.Context, fn func(domain.TxRepos) error) error {
	return pgx.BeginTxFunc(ctx, u.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		return fn(u.reposFor(sqlcdb.New(tx)))
	})
}
