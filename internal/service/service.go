package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"

	"github.com/Unic-X/webhook-delivery/internal/config"
	"github.com/Unic-X/webhook-delivery/internal/models"
	"github.com/Unic-X/webhook-delivery/internal/repository"
)

// Service defines the interface for business logic
type Service interface {
	// Subscription operations
	CreateSubscription(ctx context.Context, req models.SubscriptionRequest) (models.Subscription, error)
	GetSubscription(ctx context.Context, id uuid.UUID) (models.Subscription, error)
	UpdateSubscription(ctx context.Context, id uuid.UUID, req models.SubscriptionRequest) (models.Subscription, error)
	DeleteSubscription(ctx context.Context, id uuid.UUID) error
	ListSubscriptions(ctx context.Context) ([]models.Subscription, error)

	// Webhook operations
	IngestWebhook(ctx context.Context, subscriptionID uuid.UUID, eventType string, payload json.RawMessage, signature string) error
	VerifySignature(payload []byte, signature string, secretKey string) bool

	// Delivery operations
	GetDeliveryStatus(ctx context.Context, id uuid.UUID) (models.DeliveryStatusResponse, error)
	GetRecentDeliveries(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]models.WebhookDelivery, error)
}

// WebhookService implements the Service interface
type WebhookService struct {
	repo       repository.Repository
	taskClient *asynq.Client
	cache      *cache.Cache
	config     *config.Config
	logger     *logrus.Logger
}

// NewWebhookService creates a new WebhookService
func NewWebhookService(repo repository.Repository, redisClient *redis.Client, cfg *config.Config, logger *logrus.Logger) *WebhookService {
	taskClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Initialize cache with 5 minute expiration and 10 minute cleanup interval
	c := cache.New(5*time.Minute, 10*time.Minute)

	return &WebhookService{
		repo:       repo,
		taskClient: taskClient,
		cache:      c,
		config:     cfg,
		logger:     logger,
	}
}

// CreateSubscription creates a new subscription
func (s *WebhookService) CreateSubscription(ctx context.Context, req models.SubscriptionRequest) (models.Subscription, error) {
	sub := models.Subscription{
		ID:         uuid.New(),
		TargetURL:  req.TargetURL,
		SecretKey:  req.SecretKey,
		EventTypes: models.StringArray(req.EventTypes),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.CreateSubscription(ctx, &sub); err != nil {
		s.logger.WithError(err).Error("Failed to create subscription")
		return models.Subscription{}, err
	}

	return sub, nil
}

// GetSubscription retrieves a subscription by ID
func (s *WebhookService) GetSubscription(ctx context.Context, id uuid.UUID) (models.Subscription, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("subscription:%s", id.String())
	if cached, found := s.cache.Get(cacheKey); found {
		s.logger.WithField("subscription_id", id).Debug("Subscription retrieved from cache")
		return cached.(models.Subscription), nil
	}

	sub, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("subscription_id", id).Error("Failed to get subscription")
		return models.Subscription{}, err
	}

	// Cache the result
	s.cache.Set(cacheKey, *sub, cache.DefaultExpiration)

	return *sub, nil
}

// UpdateSubscription updates an existing subscription
func (s *WebhookService) UpdateSubscription(ctx context.Context, id uuid.UUID, req models.SubscriptionRequest) (models.Subscription, error) {
	sub, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("subscription_id", id).Error("Failed to get subscription for update")
		return models.Subscription{}, err
	}

	// Update the fields
	sub.TargetURL = req.TargetURL
	sub.SecretKey = req.SecretKey
	sub.EventTypes = models.StringArray(req.EventTypes)
	sub.UpdatedAt = time.Now()

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		s.logger.WithError(err).WithField("subscription_id", id).Error("Failed to update subscription")
		return models.Subscription{}, err
	}

	// Update the cache
	cacheKey := fmt.Sprintf("subscription:%s", id.String())
	s.cache.Set(cacheKey, *sub, cache.DefaultExpiration)

	return *sub, nil
}

// DeleteSubscription deletes a subscription by ID
func (s *WebhookService) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteSubscription(ctx, id); err != nil {
		s.logger.WithError(err).WithField("subscription_id", id).Error("Failed to delete subscription")
		return err
	}

	// Remove from cache
	cacheKey := fmt.Sprintf("subscription:%s", id.String())
	s.cache.Delete(cacheKey)

	return nil
}

// ListSubscriptions returns all subscriptions
func (s *WebhookService) ListSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	subs, err := s.repo.ListSubscriptions(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list subscriptions")
		return nil, err
	}
	return subs, nil
}

// IngestWebhook ingests a webhook payload and queues it for delivery
func (s *WebhookService) IngestWebhook(ctx context.Context, subscriptionID uuid.UUID, eventType string, payload json.RawMessage, signature string) error {
	// Verify subscription exists
	sub, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		s.logger.WithError(err).WithField("subscription_id", subscriptionID).Error("Failed to get subscription for webhook ingestion")
		return err
	}

	// Check event type filtering if provided
	if eventType != "" && len(sub.EventTypes) > 0 {
		matched := false
		for _, et := range sub.EventTypes {
			if et == eventType {
				matched = true
				break
			}
		}
		if !matched {
			s.logger.WithFields(logrus.Fields{
				"subscription_id": subscriptionID,
				"event_type":      eventType,
				"allowed_types":   sub.EventTypes,
			}).Info("Event type not matched for subscription, skipping delivery")
			return nil
		}
	}

	// Verify signature if a secret key is present
	if sub.SecretKey != nil && *sub.SecretKey != "" && signature != "" {
		if !s.VerifySignature([]byte(payload), signature, *sub.SecretKey) {
			s.logger.WithField("subscription_id", subscriptionID).Warn("Invalid signature for webhook")
			return errors.New("invalid signature")
		}
	}

	// Create delivery record
	var eventTypePtr *string
	if eventType != "" {
		eventTypePtr = &eventType
	}

	delivery := models.WebhookDelivery{
		ID:             uuid.New(),
		SubscriptionID: subscriptionID,
		Payload:        payload,
		EventType:      eventTypePtr,
		CreatedAt:      time.Now(),
		Status:         models.StatusPending,
		RetryCount:     0,
		MaxRetries:     s.config.RetryLimit,
	}

	if err := s.repo.CreateWebhookDelivery(ctx, &delivery); err != nil {
		s.logger.WithError(err).WithField("subscription_id", subscriptionID).Error("Failed to create webhook delivery record")
		return err
	}

	// Queue task for processing
	task := asynq.NewTask("webhook:deliver", []byte(delivery.ID.String()))
	_, err = s.taskClient.Enqueue(task)
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", delivery.ID).Error("Failed to enqueue webhook delivery task")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"delivery_id":     delivery.ID,
		"subscription_id": subscriptionID,
	}).Info("Webhook queued for delivery")

	return nil
}

// VerifySignature verifies the HMAC-SHA256 signature of a payload
func (s *WebhookService) VerifySignature(payload []byte, signature string, secretKey string) bool {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(payload)
	expectedSignature := "sha256=" + hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// GetDeliveryStatus retrieves the status and attempts for a webhook delivery
func (s *WebhookService) GetDeliveryStatus(ctx context.Context, id uuid.UUID) (models.DeliveryStatusResponse, error) {
	delivery, err := s.repo.GetWebhookDelivery(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", id).Error("Failed to get webhook delivery")
		return models.DeliveryStatusResponse{}, err
	}

	attempts, err := s.repo.GetDeliveryAttempts(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", id).Error("Failed to get delivery attempts")
		return models.DeliveryStatusResponse{}, err
	}

	return models.DeliveryStatusResponse{
		Delivery: *delivery,
		Attempts: attempts,
	}, nil
}

// GetRecentDeliveries retrieves recent deliveries for a subscription
func (s *WebhookService) GetRecentDeliveries(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}

	deliveries, err := s.repo.GetRecentDeliveries(ctx, subscriptionID, limit)
	if err != nil {
		s.logger.WithError(err).WithField("subscription_id", subscriptionID).Error("Failed to get recent deliveries")
		return nil, err
	}

	return deliveries, nil
}

// DeliverWebhook delivers a webhook to the target URL
func (s *WebhookService) DeliverWebhook(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := s.repo.GetWebhookDelivery(ctx, deliveryID)
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to get webhook delivery for processing")
		return err
	}

	// Update status to processing
	delivery.Status = models.StatusProcessing
	if err := s.repo.UpdateWebhookDelivery(ctx, delivery); err != nil {
		s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to update webhook delivery status to processing")
		return err
	}

	// Get subscription details
	subscription, err := s.GetSubscription(ctx, delivery.SubscriptionID)
	if err != nil {
		s.logger.WithError(err).WithField("subscription_id", delivery.SubscriptionID).Error("Failed to get subscription for webhook delivery")
		return err
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.TargetURL, nil)
	if err != nil {
		s.logger.WithError(err).WithField("target_url", subscription.TargetURL).Error("Failed to create HTTP request")
		return s.handleDeliveryFailure(ctx, delivery, err, nil)
	}

	// Set payload as request body
	req.Body = http.NoBody
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(delivery.Payload)), nil
	}
	req.ContentLength = int64(len(delivery.Payload))
	req.Header.Set("Content-Type", "application/json")

	// Add event type header if present
	if delivery.EventType != nil {
		req.Header.Set("X-Webhook-Event", *delivery.EventType)
	}

	// Add delivery ID header
	req.Header.Set("X-Webhook-ID", delivery.ID.String())

	// Add signature if secret key is present
	if subscription.SecretKey != nil && *subscription.SecretKey != "" {
		h := hmac.New(sha256.New, []byte(*subscription.SecretKey))
		h.Write(delivery.Payload)
		signature := "sha256=" + hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", signature)
	}

	// Make the request
	resp, err := client.Do(req)

	// Create attempt record
	attempt := models.DeliveryAttempt{
		ID:            uuid.New(),
		DeliveryID:    deliveryID,
		AttemptNumber: delivery.RetryCount + 1,
		CreatedAt:     time.Now(),
	}

	// Handle any request errors
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to deliver webhook")
		errDetails := err.Error()
		attempt.Status = models.StatusFailed
		attempt.ErrorDetails = &errDetails

		if err := s.repo.CreateDeliveryAttempt(ctx, &attempt); err != nil {
			s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to create delivery attempt record")
		}

		return s.handleDeliveryFailure(ctx, delivery, err, nil)
	}

	// Process response
	defer resp.Body.Close()

	// Add status code to attempt
	attempt.StatusCode = &resp.StatusCode

	// Check if it's a success (2xx status code)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		attempt.Status = models.StatusSuccess

		if err := s.repo.CreateDeliveryAttempt(ctx, &attempt); err != nil {
			s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to create delivery attempt record")
		}

		// Update delivery status to delivered
		delivery.Status = models.StatusDelivered
		if err := s.repo.UpdateWebhookDelivery(ctx, delivery); err != nil {
			s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to update webhook delivery status to delivered")
		}

		s.logger.WithFields(logrus.Fields{
			"delivery_id": deliveryID,
			"status_code": resp.StatusCode,
			"attempt":     attempt.AttemptNumber,
		}).Info("Webhook delivered successfully")

		return nil
	}

	// Handle failure
	attempt.Status = models.StatusFailed
	respBody, _ := io.ReadAll(resp.Body)
	errDetails := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	attempt.ErrorDetails = &errDetails

	if err := s.repo.CreateDeliveryAttempt(ctx, &attempt); err != nil {
		s.logger.WithError(err).WithField("delivery_id", deliveryID).Error("Failed to create delivery attempt record")
	}

	s.logger.WithFields(logrus.Fields{
		"delivery_id": deliveryID,
		"status_code": resp.StatusCode,
		"attempt":     attempt.AttemptNumber,
	}).Warn("Webhook delivery failed")

	return s.handleDeliveryFailure(ctx, delivery, fmt.Errorf("HTTP %d", resp.StatusCode), &resp.StatusCode)
}

// handleDeliveryFailure handles the failure of a webhook delivery
func (s *WebhookService) handleDeliveryFailure(ctx context.Context, delivery *models.WebhookDelivery, err error, statusCode *int) error {
	delivery.RetryCount++

	// Check if max retries reached
	if delivery.RetryCount >= delivery.MaxRetries {
		s.logger.WithField("delivery_id", delivery.ID).Info("Max retries reached, marking as failed")

		delivery.Status = models.StatusFailed
		delivery.NextRetryAt = nil

		if err := s.repo.UpdateWebhookDelivery(ctx, delivery); err != nil {
			s.logger.WithError(err).WithField("delivery_id", delivery.ID).Error("Failed to update webhook delivery status to failed")
			return err
		}

		return nil
	}

	// Calculate next retry time with exponential backoff
	var delay time.Duration
	if delivery.RetryCount <= len(s.config.RetryDelays) {
		delay = s.config.RetryDelays[delivery.RetryCount-1]
	} else {
		delay = s.config.RetryDelays[len(s.config.RetryDelays)-1]
	}

	nextRetry := time.Now().Add(delay)
	delivery.Status = models.StatusPending
	delivery.NextRetryAt = &nextRetry

	if err := s.repo.UpdateWebhookDelivery(ctx, delivery); err != nil {
		s.logger.WithError(err).WithField("delivery_id", delivery.ID).Error("Failed to update webhook delivery for retry")
		return err
	}

	// Enqueue the task for the next retry
	task := asynq.NewTask("webhook:deliver", []byte(delivery.ID.String()))
	_, err = s.taskClient.Enqueue(task, asynq.ProcessIn(delay))
	if err != nil {
		s.logger.WithError(err).WithField("delivery_id", delivery.ID).Error("Failed to enqueue webhook delivery retry task")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"delivery_id": delivery.ID,
		"retry_count": delivery.RetryCount,
		"next_retry":  nextRetry,
	}).Info("Webhook scheduled for retry")

	return nil
}

// CleanupOldLogs deletes logs older than the retention period
func (s *WebhookService) CleanupOldLogs(ctx context.Context) error {
	cutoff := time.Now().Add(-time.Duration(s.config.LogRetentionHours) * time.Hour)

	count, err := s.repo.DeleteOldDeliveryAttempts(ctx, cutoff)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete old delivery attempts")
		return err
	}

	s.logger.WithField("deleted_count", count).Info("Old delivery attempts cleaned up")
	return nil
}
