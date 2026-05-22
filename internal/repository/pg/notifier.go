package pg

import (
	"context"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	"github.com/abduromanov2020/tasks-api/internal/logger"
)

// LogNotifier is the mock notifier per the assignment ("boleh mock/log saja").
// It writes a structured INFO line inside the caller's transaction. If the
// transaction rolls back, the log line will already have been emitted — that
// is the intentional trade-off documented in the README. For a transactional
// outbox in production, a notifications table row would be inserted in the
// same tx; the structured log alone is acceptable for the assignment scope.
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier { return &LogNotifier{} }

func (n *LogNotifier) Notify(ctx context.Context, e domain.Notification) error {
	logger.FromCtx(ctx).Info("notification.sent",
		"event", "notification.sent",
		"kind", e.Kind,
		"user_id", e.UserID,
		"task_id", e.TaskID,
	)
	return nil
}
