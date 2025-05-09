package repository

import (
	"context"
	"time"

	"github.com/Unic-X/webhook-delivery/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines the interface for database operations
type Repository interface {
	// Subscription operations
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscription(ctx context.Context, id uuid.UUID) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, id uuid.UUID) error
	ListSubscriptions(ctx context.Context) ([]models.Subscription, error)

	// Webhook delivery operations
	CreateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetWebhookDelivery(ctx context.Context, id uuid.UUID) (*models.WebhookDelivery, error)
	UpdateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetPendingDeliveries(ctx context.Context, limit int) ([]models.WebhookDelivery, error)

	// Delivery attempt operations
	CreateDeliveryAttempt(ctx context.Context, attempt *models.DeliveryAttempt) error
	GetDeliveryAttempts(ctx context.Context, deliveryID uuid.UUID) ([]models.DeliveryAttempt, error)

	// Log retention
	DeleteOldDeliveryAttempts(ctx context.Context, olderThan time.Time) (int64, error)

	// Analytics
	GetRecentDeliveries(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]models.WebhookDelivery, error)
}

// PostgresRepository implements the Repository interface
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgresRepository
func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateSubscription creates a new subscription
func (r *PostgresRepository) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	query := `
		INSERT INTO subscriptions (id, target_url, secret_key, event_types, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		sub.ID, sub.TargetURL, sub.SecretKey, sub.EventTypes, sub.CreatedAt, sub.UpdatedAt)
	return err
}

// GetSubscription retrieves a subscription by ID
func (r *PostgresRepository) GetSubscription(ctx context.Context, id uuid.UUID) (*models.Subscription, error) {
	query := `SELECT * FROM subscriptions WHERE id = $1`
	var sub models.Subscription
	err := r.db.GetContext(ctx, &sub, query, id)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// UpdateSubscription updates an existing subscription
func (r *PostgresRepository) UpdateSubscription(ctx context.Context, sub *models.Subscription) error {
	query := `
		UPDATE subscriptions
		SET target_url = $1, secret_key = $2, event_types = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		sub.TargetURL, sub.SecretKey, sub.EventTypes, time.Now(), sub.ID)
	return err
}

// DeleteSubscription deletes a subscription by ID
func (r *PostgresRepository) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subscriptions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListSubscriptions returns all subscriptions
func (r *PostgresRepository) ListSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	query := `SELECT * FROM subscriptions ORDER BY created_at DESC`
	var subs []models.Subscription
	err := r.db.SelectContext(ctx, &subs, query)
	return subs, err
}

// CreateWebhookDelivery creates a new webhook delivery
func (r *PostgresRepository) CreateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (id, subscription_id, payload, event_type, created_at, status, next_retry_at, retry_count, max_retries)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		delivery.ID, delivery.SubscriptionID, delivery.Payload, delivery.EventType,
		delivery.CreatedAt, delivery.Status, delivery.NextRetryAt, delivery.RetryCount, delivery.MaxRetries)
	return err
}

// GetWebhookDelivery retrieves a webhook delivery by ID
func (r *PostgresRepository) GetWebhookDelivery(ctx context.Context, id uuid.UUID) (*models.WebhookDelivery, error) {
	query := `SELECT * FROM webhook_deliveries WHERE id = $1`
	var delivery models.WebhookDelivery
	err := r.db.GetContext(ctx, &delivery, query, id)
	if err != nil {
		return nil, err
	}
	return &delivery, nil
}

// UpdateWebhookDelivery updates an existing webhook delivery
func (r *PostgresRepository) UpdateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	query := `
		UPDATE webhook_deliveries
		SET status = $1, next_retry_at = $2, retry_count = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query,
		delivery.Status, delivery.NextRetryAt, delivery.RetryCount, delivery.ID)
	return err
}

// GetPendingDeliveries retrieves pending webhook deliveries that are due for processing
func (r *PostgresRepository) GetPendingDeliveries(ctx context.Context, limit int) ([]models.WebhookDelivery, error) {
	query := `
		SELECT * FROM webhook_deliveries
		WHERE status = $1 AND (next_retry_at IS NULL OR next_retry_at <= $2)
		ORDER BY created_at ASC
		LIMIT $3
	`
	var deliveries []models.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries, query, models.StatusPending, time.Now(), limit)
	return deliveries, err
}

// CreateDeliveryAttempt creates a new delivery attempt
func (r *PostgresRepository) CreateDeliveryAttempt(ctx context.Context, attempt *models.DeliveryAttempt) error {
	query := `
		INSERT INTO delivery_attempts (id, delivery_id, attempt_number, status, status_code, error_details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		attempt.ID, attempt.DeliveryID, attempt.AttemptNumber, attempt.Status,
		attempt.StatusCode, attempt.ErrorDetails, attempt.CreatedAt)
	return err
}

// GetDeliveryAttempts retrieves all delivery attempts for a webhook delivery
func (r *PostgresRepository) GetDeliveryAttempts(ctx context.Context, deliveryID uuid.UUID) ([]models.DeliveryAttempt, error) {
	query := `
		SELECT * FROM delivery_attempts
		WHERE delivery_id = $1
		ORDER BY attempt_number ASC
	`
	var attempts []models.DeliveryAttempt
	err := r.db.SelectContext(ctx, &attempts, query, deliveryID)
	return attempts, err
}

// DeleteOldDeliveryAttempts deletes delivery attempts older than the specified time
func (r *PostgresRepository) DeleteOldDeliveryAttempts(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM delivery_attempts WHERE created_at < $1`
	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetRecentDeliveries retrieves recent deliveries for a subscription
func (r *PostgresRepository) GetRecentDeliveries(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]models.WebhookDelivery, error) {
	query := `
		SELECT * FROM webhook_deliveries
		WHERE subscription_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	var deliveries []models.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries, query, subscriptionID, limit)
	return deliveries, err
}
