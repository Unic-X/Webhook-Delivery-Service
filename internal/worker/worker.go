package worker

import (
	"context"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"

	"github.com/Unic-X/webhook-delivery/internal/config"
	"github.com/Unic-X/webhook-delivery/internal/service"
)

// Worker handles background tasks
type Worker struct {
	server  *asynq.Server
	service *service.WebhookService
	logger  *logrus.Logger
	config  *config.Config
}

// NewWorker creates a new Worker
func NewWorker(service *service.WebhookService, cfg *config.Config, logger *logrus.Logger) *Worker {
	// Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Logger:      logger,
		},
	)

	return &Worker{
		server:  srv,
		service: service,
		logger:  logger,
		config:  cfg,
	}
}

// Start starts the worker
func (w *Worker) Start() error {
	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc("webhook:deliver", w.handleWebhookDelivery)
	mux.HandleFunc("cleanup:old_logs", w.handleCleanupOldLogs)

	// Set up periodic task for log cleanup
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     w.config.RedisAddr,
			Password: w.config.RedisPassword,
			DB:       w.config.RedisDB,
		},
		&asynq.SchedulerOpts{},
	)

	// Schedule log cleanup to run every hour
	if _, err := scheduler.Register("@every 1h", asynq.NewTask("cleanup:old_logs", nil)); err != nil {
		w.logger.WithError(err).Error("Failed to register cleanup task")
		return err
	}

	// Start the scheduler
	go func() {
		if err := scheduler.Run(); err != nil {
			w.logger.WithError(err).Fatal("Failed to start scheduler")
		}
	}()

	// Start the worker
	w.logger.Info("Starting worker with concurrency: ", w.config.WorkerConcurrency)
	return w.server.Start(mux)
}

// Shutdown gracefully shuts down the worker
func (w *Worker) Shutdown() {
	w.server.Shutdown()
	w.logger.Info("Worker shut down")
}

// handleWebhookDelivery handles the webhook delivery task
func (w *Worker) handleWebhookDelivery(ctx context.Context, task *asynq.Task) error {
	deliveryID, err := uuid.Parse(string(task.Payload()))
	if err != nil {
		w.logger.WithError(err).Error("Invalid delivery ID in task payload")
		return err
	}

	w.logger.WithField("delivery_id", deliveryID).Info("Processing webhook delivery")
	return w.service.DeliverWebhook(ctx, deliveryID)
}

// handleCleanupOldLogs handles the log cleanup task
func (w *Worker) handleCleanupOldLogs(ctx context.Context, _ *asynq.Task) error {
	w.logger.Info("Running log cleanup task")
	return w.service.CleanupOldLogs(ctx)
}
