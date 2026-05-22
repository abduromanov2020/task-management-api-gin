// Package mocks contains hand-written, race-safe fakes that implement the
// domain repository ports for use in usecase unit tests. The MockIdemRepo
// in particular is designed to emulate Postgres' UNIQUE/ON CONFLICT semantics
// closely enough that the concurrency race-condition test is a meaningful
// proof: only one caller may observe Acquired=true for the same key.
package mocks

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/domain"
)

// txSnapshotter lets the fake UoW capture each repo's state at the start of a
// transaction and restore it if the closure returns an error. This mirrors
// pgx.BeginTxFunc rollback semantics closely enough that usecase-layer tests
// can verify atomicity of multi-step flows (e.g. assign = update + log + notify).
// Note: snapshotting is not safe under interleaved transactions in the same
// repo — fine for these tests because rollback paths are exercised sequentially.
type txSnapshotter interface {
	snapshot() func()
}

// --- Users ------------------------------------------------------------------

type UserRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]domain.User
	byEm map[string]domain.User
}

func NewUserRepo(seed ...domain.User) *UserRepo {
	r := &UserRepo{byID: map[uuid.UUID]domain.User{}, byEm: map[string]domain.User{}}
	for _, u := range seed {
		r.byID[u.ID] = u
		r.byEm[u.Email] = u
	}
	return r
}

func (r *UserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}
func (r *UserRepo) GetByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byEm[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}
func (r *UserRepo) snapshot() func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	byID := make(map[uuid.UUID]domain.User, len(r.byID))
	for k, v := range r.byID {
		byID[k] = v
	}
	byEm := make(map[string]domain.User, len(r.byEm))
	for k, v := range r.byEm {
		byEm[k] = v
	}
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.byID = byID
		r.byEm = byEm
	}
}

func (r *UserRepo) Create(_ context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEm[u.Email]; exists {
		return domain.User{}, domain.ErrConflict
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt
	r.byID[u.ID] = u
	r.byEm[u.Email] = u
	return u, nil
}

// --- Teams ------------------------------------------------------------------

type TeamRepo struct{}

func NewTeamRepo() *TeamRepo { return &TeamRepo{} }

func (*TeamRepo) Create(_ context.Context, name string) (domain.Team, error) {
	return domain.Team{ID: uuid.New(), Name: name, CreatedAt: time.Now().UTC()}, nil
}

// --- Tasks ------------------------------------------------------------------

type TaskRepo struct {
	mu          sync.Mutex
	tasks       map[uuid.UUID]domain.Task
	insertCount int64
	failNextOn  string // "Create", "Update", "UpdateAssignee", "Delete"
}

func NewTaskRepo() *TaskRepo { return &TaskRepo{tasks: map[uuid.UUID]domain.Task{}} }

func (r *TaskRepo) FailNextOn(op string) { r.mu.Lock(); r.failNextOn = op; r.mu.Unlock() }

func (r *TaskRepo) InsertCount() int64 { return atomic.LoadInt64(&r.insertCount) }

func (r *TaskRepo) snapshot() func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := make(map[uuid.UUID]domain.Task, len(r.tasks))
	for k, v := range r.tasks {
		snap[k] = v
	}
	snapInsert := atomic.LoadInt64(&r.insertCount)
	snapFailOn := r.failNextOn
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.tasks = snap
		atomic.StoreInt64(&r.insertCount, snapInsert)
		r.failNextOn = snapFailOn
	}
}

func (r *TaskRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return t, nil
}
func (r *TaskRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	return r.GetByID(ctx, id)
}
func (r *TaskRepo) List(_ context.Context, f domain.TaskFilter) ([]domain.Task, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Task{}
	for _, t := range r.tasks {
		if t.TeamID != f.TeamID {
			continue
		}
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		out = append(out, t)
	}
	return out, int64(len(out)), nil
}
func (r *TaskRepo) Create(_ context.Context, t domain.Task) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextOn == "Create" {
		r.failNextOn = ""
		return domain.Task{}, errFakeFailure
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt
	r.tasks[t.ID] = t
	atomic.AddInt64(&r.insertCount, 1)
	return t, nil
}
func (r *TaskRepo) UpdateAssignee(_ context.Context, id, assignee uuid.UUID) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextOn == "UpdateAssignee" {
		r.failNextOn = ""
		return domain.Task{}, errFakeFailure
	}
	t, ok := r.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	t.AssigneeID = &assignee
	t.UpdatedAt = time.Now().UTC()
	r.tasks[id] = t
	return t, nil
}
func (r *TaskRepo) Update(_ context.Context, t domain.Task) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextOn == "Update" {
		r.failNextOn = ""
		return domain.Task{}, errFakeFailure
	}
	if _, ok := r.tasks[t.ID]; !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	t.UpdatedAt = time.Now().UTC()
	r.tasks[t.ID] = t
	return t, nil
}
func (r *TaskRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextOn == "Delete" {
		r.failNextOn = ""
		return errFakeFailure
	}
	if _, ok := r.tasks[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}

// --- TaskLogs ---------------------------------------------------------------

type TaskLogRepo struct {
	mu       sync.Mutex
	logs     []domain.TaskLog
	failNext bool
}

func NewTaskLogRepo() *TaskLogRepo { return &TaskLogRepo{} }

func (r *TaskLogRepo) FailNext() { r.mu.Lock(); r.failNext = true; r.mu.Unlock() }

func (r *TaskLogRepo) Count() int { r.mu.Lock(); defer r.mu.Unlock(); return len(r.logs) }

func (r *TaskLogRepo) snapshot() func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := append([]domain.TaskLog(nil), r.logs...)
	snapFail := r.failNext
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.logs = snap
		r.failNext = snapFail
	}
}

func (r *TaskLogRepo) Insert(_ context.Context, log domain.TaskLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNext {
		r.failNext = false
		return errFakeFailure
	}
	r.logs = append(r.logs, log)
	return nil
}

// --- Notifier ---------------------------------------------------------------

type Notifier struct {
	mu       sync.Mutex
	sent     []domain.Notification
	failNext bool
}

func NewNotifier() *Notifier { return &Notifier{} }

func (n *Notifier) FailNext() { n.mu.Lock(); n.failNext = true; n.mu.Unlock() }
func (n *Notifier) Count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.sent)
}

func (n *Notifier) snapshot() func() {
	n.mu.Lock()
	defer n.mu.Unlock()
	snap := append([]domain.Notification(nil), n.sent...)
	snapFail := n.failNext
	return func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.sent = snap
		n.failNext = snapFail
	}
}
func (n *Notifier) Notify(_ context.Context, e domain.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.failNext {
		n.failNext = false
		return errFakeFailure
	}
	n.sent = append(n.sent, e)
	return nil
}

// --- Idempotency ------------------------------------------------------------

type idemKey struct{ user, key uuid.UUID }
type idemRec struct {
	hash     string
	status   int
	body     json.RawMessage
	leaseExp time.Time
}

// IdemRepo emulates Postgres' INSERT ... ON CONFLICT semantics for the
// idempotency table: any number of concurrent Acquire calls for the same
// (user, key) will see exactly one Acquired=true, and the rest will see
// either InFlight=true (lease alive, other caller still working) or
// Completed=true (Complete has been called).
type IdemRepo struct {
	mu       sync.Mutex
	data     map[idemKey]*idemRec
	leaseDur time.Duration
}

func NewIdemRepo() *IdemRepo {
	return &IdemRepo{data: map[idemKey]*idemRec{}, leaseDur: 30 * time.Second}
}

func (m *IdemRepo) Acquire(_ context.Context, user, key uuid.UUID, hash string) (domain.IdempotencyAcquireResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	k := idemKey{user, key}
	rec, ok := m.data[k]
	if !ok {
		m.data[k] = &idemRec{hash: hash, leaseExp: now.Add(m.leaseDur)}
		return domain.IdempotencyAcquireResult{Acquired: true}, nil
	}
	if rec.status == 0 && rec.leaseExp.After(now) {
		return domain.IdempotencyAcquireResult{InFlight: true, StoredHash: rec.hash}, nil
	}
	if rec.status == 0 {
		// Lease expired — reclaim.
		rec.hash = hash
		rec.leaseExp = now.Add(m.leaseDur)
		rec.body = nil
		return domain.IdempotencyAcquireResult{Acquired: true}, nil
	}
	return domain.IdempotencyAcquireResult{
		Completed:    true,
		StatusCode:   rec.status,
		ResponseBody: rec.body,
		StoredHash:   rec.hash,
	}, nil
}

func (m *IdemRepo) snapshot() func() {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := make(map[idemKey]*idemRec, len(m.data))
	for k, v := range m.data {
		rec := *v
		snap[k] = &rec
	}
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.data = snap
	}
}

func (m *IdemRepo) Complete(_ context.Context, user, key uuid.UUID, status int, body json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.data[idemKey{user, key}]
	if !ok {
		// Tests should not hit this; Complete must follow an Acquire.
		return domain.ErrNotFound
	}
	rec.status = status
	rec.body = body
	return nil
}

// --- UnitOfWork -------------------------------------------------------------

type UoW struct {
	repos domain.TxRepos
}

func NewUoW(users domain.UserRepository, teams domain.TeamRepository,
	tasks domain.TaskRepository, logs domain.TaskLogRepository,
	idem domain.IdempotencyRepository, notif domain.Notifier) *UoW {
	return &UoW{repos: domain.TxRepos{
		Users: users, Teams: teams, Tasks: tasks, TaskLogs: logs, Idem: idem, Notifier: notif,
	}}
}

// InTx models pgx.BeginTxFunc: snapshot every participating repo, run the
// closure, and restore each snapshot if the closure returns a non-nil error.
// This lets usecase-layer tests prove that a failure midway through a
// multi-step flow (assign = update + log + notify) leaves no observable
// side effect — the same atomicity contract Postgres gives us in prod.
func (u *UoW) InTx(ctx context.Context, fn func(domain.TxRepos) error) error {
	candidates := []any{u.repos.Users, u.repos.Tasks, u.repos.TaskLogs, u.repos.Notifier, u.repos.Idem}
	restorers := make([]func(), 0, len(candidates))
	for _, c := range candidates {
		if s, ok := c.(txSnapshotter); ok {
			restorers = append(restorers, s.snapshot())
		}
	}
	err := fn(u.repos)
	if err != nil {
		for _, restore := range restorers {
			restore()
		}
	}
	return err
}

func (u *UoW) Repos() domain.TxRepos { return u.repos }

// --- Logger -----------------------------------------------------------------

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

func (Logger) Debug(string, ...any)        {}
func (Logger) Info(string, ...any)         {}
func (Logger) Warn(string, ...any)         {}
func (Logger) Error(string, ...any)        {}
func (l Logger) With(...any) domain.Logger { return l }

// --- errors -----------------------------------------------------------------

var errFakeFailure = &fakeError{msg: "injected fake failure"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }
