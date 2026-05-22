package usecase

import (
	"time"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/domain"
)

// CreateTaskInput / UpdateTaskInput are framework-agnostic shapes used as
// usecase inputs. Handlers convert their bind types (with gin binding tags)
// into these. JSON tags are deliberately absent — the handler owns the wire.
type CreateTaskInput struct {
	Title       string
	Description string
	Status      string
	Priority    string
	DueDate     *time.Time
	AssigneeID  *uuid.UUID
}

type UpdateTaskInput struct {
	Title       string
	Description string
	Status      string
	Priority    string
	DueDate     *time.Time
}

// TaskView is the canonical response shape the usecase produces. It is
// json-tagged because the bytes of this struct must be exactly what the
// client receives on both fresh creates and idempotent replays.
type TaskView struct {
	ID          uuid.UUID  `json:"id"`
	TeamID      uuid.UUID  `json:"team_id"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func ToTaskView(t domain.Task) TaskView {
	return TaskView{
		ID:          t.ID,
		TeamID:      t.TeamID,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  t.AssigneeID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		DueDate:     t.DueDate,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
