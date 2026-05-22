package pg

import (
	"context"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	sqlcdb "github.com/abduromanov2020/tasks-api/internal/repository/pg/sqlc"
)

type UserRepo struct{ q *sqlcdb.Queries }

func NewUserRepo(q *sqlcdb.Queries) *UserRepo { return &UserRepo{q: q} }

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return userFromSqlc(u), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return userFromSqlc(u), nil
}

func (r *UserRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	out, err := r.q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		TeamID:       u.TeamID,
	})
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return userFromSqlc(out), nil
}

type TeamRepo struct{ q *sqlcdb.Queries }

func NewTeamRepo(q *sqlcdb.Queries) *TeamRepo { return &TeamRepo{q: q} }

func (r *TeamRepo) Create(ctx context.Context, name string) (domain.Team, error) {
	t, err := r.q.CreateTeam(ctx, name)
	if err != nil {
		return domain.Team{}, mapErr(err)
	}
	return teamFromSqlc(t), nil
}
