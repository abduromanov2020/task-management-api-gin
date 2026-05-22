package pg

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

// mapErr turns pgx-specific errors into domain sentinels at the repository
// boundary so the usecase layer can use errors.Is without pulling in pgx.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func tsToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func ptrToTs(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func taskFromSqlc(t sqlcdb.Task) domain.Task {
	return domain.Task{
		ID:          t.ID,
		TeamID:      t.TeamID,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  t.AssigneeID,
		Title:       t.Title,
		Description: t.Description,
		Status:      domain.TaskStatus(t.Status),
		Priority:    domain.TaskPriority(t.Priority),
		DueDate:     tsToPtr(t.DueDate),
		CreatedAt:   tsToTime(t.CreatedAt),
		UpdatedAt:   tsToTime(t.UpdatedAt),
	}
}

func taskFromListRow(r sqlcdb.ListTasksRow) domain.Task {
	return domain.Task{
		ID:          r.ID,
		TeamID:      r.TeamID,
		CreatedBy:   r.CreatedBy,
		AssigneeID:  r.AssigneeID,
		Title:       r.Title,
		Description: r.Description,
		Status:      domain.TaskStatus(r.Status),
		Priority:    domain.TaskPriority(r.Priority),
		DueDate:     tsToPtr(r.DueDate),
		CreatedAt:   tsToTime(r.CreatedAt),
		UpdatedAt:   tsToTime(r.UpdatedAt),
	}
}

func userFromSqlc(u sqlcdb.User) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		TeamID:       u.TeamID,
		CreatedAt:    tsToTime(u.CreatedAt),
		UpdatedAt:    tsToTime(u.UpdatedAt),
	}
}

func teamFromSqlc(t sqlcdb.Team) domain.Team {
	return domain.Team{ID: t.ID, Name: t.Name, CreatedAt: tsToTime(t.CreatedAt)}
}

func encodePayload(p map[string]any) ([]byte, error) {
	if p == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(p)
}
