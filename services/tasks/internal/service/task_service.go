package service

import (
	"context"
	"fmt"
	"time"

	"tasks-service/internal/client"
	"tasks-service/internal/repository"
	"tech-ip-sem2/shared/logger"
	"tech-ip-sem2/shared/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TaskService struct {
	authClient *client.AuthClient
	repo       *repository.TaskRepository
	logger     *zap.Logger
}

func NewTaskService(
	authClient *client.AuthClient,
	repo *repository.TaskRepository,
	logger *zap.Logger,
) *TaskService {
	return &TaskService{
		authClient: authClient,
		repo:       repo,
		logger:     logger.With(zap.String("component", "service")),
	}
}

func (s *TaskService) Create(ctx context.Context, token string, title, description, dueDate string) (models.Task, error) {
	requestID, _ := ctx.Value(logger.RequestIDKey{}).(string)

	log := s.logger.With(
		zap.String("request_id", requestID),
		zap.String("operation", "create"),
	)

	username, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return models.Task{}, fmt.Errorf("auth failed: %w", err)
	}

	task := models.Task{
		ID:          "t_" + uuid.New().String()[:8],
		Title:       title,
		Description: description,
		DueDate:     dueDate,
		Done:        false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return models.Task{}, err
	}

	log.Info("Task created",
		zap.String("task_id", task.ID),
		zap.String("username", username),
	)

	return task, nil
}

func (s *TaskService) GetAll(ctx context.Context, token string) ([]models.Task, error) {
	_, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	return s.repo.GetAll(ctx)
}

func (s *TaskService) GetByID(ctx context.Context, token, id string) (models.Task, error) {
	_, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return models.Task{}, fmt.Errorf("auth failed: %w", err)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return models.Task{}, fmt.Errorf("task not found")
	}

	return task, nil
}

func (s *TaskService) Update(ctx context.Context, token, id string, title *string, done *bool) (models.Task, error) {
	_, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return models.Task{}, fmt.Errorf("auth failed: %w", err)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return models.Task{}, fmt.Errorf("task not found")
	}

	if title != nil {
		task.Title = *title
	}
	if done != nil {
		task.Done = *done
	}

	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		return models.Task{}, err
	}

	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, token, id string) error {
	_, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("task not found")
	}

	return nil
}

func (s *TaskService) SearchByTitle(ctx context.Context, token, title string) ([]models.Task, error) {
	requestID, _ := ctx.Value(logger.RequestIDKey{}).(string)

	log := s.logger.With(
		zap.String("request_id", requestID),
		zap.String("operation", "search_vulnerable"),
	)

	_, err := s.authClient.VerifyToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	log.Warn("Using VULNERABLE search method! Do not use in production!",
		zap.String("title_input", title))

	tasks, err := s.repo.SearchByTitle(ctx, title)
	if err != nil {
		log.Error("Failed to search tasks (vulnerable)", zap.Error(err))
		return nil, fmt.Errorf("failed to search tasks: %w", err)
	}

	log.Info("Vulnerable search completed",
		zap.Int("count", len(tasks)),
		zap.String("search_term", title))

	return tasks, nil
}
