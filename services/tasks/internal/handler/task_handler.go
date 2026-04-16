package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"tasks-service/internal/service"
	"tech-ip-sem2/shared/logger"
	"tech-ip-sem2/shared/models"

	"go.uber.org/zap"
)

type TaskHandler struct {
	service *service.TaskService
	logger  *zap.Logger
}

type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DueDate     string `json:"due_date"`
}

type UpdateTaskRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

func NewTaskHandler(service *service.TaskService, parentLogger *zap.Logger) *TaskHandler {
	return &TaskHandler{
		service: service,
		logger:  parentLogger.With(zap.String("component", "handler")),
	}
}

func (h *TaskHandler) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", http.ErrNoCookie
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", http.ErrNoCookie
	}

	return parts[1], nil
}

// CreateTask обрабатывает создание новой задачи
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "CreateTask"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Invalid request format", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid request format"})
		return
	}

	// Валидация обязательных полей
	if req.Title == "" {
		log.Warn("Missing required field: title")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "title is required"})
		return
	}

	log.Debug("Creating task",
		zap.String("title", req.Title),
		zap.String("due_date", req.DueDate))

	task, err := h.service.Create(r.Context(), token, req.Title, req.Description, req.DueDate)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "token invalid"):
			fallthrough
		case strings.Contains(err.Error(), "unauthorized"):
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})

		case strings.Contains(err.Error(), "timeout"):
			fallthrough
		case strings.Contains(err.Error(), "unavailable"):
			log.Error("Auth service unavailable", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "auth service unavailable"})

		default:
			log.Error("Failed to create task", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	log.Info("Task created successfully",
		zap.String("task_id", task.ID))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// GetTasks обрабатывает получение списка всех задач
func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "GetTasks"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	tasks, err := h.service.GetAll(r.Context(), token)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "auth failed"):
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})

		case strings.Contains(err.Error(), "timeout"):
			fallthrough
		case strings.Contains(err.Error(), "unavailable"):
			log.Error("Auth service unavailable", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "auth service unavailable"})

		default:
			log.Error("Failed to get tasks", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	log.Info("Tasks retrieved successfully",
		zap.Int("count", len(tasks)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetTask обрабатывает получение задачи по ID
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "GetTask"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		log.Warn("Missing task ID")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task id is required"})
		return
	}

	log.Debug("Getting task by ID", zap.String("task_id", id))

	task, err := h.service.GetByID(r.Context(), token, id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "auth failed"):
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})

		case strings.Contains(err.Error(), "not found"):
			log.Info("Task not found", zap.String("task_id", id))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task not found"})

		case strings.Contains(err.Error(), "timeout"):
			fallthrough
		case strings.Contains(err.Error(), "unavailable"):
			log.Error("Auth service unavailable", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "auth service unavailable"})

		default:
			log.Error("Failed to get task",
				zap.String("task_id", id),
				zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	log.Info("Task retrieved successfully",
		zap.String("task_id", task.ID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// UpdateTask обрабатывает обновление задачи
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "UpdateTask"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		log.Warn("Missing task ID")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task id is required"})
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Invalid request format", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid request format"})
		return
	}

	// Проверяем, что хотя бы одно поле для обновления передано
	if req.Title == nil && req.Done == nil {
		log.Warn("No fields to update")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "no fields to update"})
		return
	}

	log.Debug("Updating task",
		zap.String("task_id", id),
		zap.Any("title", req.Title),
		zap.Any("done", req.Done))

	task, err := h.service.Update(r.Context(), token, id, req.Title, req.Done)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "auth failed"):
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})

		case strings.Contains(err.Error(), "not found"):
			log.Info("Task not found", zap.String("task_id", id))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task not found"})

		case strings.Contains(err.Error(), "timeout"):
			fallthrough
		case strings.Contains(err.Error(), "unavailable"):
			log.Error("Auth service unavailable", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "auth service unavailable"})

		default:
			log.Error("Failed to update task",
				zap.String("task_id", id),
				zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	log.Info("Task updated successfully",
		zap.String("task_id", task.ID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// DeleteTask обрабатывает удаление задачи
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "DeleteTask"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		log.Warn("Missing task ID")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task id is required"})
		return
	}

	log.Debug("Deleting task", zap.String("task_id", id))

	err = h.service.Delete(r.Context(), token, id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "auth failed"):
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})

		case strings.Contains(err.Error(), "not found"):
			log.Info("Task not found", zap.String("task_id", id))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "task not found"})

		case strings.Contains(err.Error(), "timeout"):
			fallthrough
		case strings.Contains(err.Error(), "unavailable"):
			log.Error("Auth service unavailable", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "auth service unavailable"})

		default:
			log.Error("Failed to delete task",
				zap.String("task_id", id),
				zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	log.Info("Task deleted successfully",
		zap.String("task_id", id))

	w.WriteHeader(http.StatusNoContent)
}

// SearchTasks обрабатывает поиск задач по заголовку
func (h *TaskHandler) SearchTasks(w http.ResponseWriter, r *http.Request) {
	requestID, _ := r.Context().Value(logger.RequestIDKey{}).(string)
	log := h.logger.With(
		zap.String("request_id", requestID),
		zap.String("handler", "SearchTasksVulnerable"),
	)

	token, err := h.extractToken(r)
	if err != nil {
		log.Warn("Missing or invalid authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "invalid authorization"})
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		log.Warn("Empty search query")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "title parameter is required"})
		return
	}

	log.Warn("!!! VULNERABLE ENDPOINT CALLED !!!",
		zap.String("title", title))

	tasks, err := h.service.SearchByTitle(r.Context(), token, title)
	if err != nil {
		if strings.Contains(err.Error(), "auth failed") {
			log.Warn("Authorization failed", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "unauthorized"})
			return
		}
		log.Error("Failed to search tasks", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "internal server error"})
		return
	}

	log.Warn("VULNERABLE search completed",
		zap.String("query", title),
		zap.Int("count", len(tasks)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
