package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	TeamID       uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Team struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

type Actor struct {
	UserID uuid.UUID
	TeamID uuid.UUID
}
