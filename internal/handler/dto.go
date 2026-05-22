package handler

import (
	"time"

	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	"github.com/abduromanov2020/tasks-api/internal/usecase"
)

// --- Auth -------------------------------------------------------------------

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	TeamName string `json:"team_name" binding:"omitempty,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=1,max=128"`
}

type AuthResponse struct {
	User        UserResponse `json:"user"`
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
}

type UserResponse struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
	TeamID uuid.UUID `json:"team_id"`
}

func ToUserResponse(u domain.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name, TeamID: u.TeamID}
}

// --- Tasks ------------------------------------------------------------------

type CreateTaskRequest struct {
	Title       string     `json:"title" binding:"required,min=1,max=200"`
	Description string     `json:"description" binding:"max=2000"`
	Status      string     `json:"status" binding:"omitempty,oneof=pending in_progress done"`
	Priority    string     `json:"priority" binding:"omitempty,oneof=low medium high"`
	DueDate     *time.Time `json:"due_date" binding:"omitempty"`
	AssigneeID  *uuid.UUID `json:"assignee_id" binding:"omitempty"`
}

type UpdateTaskRequest struct {
	Title       string     `json:"title" binding:"required,min=1,max=200"`
	Description string     `json:"description" binding:"max=2000"`
	Status      string     `json:"status" binding:"required,oneof=pending in_progress done"`
	Priority    string     `json:"priority" binding:"required,oneof=low medium high"`
	DueDate     *time.Time `json:"due_date" binding:"omitempty"`
}

type AssignTaskRequest struct {
	AssigneeID uuid.UUID `json:"assignee_id" binding:"required"`
}

type ListTasksResponse struct {
	Items      []usecase.TaskView `json:"items"`
	Total      int64              `json:"total"`
	Page       int32              `json:"page"`
	Limit      int32              `json:"limit"`
	TotalPages int64              `json:"total_pages"`
}

// silence unused warnings for the imports above when only some types are used.
var _ time.Time
var _ uuid.UUID
var _ domain.User
