package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/Unic-X/webhook-delivery/internal/docs"
	"github.com/Unic-X/webhook-delivery/internal/models"
	"github.com/Unic-X/webhook-delivery/internal/service"
)

// Handler contains the API handlers and dependencies
type Handler struct {
	service service.Service
	logger  *logrus.Logger
}

// NewHandler creates a new Handler
func NewHandler(service service.Service, logger *logrus.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// SetupRoutes sets up the API routes
// @title Webhook Delivery Service API
// @version 1.0
// @description A robust webhook delivery service API
// @host localhost:8080
// @BasePath /api/v1
func (h *Handler) SetupRoutes(router *gin.Engine) {
	r := router.Group("/")
	{
		// Subscriptions
		subs := r.Group("/subscriptions")
		{
			subs.HEAD("/", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, "Ready") })
			subs.POST("/", h.CreateSubscription)
			subs.GET("/", h.ListSubscriptions)
			subs.GET("/:id", h.GetSubscription)
			subs.PUT("/:id", h.UpdateSubscription)
			subs.DELETE("/:id", h.DeleteSubscription)
			subs.GET("/:id/deliveries", h.GetSubscriptionDeliveries)
		}

		// Webhooks
		webhooks := r.Group("/webhooks")
		{
			webhooks.POST("/ingest/:subscription_id", h.IngestWebhook)
			webhooks.GET("/deliveries/:id", h.GetDeliveryStatus)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
}

// CreateSubscription creates a new webhook subscription
// @Summary Create a new webhook subscription
// @Description Create a new webhook subscription with the provided details
// @Tags subscriptions
// @Accept json
// @Produce json
// @Param subscription body models.SubscriptionRequest true "Subscription details"
// @Success 201 {object} models.Subscription
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions [post]
func (h *Handler) CreateSubscription(c *gin.Context) {
	var req models.SubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Warn("Invalid subscription request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	subscription, err := h.service.CreateSubscription(c.Request.Context(), req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create subscription")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create subscription"})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// GetSubscription retrieves a webhook subscription by ID
// @Summary Get a webhook subscription
// @Description Get a webhook subscription by its ID
// @Tags subscriptions
// @Produce json
// @Param id path string true "Subscription ID"
// @Success 200 {object} models.Subscription
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions/{id} [get]
func (h *Handler) GetSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid subscription ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid subscription ID"})
		return
	}

	subscription, err := h.service.GetSubscription(c.Request.Context(), id)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get subscription")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Subscription not found"})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// UpdateSubscription updates a webhook subscription
// @Summary Update a webhook subscription
// @Description Update a webhook subscription with the provided details
// @Tags subscriptions
// @Accept json
// @Produce json
// @Param id path string true "Subscription ID"
// @Param subscription body models.SubscriptionRequest true "Updated subscription details"
// @Success 200 {object} models.Subscription
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions/{id} [put]
func (h *Handler) UpdateSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid subscription ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid subscription ID"})
		return
	}

	var req models.SubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Warn("Invalid subscription request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	subscription, err := h.service.UpdateSubscription(c.Request.Context(), id, req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to update subscription")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Subscription not found or update failed"})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// DeleteSubscription deletes a webhook subscription
// @Summary Delete a webhook subscription
// @Description Delete a webhook subscription by its ID
// @Tags subscriptions
// @Produce json
// @Param id path string true "Subscription ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions/{id} [delete]
func (h *Handler) DeleteSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid subscription ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid subscription ID"})
		return
	}

	if err := h.service.DeleteSubscription(c.Request.Context(), id); err != nil {
		h.logger.WithError(err).Error("Failed to delete subscription")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete subscription"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListSubscriptions lists all webhook subscriptions
// @Summary List all webhook subscriptions
// @Description Get a list of all webhook subscriptions
// @Tags subscriptions
// @Produce json
// @Success 200 {array} models.Subscription
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions [get]
func (h *Handler) ListSubscriptions(c *gin.Context) {
	subscriptions, err := h.service.ListSubscriptions(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to list subscriptions")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list subscriptions"})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}

// IngestWebhook ingests a webhook for delivery
// @Summary Ingest a webhook
// @Description Ingest a webhook payload for a subscription
// @Tags webhooks
// @Accept json
// @Produce json
// @Param subscription_id path string true "Subscription ID"
// @Param X-Event-Type header string false "Event Type"
// @Param X-Hub-Signature-256 header string false "Webhook Signature"
// @Param payload body models.WebhookRequest true "Webhook payload"
// @Success 202 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhooks/ingest/{subscription_id} [post]
func (h *Handler) IngestWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscription_id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid subscription ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid subscription ID"})
		return
	}

	// Read the raw payload for signature verification
	var reqBody models.WebhookRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		h.logger.WithError(err).Warn("Invalid webhook payload")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid webhook payload"})
		return
	}

	// Get headers
	eventType := c.GetHeader("X-Event-Type")
	signature := c.GetHeader("X-Hub-Signature-256")

	// Process the webhook
	err = h.service.IngestWebhook(c.Request.Context(), id, eventType, reqBody.Payload, signature)
	if err != nil {
		h.logger.WithError(err).Error("Failed to ingest webhook")
		if err.Error() == "invalid signature" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid signature"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to process webhook"})
		return
	}

	c.JSON(http.StatusAccepted, SuccessResponse{Message: "Webhook accepted for processing"})
}

// GetDeliveryStatus gets the status of a webhook delivery
// @Summary Get webhook delivery status
// @Description Get the status and attempt history of a webhook delivery
// @Tags webhooks
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} models.DeliveryStatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhooks/deliveries/{id} [get]
func (h *Handler) GetDeliveryStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid delivery ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid delivery ID"})
		return
	}

	status, err := h.service.GetDeliveryStatus(c.Request.Context(), id)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get delivery status")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Delivery not found"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetSubscriptionDeliveries gets recent deliveries for a subscription
// @Summary Get recent deliveries
// @Description Get recent webhook deliveries for a subscription
// @Tags subscriptions
// @Produce json
// @Param id path string true "Subscription ID"
// @Param limit query int false "Limit results (default 20)"
// @Success 200 {array} models.WebhookDelivery
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /subscriptions/{id}/deliveries [get]
func (h *Handler) GetSubscriptionDeliveries(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.WithError(err).Warn("Invalid subscription ID")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid subscription ID"})
		return
	}

	// Parse limit parameter with default
	limitStr := c.DefaultQuery("limit", "20")
	var limit int
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit <= 0 {
		limit = 20
	}

	deliveries, err := h.service.GetRecentDeliveries(c.Request.Context(), id, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get recent deliveries")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get recent deliveries"})
		return
	}

	c.JSON(http.StatusOK, deliveries)
}

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse is the standard success response format
type SuccessResponse struct {
	Message string `json:"message"`
}
