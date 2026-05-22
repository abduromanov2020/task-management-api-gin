package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/usecase"
)

type AuthHandler struct{ uc *usecase.AuthUsecase }

func NewAuthHandler(uc *usecase.AuthUsecase) *AuthHandler { return &AuthHandler{uc: uc} }

// Register godoc
// @Summary  Register a new user (creates a team if team_name is empty)
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body RegisterRequest true "registration payload"
// @Success  201 {object} AuthResponse
// @Failure  409 {object} ErrorResponse
// @Failure  422 {object} ErrorResponse
// @Router   /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.Validation("Invalid registration payload", err))
		return
	}
	res, err := h.uc.Register(c.Request.Context(), usecase.RegisterInput{
		Email: req.Email, Password: req.Password, Name: req.Name, TeamName: req.TeamName,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, AuthResponse{
		User:        ToUserResponse(res.User),
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   res.ExpiresIn,
	})
}

// Login godoc
// @Summary  Log in and receive a JWT
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body LoginRequest true "credentials"
// @Success  200 {object} AuthResponse
// @Failure  401 {object} ErrorResponse
// @Failure  422 {object} ErrorResponse
// @Router   /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.Validation("Invalid login payload", err))
		return
	}
	res, err := h.uc.Login(c.Request.Context(), usecase.LoginInput{
		Email: req.Email, Password: req.Password,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, AuthResponse{
		User:        ToUserResponse(res.User),
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   res.ExpiresIn,
	})
}

// ErrorResponse is the canonical error envelope (declared here for swag).
type ErrorResponse struct {
	Status    string         `json:"status"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Timestamp string         `json:"timestamp"`
	RequestID string         `json:"request_id"`
}
