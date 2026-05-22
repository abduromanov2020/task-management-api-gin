package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/domain"
)

// CreateTaskResult carries the HTTP-shape decision out of the usecase. Body
// is pre-marshaled JSON so the handler can write exactly the bytes the
// idempotency store recorded on the original request.
type CreateTaskResult struct {
	StatusCode int
	Body       json.RawMessage
}

type ListTasksResult struct {
	Items []domain.Task
	Total int64
}

type TaskUsecase struct {
	uow domain.UnitOfWork
	log domain.Logger
}

func NewTaskUsecase(uow domain.UnitOfWork, log domain.Logger) *TaskUsecase {
	return &TaskUsecase{uow: uow, log: log}
}

// --- Create with idempotency -----------------------------------------------

// Create wraps the entire claim → insert → complete cycle inside a single
// database transaction. If any step fails the whole thing (including the
// idempotency row) rolls back — no orphan in-flight rows from request-span
// crashes. The 30-second lease (set inside the Acquire SQL) is a safety net
// for harder failures (process kill, OOM) so the key becomes reclaimable.
func (u *TaskUsecase) Create(ctx context.Context, actor domain.Actor, idemKey uuid.UUID, in CreateTaskInput) (CreateTaskResult, error) {
	body, err := canonicalJSON(in)
	if err != nil {
		return CreateTaskResult{}, apperr.Validation("Invalid request body", err)
	}
	hash := sha256hex(body)

	var out CreateTaskResult
	txErr := u.uow.InTx(ctx, func(r domain.TxRepos) error {
		acq, err := r.Idem.Acquire(ctx, actor.UserID, idemKey, hash)
		if err != nil {
			return err
		}
		switch {
		case acq.InFlight:
			return domain.ErrIdemInFlight
		case acq.Completed:
			// Spec: any request with the same key within the 24h window must
			// return the original response without creating a new task,
			// regardless of body. We log a warning if the body differs from
			// the original so an operator can spot client bugs in retries.
			if acq.StoredHash != hash {
				u.log.Warn("idempotency.body_mismatch",
					"event", "idempotency.body_mismatch",
					"user_id", actor.UserID,
					"idempotency_key", idemKey,
				)
			}
			out = CreateTaskResult{StatusCode: acq.StatusCode, Body: acq.ResponseBody}
			u.log.Info("idempotency.hit",
				"event", "idempotency.hit",
				"user_id", actor.UserID,
				"idempotency_key", idemKey,
			)
			return nil
		}
		// Acquired: do the work.
		t := buildTaskFromInput(actor, in)
		created, err := r.Tasks.Create(ctx, t)
		if err != nil {
			return err
		}
		view := ToTaskView(created)
		respBytes, err := json.Marshal(view)
		if err != nil {
			return err
		}
		if err := r.Idem.Complete(ctx, actor.UserID, idemKey, 201, respBytes); err != nil {
			return err
		}
		out = CreateTaskResult{StatusCode: 201, Body: respBytes}
		u.log.Info("task.created",
			"event", "task.created",
			"task_id", created.ID,
			"user_id", actor.UserID,
			"team_id", actor.TeamID,
			"idempotency_key", idemKey,
		)
		return nil
	})
	if txErr != nil {
		return CreateTaskResult{}, txErr
	}
	return out, nil
}

// --- Read / update / delete -------------------------------------------------

func (u *TaskUsecase) Get(ctx context.Context, actor domain.Actor, id uuid.UUID) (TaskView, error) {
	t, err := u.uow.Repos().Tasks.GetByID(ctx, id)
	if err != nil {
		return TaskView{}, err
	}
	if t.TeamID != actor.TeamID {
		return TaskView{}, domain.ErrNotFound
	}
	return ToTaskView(t), nil
}

func (u *TaskUsecase) List(ctx context.Context, actor domain.Actor, status *domain.TaskStatus, q string, page, limit int32) (ListTasksResult, error) {
	limit = clampInt32(limit, 1, 100)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	if offset > 1000 {
		return ListTasksResult{}, apperr.Validation("Pagination offset is capped at 1000; use a tighter filter", nil)
	}
	items, total, err := u.uow.Repos().Tasks.List(ctx, domain.TaskFilter{
		TeamID: actor.TeamID,
		Status: status,
		Query:  strings.TrimSpace(q),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return ListTasksResult{}, err
	}
	return ListTasksResult{Items: items, Total: total}, nil
}

func (u *TaskUsecase) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateTaskInput) (TaskView, error) {
	current, err := u.uow.Repos().Tasks.GetByID(ctx, id)
	if err != nil {
		return TaskView{}, err
	}
	if current.TeamID != actor.TeamID {
		return TaskView{}, domain.ErrNotFound
	}
	current.Title = strings.TrimSpace(in.Title)
	current.Description = in.Description
	current.Status = domain.TaskStatus(in.Status)
	current.Priority = domain.TaskPriority(in.Priority)
	current.DueDate = in.DueDate
	updated, err := u.uow.Repos().Tasks.Update(ctx, current)
	if err != nil {
		return TaskView{}, err
	}
	u.log.Info("task.updated",
		"event", "task.updated",
		"task_id", updated.ID,
		"user_id", actor.UserID,
	)
	return ToTaskView(updated), nil
}

func (u *TaskUsecase) Delete(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	t, err := u.uow.Repos().Tasks.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.TeamID != actor.TeamID {
		return domain.ErrNotFound
	}
	if t.CreatedBy != actor.UserID {
		return domain.ErrForbidden
	}
	if err := u.uow.Repos().Tasks.Delete(ctx, id); err != nil {
		return err
	}
	u.log.Info("task.deleted",
		"event", "task.deleted",
		"task_id", id,
		"user_id", actor.UserID,
	)
	return nil
}

// --- Assign (transactional) ------------------------------------------------

func (u *TaskUsecase) Assign(ctx context.Context, actor domain.Actor, taskID, assigneeID uuid.UUID) (TaskView, error) {
	var out TaskView
	err := u.uow.InTx(ctx, func(r domain.TxRepos) error {
		t, err := r.Tasks.GetForUpdate(ctx, taskID)
		if err != nil {
			return err
		}
		if t.TeamID != actor.TeamID {
			return domain.ErrNotFound
		}
		assignee, err := r.Users.GetByID(ctx, assigneeID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return apperr.Validation("Assignee not found", nil)
			}
			return err
		}
		if assignee.TeamID != actor.TeamID {
			return apperr.Forbidden("Cannot assign to a user outside your team")
		}
		prevAssignee := ""
		if t.AssigneeID != nil {
			prevAssignee = t.AssigneeID.String()
		}
		updated, err := r.Tasks.UpdateAssignee(ctx, taskID, assigneeID)
		if err != nil {
			return err
		}
		if err := r.TaskLogs.Insert(ctx, domain.TaskLog{
			TaskID:  taskID,
			ActorID: actor.UserID,
			Action:  "assigned",
			Payload: map[string]any{"from": prevAssignee, "to": assigneeID.String()},
		}); err != nil {
			return err
		}
		if err := r.Notifier.Notify(ctx, domain.Notification{
			UserID: assigneeID, Kind: "task.assigned", TaskID: taskID,
		}); err != nil {
			return err
		}
		out = ToTaskView(updated)
		return nil
	})
	if err == nil {
		u.log.Info("task.assigned",
			"event", "task.assigned",
			"task_id", taskID,
			"actor_user_id", actor.UserID,
			"assignee_id", assigneeID,
		)
	}
	return out, err
}

// --- helpers ---------------------------------------------------------------

func buildTaskFromInput(actor domain.Actor, in CreateTaskInput) domain.Task {
	status := domain.TaskStatus(in.Status)
	if !status.Valid() {
		status = domain.StatusPending
	}
	priority := domain.TaskPriority(in.Priority)
	if !priority.Valid() {
		priority = domain.PriorityMedium
	}
	assignee := in.AssigneeID
	if assignee == nil {
		v := actor.UserID
		assignee = &v
	}
	return domain.Task{
		TeamID:      actor.TeamID,
		CreatedBy:   actor.UserID,
		AssigneeID:  assignee,
		Title:       strings.TrimSpace(in.Title),
		Description: in.Description,
		Status:      status,
		Priority:    priority,
		DueDate:     in.DueDate,
	}
}

func canonicalJSON(in CreateTaskInput) ([]byte, error) {
	type canon struct {
		Title       string     `json:"title"`
		Description string     `json:"description"`
		Status      string     `json:"status"`
		Priority    string     `json:"priority"`
		DueDate     *string    `json:"due_date"`
		AssigneeID  *string    `json:"assignee_id"`
	}
	var dueStr *string
	if in.DueDate != nil {
		s := in.DueDate.UTC().Format("2006-01-02T15:04:05.999999999Z")
		dueStr = &s
	}
	var asgStr *string
	if in.AssigneeID != nil {
		s := in.AssigneeID.String()
		asgStr = &s
	}
	c := canon{
		Title:       strings.TrimSpace(in.Title),
		Description: in.Description,
		Status:      in.Status,
		Priority:    in.Priority,
		DueDate:     dueStr,
		AssigneeID:  asgStr,
	}
	return json.Marshal(c)
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
