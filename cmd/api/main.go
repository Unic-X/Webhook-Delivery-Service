package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/Unic-X/webhook-delivery/internal/api"
	"github.com/Unic-X/webhook-delivery/internal/config"
	"github.com/Unic-X/webhook-delivery/internal/repository"
	"github.com/Unic-X/webhook-delivery/internal/service"
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

	// Run database migrations
	// if err := runMigrations(cfg, logger); err != nil {
	// 	logger.WithError(err).Fatal("Failed to run database migrations")
	// }

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

	// Initialize HTTP handler
	handler := api.NewHandler(svc, logger)

	// Set up Gin router
	router := gin.Default()

	// Set up routes
	handler.SetupRoutes(router)

	// Set up HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting server on port ", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline for the shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Info("Server exiting")
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

// runMigrations runs database migrations
func runMigrations(cfg *config.Config, logger *logrus.Logger) error {
	migrationURL := "file://migrations"
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)

	logger.Info("Running database migrations")

	m, err := migrate.New(migrationURL, dbURL)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	logger.Info("Database migrations completed")
	return nil
}
