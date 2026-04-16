package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tasks-service/internal/client"
	"tasks-service/internal/handler"
	"tasks-service/internal/metrics"
	"tasks-service/internal/repository"
	"tasks-service/internal/service"
	"tech-ip-sem2/shared/logger"
	"tech-ip-sem2/shared/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	port := os.Getenv("TASKS_PORT")
	if port == "" {
		port = "8082"
	}

	authGRPCAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authGRPCAddr == "" {
		authGRPCAddr = "localhost:50051"
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://tasks_user:tasks_pass@localhost:5433/tasks_db?sslmode=disable"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	zapLogger := logger.MustLogger(logger.Config{
		ServiceName: "tasks",
		Environment: env,
		LogLevel:    logLevel,
	})
	defer zapLogger.Sync()

	zapLogger.Info("DB_URL", zap.String("db_url", dbURL))

	// --- Auth client ---
	authClient, err := client.NewAuthClient(authGRPCAddr, 2*time.Second, zapLogger)
	if err != nil {
		zapLogger.Fatal("Failed to create auth client", zap.Error(err))
	}
	defer authClient.Close()

	db, err := repository.NewPostgres(dbURL)
	if err != nil {
		zapLogger.Fatal("Failed to connect DB", zap.Error(err))
	}

	repo, err := repository.NewTaskRepository(db)
	if err != nil {
		zapLogger.Fatal("Failed to init repository", zap.Error(err))
	}

	taskService := service.NewTaskService(authClient, repo, zapLogger)
	taskHandler := handler.NewTaskHandler(taskService, zapLogger)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/tasks", taskHandler.CreateTask)
	mux.HandleFunc("GET /v1/tasks", taskHandler.GetTasks)
	mux.HandleFunc("GET /v1/tasks/{id}", taskHandler.GetTask)
	mux.HandleFunc("PATCH /v1/tasks/{id}", taskHandler.UpdateTask)
	mux.HandleFunc("DELETE /v1/tasks/{id}", taskHandler.DeleteTask)
	mux.HandleFunc("GET /v1/tasks/search", taskHandler.SearchTasks)

	mux.Handle("GET /metrics", promhttp.Handler())

	handler := metrics.MetricsMiddleware(mux)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.AccessLogMiddleware(zapLogger)(handler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		zapLogger.Info("Tasks service starting",
			zap.String("port", port),
			zap.String("env", env),
			zap.String("auth_addr", authGRPCAddr),
			zap.String("db_url", dbURL),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatal("Server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Shutting down Tasks service...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		zapLogger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	zapLogger.Info("Tasks service stopped")
}
