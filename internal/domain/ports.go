package domain

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Logger is the cross-cutting structured logger contract used by usecases.
// Implemented by internal/logger to keep zap out of the usecase layer.
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
	With(kv ...any) Logger
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u User) (User, error)
}

type TeamRepository interface {
	Create(ctx context.Context, name string) (Team, error)
}

type TaskFilter struct {
	TeamID uuid.UUID
	Status *TaskStatus
	Query  string
	Limit  int32
	Offset int32
}

type TaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Task, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (Task, error)
	List(ctx context.Context, f TaskFilter) (items []Task, total int64, err error)
	Create(ctx context.Context, t Task) (Task, error)
	UpdateAssignee(ctx context.Context, id, assignee uuid.UUID) (Task, error)
	Update(ctx context.Context, t Task) (Task, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type TaskLogRepository interface {
	Insert(ctx context.Context, log TaskLog) error
}

// IdempotencyAcquireResult mirrors the SQL upsert return values exactly:
//   - Acquired = true  → caller owns the lease; proceed to do the work.
//   - InFlight = true  → another caller holds an unexpired lease (return 409).
//   - Completed = true → stored response present; verify hash and replay.
type IdempotencyAcquireResult struct {
	Acquired     bool
	InFlight     bool
	Completed    bool
	StatusCode   int
	ResponseBody json.RawMessage
	StoredHash   string
}

type IdempotencyRepository interface {
	Acquire(ctx context.Context, userID, key uuid.UUID, requestHash string) (IdempotencyAcquireResult, error)
	Complete(ctx context.Context, userID, key uuid.UUID, status int, body json.RawMessage) error
}

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// TxRepos is the set of repositories bound to a single transaction (or pool
// for non-tx access). Composed in pg/uow.go and in test mocks.
type TxRepos struct {
	Users    UserRepository
	Teams    TeamRepository
	Tasks    TaskRepository
	TaskLogs TaskLogRepository
	Idem     IdempotencyRepository
	Notifier Notifier
}

// UnitOfWork hides the pgx transaction lifecycle behind a closure.
type UnitOfWork interface {
	InTx(ctx context.Context, fn func(TxRepos) error) error
	Repos() TxRepos
}
