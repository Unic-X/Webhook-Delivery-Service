package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/Unic-X/webhook-delivery/internal/config"
	"github.com/Unic-X/webhook-delivery/internal/repository"
	"github.com/Unic-X/webhook-delivery/internal/service"
	"github.com/Unic-X/webhook-delivery/internal/worker"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Set up database connection
	db, err := setupDatabase(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to set up database")
	}
	defer db.Close()

	// Set up Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	// Ping Redis to check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		logger.WithError(err).Fatal("Failed to connect to Redis")
	}

	// Initialize repository
	repo := repository.NewPostgresRepository(db)

	// Initialize service
	svc := service.NewWebhookService(repo, redisClient, cfg, logger)

	// Initialize worker
	wkr := worker.NewWorker(svc, cfg, logger)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ----------------------------------------------
	// This is created for Render to run this process
	// as a Web service as Background service is paid
	// ----------------------------------------------

	router := gin.Default()

	router.HEAD("/healthz", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, "Ready") })

	server := &http.Server{
		Addr:    ":" + "9090",
		Handler: router,
	}

	go func() {
		logger.Info("Starting server on port ", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Start worker
	go func() {
		if err := wkr.Start(); err != nil {
			logger.WithError(err).Fatal("Failed to start worker")
		}
	}()

	logger.Info("Worker started")

	// Wait for interrupt signal
	<-quit

	logger.Info("Shutting down worker...")
	wkr.Shutdown()
	logger.Info("Worker shut down")
}

// setupDatabase sets up the database connection
func setupDatabase(cfg *config.Config, logger *logrus.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	logger.Info("Connected to database")
	return db, nil
}
