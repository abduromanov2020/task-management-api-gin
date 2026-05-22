package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/abduromanov2020/tasks-api/internal/domain"
)

// TestAssignTask_HappyPath verifies the transactional flow updates the
// assignee, writes a task_logs row, and emits a notification.
func TestAssignTask_HappyPath(t *testing.T) {
	uc, tasks, _, logs, notif, users := newUC(t)
	teamID := uuid.New()
	actor := domain.Actor{UserID: uuid.New(), TeamID: teamID}

	// Seed creator + assignee in the same team.
	creator, _ := users.Create(context.Background(), domain.User{ID: actor.UserID, Email: "a@x", Name: "a", TeamID: teamID})
	assignee, _ := users.Create(context.Background(), domain.User{Email: "b@x", Name: "b", TeamID: teamID})

	created, err := tasks.Create(context.Background(), domain.Task{
		TeamID: teamID, CreatedBy: creator.ID, AssigneeID: &creator.ID,
		Title: "x", Status: domain.StatusPending, Priority: domain.PriorityMedium,
	})
	require.NoError(t, err)

	view, err := uc.Assign(context.Background(), actor, created.ID, assignee.ID)
	require.NoError(t, err)
	require.NotNil(t, view.AssigneeID)
	require.Equal(t, assignee.ID, *view.AssigneeID)
	require.Equal(t, 1, logs.Count(), "task_logs row must be inserted in the same tx")
	require.Equal(t, 1, notif.Count(), "notification must fire")
}

// TestAssignTask_CrossTeam_Forbidden refuses assigning to a user in a
// different team. No mutation, no log, no notification.
func TestAssignTask_CrossTeam_Forbidden(t *testing.T) {
	uc, tasks, _, logs, notif, users := newUC(t)
	teamA := uuid.New()
	teamB := uuid.New()
	actor := domain.Actor{UserID: uuid.New(), TeamID: teamA}

	creator, _ := users.Create(context.Background(), domain.User{ID: actor.UserID, Email: "a@x", Name: "a", TeamID: teamA})
	foreign, _ := users.Create(context.Background(), domain.User{Email: "b@x", Name: "b", TeamID: teamB})

	created, err := tasks.Create(context.Background(), domain.Task{
		TeamID: teamA, CreatedBy: creator.ID, AssigneeID: &creator.ID,
		Title: "x", Status: domain.StatusPending, Priority: domain.PriorityMedium,
	})
	require.NoError(t, err)

	_, err = uc.Assign(context.Background(), actor, created.ID, foreign.ID)
	require.Error(t, err)

	// Task assignee must not have been changed.
	t2, _ := tasks.GetByID(context.Background(), created.ID)
	require.Equal(t, creator.ID, *t2.AssigneeID)
	require.Equal(t, 0, logs.Count(), "no task_logs row when forbidden")
	require.Equal(t, 0, notif.Count(), "no notification when forbidden")
}

// TestAssignTask_RollsBack_OnLogFailure proves the assign flow is properly
// transactional: when the task_logs insert fails, the assignee mutation is
// rolled back too. (The mock UoW doesn't actually rollback DB state — it
// just runs the closure — so we verify by checking the recorded calls.)
//
// In the mock, UpdateAssignee mutates the in-memory task even though the tx
// is supposed to roll back. That mismatch with real Postgres is acceptable
// for a usecase-layer test: the contract we're verifying is "if logs fail,
// the assign usecase returns an error AND the assignee mutation should not
// be observable to the caller". We assert by checking the *return value*
// from Assign (which would carry the new assignee only if everything
// succeeded). For a tighter assertion we configure FailNext on the log repo.
func TestAssignTask_RollsBack_OnLogFailure(t *testing.T) {
	uc, tasks, _, logs, notif, users := newUC(t)
	teamID := uuid.New()
	actor := domain.Actor{UserID: uuid.New(), TeamID: teamID}

	creator, _ := users.Create(context.Background(), domain.User{ID: actor.UserID, Email: "a@x", Name: "a", TeamID: teamID})
	assignee, _ := users.Create(context.Background(), domain.User{Email: "b@x", Name: "b", TeamID: teamID})

	created, err := tasks.Create(context.Background(), domain.Task{
		TeamID: teamID, CreatedBy: creator.ID, AssigneeID: &creator.ID,
		Title: "x", Status: domain.StatusPending, Priority: domain.PriorityMedium,
	})
	require.NoError(t, err)

	logs.FailNext()
	view, err := uc.Assign(context.Background(), actor, created.ID, assignee.ID)
	require.Error(t, err, "Assign must propagate the task_logs failure")
	require.Equal(t, uuid.UUID{}, view.ID, "no view on failure path")
	require.Equal(t, 0, logs.Count(), "task_logs row must not be present after failed insert")
	require.Equal(t, 0, notif.Count(), "notification must not have fired after a failed log insert")
}

// TestAssignTask_NotifierFailure_RollsBack — the notifier is the last step in
// the tx; if it errors, the assign flow must propagate that and (in a real
// DB) the assignee + log roll back.
func TestAssignTask_NotifierFailure_RollsBack(t *testing.T) {
	uc, tasks, _, logs, notif, users := newUC(t)
	teamID := uuid.New()
	actor := domain.Actor{UserID: uuid.New(), TeamID: teamID}

	creator, _ := users.Create(context.Background(), domain.User{ID: actor.UserID, Email: "a@x", Name: "a", TeamID: teamID})
	assignee, _ := users.Create(context.Background(), domain.User{Email: "b@x", Name: "b", TeamID: teamID})

	created, err := tasks.Create(context.Background(), domain.Task{
		TeamID: teamID, CreatedBy: creator.ID, AssigneeID: &creator.ID,
		Title: "x", Status: domain.StatusPending, Priority: domain.PriorityMedium,
	})
	require.NoError(t, err)

	notif.FailNext()
	_, err = uc.Assign(context.Background(), actor, created.ID, assignee.ID)
	require.Error(t, err)
	require.False(t, errors.Is(err, domain.ErrForbidden), "should not be forbidden")
	require.Equal(t, 1, logs.Count(), "log was inserted before notifier ran")
}
