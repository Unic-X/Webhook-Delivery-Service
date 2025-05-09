package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Subscription represents a webhook subscription
type Subscription struct {
	ID         uuid.UUID   `json:"id" db:"id"`
	TargetURL  string      `json:"target_url" db:"target_url"`
	SecretKey  *string     `json:"secret_key,omitempty" db:"secret_key"`
	EventTypes StringArray `json:"event_types,omitempty" db:"event_types"`
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at" db:"updated_at"`
}

// StringArray is a type for handling string arrays in PostgreSQL
type StringArray []string

// Value converts the StringArray to a PostgreSQL array
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return "{" + strings.Join(s, ",") + "}", nil
}

// Scan scans the PostgreSQL array into the StringArray
func (s *StringArray) Scan(src interface{}) error {
	if src == nil {
		*s = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		str := string(v)
		// Remove the braces
		if len(str) < 2 || str[0] != '{' || str[len(str)-1] != '}' {
			return errors.New("invalid format for StringArray")
		}
		str = str[1 : len(str)-1]
		if str == "" {
			*s = make(StringArray, 0)
			return nil
		}
		*s = strings.Split(str, ",")
		return nil
	case string:
		// Remove the braces
		if len(v) < 2 || v[0] != '{' || v[len(v)-1] != '}' {
			return errors.New("invalid format for StringArray")
		}
		v = v[1 : len(v)-1]
		if v == "" {
			*s = make(StringArray, 0)
			return nil
		}
		*s = strings.Split(v, ",")
		return nil
	default:
		return errors.New("unsupported type for StringArray")
	}
}

// WebhookDelivery represents a webhook payload to be delivered
type WebhookDelivery struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	SubscriptionID uuid.UUID       `json:"subscription_id" db:"subscription_id"`
	Payload        json.RawMessage `json:"payload" db:"payload"`
	EventType      *string         `json:"event_type,omitempty" db:"event_type"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	Status         string          `json:"status" db:"status"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty" db:"next_retry_at"`
	RetryCount     int             `json:"retry_count" db:"retry_count"`
	MaxRetries     int             `json:"max_retries" db:"max_retries"`
}

// DeliveryAttempt represents an attempt to deliver a webhook
type DeliveryAttempt struct {
	ID            uuid.UUID `json:"id" db:"id"`
	DeliveryID    uuid.UUID `json:"delivery_id" db:"delivery_id"`
	AttemptNumber int       `json:"attempt_number" db:"attempt_number"`
	Status        string    `json:"status" db:"status"`
	StatusCode    *int      `json:"status_code,omitempty" db:"status_code"`
	ErrorDetails  *string   `json:"error_details,omitempty" db:"error_details"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Constants for status values
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusDelivered  = "DELIVERED"
	StatusFailed     = "FAILED"
	StatusSuccess    = "SUCCESS"
)

// SubscriptionRequest is used for creating/updating a subscription
type SubscriptionRequest struct {
	TargetURL  string   `json:"target_url" binding:"required,url"`
	SecretKey  *string  `json:"secret_key,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
}

// WebhookRequest is used for incoming webhook payloads
type WebhookRequest struct {
	Payload json.RawMessage `json:"payload" binding:"required"`
}

// DeliveryStatusResponse contains the delivery status and attempts
type DeliveryStatusResponse struct {
	Delivery WebhookDelivery   `json:"delivery"`
	Attempts []DeliveryAttempt `json:"attempts"`
}

// DeliveryListResponse is a list of recent delivery attempts for a subscription
type DeliveryListResponse struct {
	Deliveries []WebhookDelivery `json:"deliveries"`
}
