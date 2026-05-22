package pg

import (
	"context"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

type TaskRepo struct{ q *sqlcdb.Queries }

func NewTaskRepo(q *sqlcdb.Queries) *TaskRepo { return &TaskRepo{q: q} }

func (r *TaskRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	t, err := r.q.GetTaskByID(ctx, id)
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return taskFromSqlc(t), nil
}

func (r *TaskRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	t, err := r.q.GetTaskForUpdate(ctx, id)
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return taskFromSqlc(t), nil
}

func (r *TaskRepo) List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, int64, error) {
	var statusFilter *sqlcdb.TaskStatus
	if f.Status != nil {
		v := sqlcdb.TaskStatus(*f.Status)
		statusFilter = &v
	}
	rows, err := r.q.ListTasks(ctx, sqlcdb.ListTasksParams{
		TeamID:       f.TeamID,
		StatusFilter: statusFilter,
		Q:            f.Query,
		Off:          f.Offset,
		Lim:          f.Limit,
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}
	out := make([]domain.Task, 0, len(rows))
	var total int64
	for _, row := range rows {
		out = append(out, taskFromListRow(row))
		total = row.Total
	}
	return out, total, nil
}

func (r *TaskRepo) Create(ctx context.Context, t domain.Task) (domain.Task, error) {
	created, err := r.q.CreateTask(ctx, sqlcdb.CreateTaskParams{
		TeamID:      t.TeamID,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  t.AssigneeID,
		Title:       t.Title,
		Description: t.Description,
		Status:      sqlcdb.TaskStatus(t.Status),
		Priority:    sqlcdb.TaskPriority(t.Priority),
		DueDate:     ptrToTs(t.DueDate),
	})
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return taskFromSqlc(created), nil
}

func (r *TaskRepo) UpdateAssignee(ctx context.Context, id, assignee uuid.UUID) (domain.Task, error) {
	updated, err := r.q.UpdateTaskAssignee(ctx, sqlcdb.UpdateTaskAssigneeParams{
		ID:         id,
		AssigneeID: &assignee,
	})
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return taskFromSqlc(updated), nil
}

func (r *TaskRepo) Update(ctx context.Context, t domain.Task) (domain.Task, error) {
	updated, err := r.q.UpdateTask(ctx, sqlcdb.UpdateTaskParams{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      sqlcdb.TaskStatus(t.Status),
		Priority:    sqlcdb.TaskPriority(t.Priority),
		DueDate:     ptrToTs(t.DueDate),
	})
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return taskFromSqlc(updated), nil
}

func (r *TaskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.q.DeleteTask(ctx, id))
}
