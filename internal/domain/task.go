package domain

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusDone:
		return true
	}
	return false
}

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

func (p TaskPriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

type Task struct {
	ID          uuid.UUID
	TeamID      uuid.UUID
	CreatedBy   uuid.UUID
	AssigneeID  *uuid.UUID
	Title       string
	Description string
	Status      TaskStatus
	Priority    TaskPriority
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskLog struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	ActorID   uuid.UUID
	Action    string
	Payload   map[string]any
	CreatedAt time.Time
}

type Notification struct {
	UserID uuid.UUID
	Kind   string
	TaskID uuid.UUID
}
