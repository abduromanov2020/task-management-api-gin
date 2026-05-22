package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/auth"
	"github.com/abduromanov2020/tasks-api/internal/domain"
)

type RegisterInput struct {
	Email    string
	Password string
	Name     string
	TeamName string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	User        domain.User
	AccessToken string
	ExpiresIn   int
}

type AuthUsecase struct {
	uow    domain.UnitOfWork
	hasher *auth.PasswordHasher
	issuer *auth.Issuer
	ttl    time.Duration
	log    domain.Logger
}

func NewAuthUsecase(uow domain.UnitOfWork, h *auth.PasswordHasher, iss *auth.Issuer, ttl time.Duration, log domain.Logger) *AuthUsecase {
	return &AuthUsecase{uow: uow, hasher: h, issuer: iss, ttl: ttl, log: log}
}

func (u *AuthUsecase) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	teamName := strings.TrimSpace(in.TeamName)
	if teamName == "" {
		teamName = email + "'s team"
	}

	var (
		created domain.User
		token   string
	)
	err := u.uow.InTx(ctx, func(r domain.TxRepos) error {
		if _, err := r.Users.GetByEmail(ctx, email); err == nil {
			return apperr.Conflict("Email already registered")
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		team, err := r.Teams.Create(ctx, teamName)
		if err != nil {
			return err
		}
		hash, err := u.hasher.Hash(in.Password)
		if err != nil {
			return err
		}
		created, err = r.Users.Create(ctx, domain.User{
			Email:        email,
			PasswordHash: hash,
			Name:         strings.TrimSpace(in.Name),
			TeamID:       team.ID,
		})
		if err != nil {
			return err
		}
		t, err := u.issuer.Issue(created.ID, created.TeamID)
		if err != nil {
			return err
		}
		token = t
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	u.log.Info("auth.register",
		"event", "auth.register",
		"user_id", created.ID,
		"team_id", created.TeamID,
	)
	return AuthResult{User: created, AccessToken: token, ExpiresIn: int(u.ttl.Seconds())}, nil
}

func (u *AuthUsecase) Login(ctx context.Context, in LoginInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	user, err := u.uow.Repos().Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return AuthResult{}, err
	}

	// Run bcrypt on a dummy hash when the user doesn't exist so the response
	// timing matches the success path; defeats email enumeration.
	hash := user.PasswordHash
	if errors.Is(err, domain.ErrNotFound) {
		hash = auth.DummyHash
	}
	cmpErr := u.hasher.Compare(hash, in.Password)
	if cmpErr != nil || errors.Is(err, domain.ErrNotFound) {
		u.log.Warn("auth.login.failed",
			"event", "auth.login.failed",
			"email_hash", emailFingerprint(email),
		)
		return AuthResult{}, apperr.Unauthorized("Invalid credentials")
	}

	tok, err := u.issuer.Issue(user.ID, user.TeamID)
	if err != nil {
		return AuthResult{}, err
	}
	u.log.Info("auth.login",
		"event", "auth.login",
		"user_id", user.ID,
		"team_id", user.TeamID,
	)
	return AuthResult{User: user, AccessToken: tok, ExpiresIn: int(u.ttl.Seconds())}, nil
}

// emailFingerprint returns a short non-reversible identifier suitable for logs.
// Keeps the full email out of structured logs while still letting operators
// correlate repeated failures from the same address.
func emailFingerprint(email string) string {
	if email == "" {
		return ""
	}
	// Take the first two and the last char before "@" plus the domain length.
	at := strings.Index(email, "@")
	if at < 1 {
		return "***"
	}
	local := email[:at]
	if len(local) <= 2 {
		return local[:1] + "**"
	}
	return string(local[0]) + "***" + string(local[len(local)-1])
}
