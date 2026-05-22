package pg

import (
	"context"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

type TaskLogRepo struct{ q *sqlcdb.Queries }

func NewTaskLogRepo(q *sqlcdb.Queries) *TaskLogRepo { return &TaskLogRepo{q: q} }

func (r *TaskLogRepo) Insert(ctx context.Context, log domain.TaskLog) error {
	payload, err := encodePayload(log.Payload)
	if err != nil {
		return err
	}
	return mapErr(r.q.InsertTaskLog(ctx, sqlcdb.InsertTaskLogParams{
		TaskID:  log.TaskID,
		ActorID: log.ActorID,
		Action:  log.Action,
		Payload: payload,
	}))
}
