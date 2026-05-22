package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/domain"
	"github.com/abduromanov2020/tasks-api/internal/middleware"
	"github.com/abduromanov2020/tasks-api/internal/usecase"
)

type TaskHandler struct{ uc *usecase.TaskUsecase }

func NewTaskHandler(uc *usecase.TaskUsecase) *TaskHandler { return &TaskHandler{uc: uc} }

// Create godoc
// @Summary  Create a task (idempotent)
// @Tags     tasks
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    Idempotency-Key header string true "UUID v4"
// @Param    body body CreateTaskRequest true "task payload"
// @Success  201 {object} usecase.TaskView
// @Failure  401 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse "IDEMPOTENCY_IN_FLIGHT"
// @Failure  422 {object} ErrorResponse
// @Router   /tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	keyRaw := c.GetHeader("Idempotency-Key")
	key, err := uuid.Parse(keyRaw)
	if err != nil {
		_ = c.Error(apperr.Validation("Idempotency-Key header must be a valid UUID", err))
		return
	}
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.Validation("Invalid task payload", err))
		return
	}
	in := usecase.CreateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
		AssigneeID:  req.AssigneeID,
	}
	res, err := h.uc.Create(c.Request.Context(), middleware.ActorFromCtx(c), key, in)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.Data(res.StatusCode, "application/json; charset=utf-8", res.Body)
}

// Get godoc
// @Summary  Get a task by id (team-scoped)
// @Tags     tasks
// @Security ApiKeyAuth
// @Produce  json
// @Param    id path string true "task id"
// @Success  200 {object} usecase.TaskView
// @Failure  404 {object} ErrorResponse
// @Router   /tasks/{id} [get]
func (h *TaskHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		_ = c.Error(apperr.Validation("Invalid task id", err))
		return
	}
	view, err := h.uc.Get(c.Request.Context(), middleware.ActorFromCtx(c), id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// List godoc
// @Summary  List team tasks with filter, search, pagination
// @Tags     tasks
// @Security ApiKeyAuth
// @Produce  json
// @Param    status query string false "filter by status (pending|in_progress|done)"
// @Param    q      query string false "search by title (substring, case-insensitive)"
// @Param    page   query int    false "1-based page" default(1)
// @Param    limit  query int    false "page size (max 100)" default(10)
// @Success  200 {object} ListTasksResponse
// @Router   /tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 32)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	var statusFilter *domain.TaskStatus
	if s := c.Query("status"); s != "" {
		ts := domain.TaskStatus(s)
		if !ts.Valid() {
			_ = c.Error(apperr.Validation("status must be pending|in_progress|done", nil))
			return
		}
		statusFilter = &ts
	}
	res, err := h.uc.List(c.Request.Context(), middleware.ActorFromCtx(c),
		statusFilter, c.Query("q"), int32(page), int32(limit))
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]usecase.TaskView, 0, len(res.Items))
	for _, t := range res.Items {
		views = append(views, usecase.ToTaskView(t))
	}
	totalPages := int64(0)
	if limit > 0 {
		totalPages = (res.Total + limit - 1) / limit
	}
	c.JSON(http.StatusOK, ListTasksResponse{
		Items: views, Total: res.Total, Page: int32(page), Limit: int32(limit), TotalPages: totalPages,
	})
}

// Update godoc
// @Summary  Update a task (full replace of mutable fields)
// @Tags     tasks
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "task id"
// @Param    body body UpdateTaskRequest true "task payload"
// @Success  200 {object} usecase.TaskView
// @Failure  404 {object} ErrorResponse
// @Router   /tasks/{id} [put]
func (h *TaskHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		_ = c.Error(apperr.Validation("Invalid task id", err))
		return
	}
	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.Validation("Invalid task payload", err))
		return
	}
	view, err := h.uc.Update(c.Request.Context(), middleware.ActorFromCtx(c), id, usecase.UpdateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// Delete godoc
// @Summary  Delete a task (creator only)
// @Tags     tasks
// @Security ApiKeyAuth
// @Param    id path string true "task id"
// @Success  204
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Router   /tasks/{id} [delete]
func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		_ = c.Error(apperr.Validation("Invalid task id", err))
		return
	}
	if err := h.uc.Delete(c.Request.Context(), middleware.ActorFromCtx(c), id); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Assign godoc
// @Summary  Assign a task to another team member (transactional)
// @Tags     tasks
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "task id"
// @Param    body body AssignTaskRequest true "assignee payload"
// @Success  200 {object} usecase.TaskView
// @Failure  403 {object} ErrorResponse "cross-team assignment refused"
// @Router   /tasks/{id}/assign [post]
func (h *TaskHandler) Assign(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		_ = c.Error(apperr.Validation("Invalid task id", err))
		return
	}
	var req AssignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.Validation("Invalid assign payload", err))
		return
	}
	view, err := h.uc.Assign(c.Request.Context(), middleware.ActorFromCtx(c), id, req.AssigneeID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, view)
}
