# Webhook Delivery Service (Segwise.ai Assignment)

 Assignment based on webhook delivery service. This service ingests incoming webhooks, queues them, and attempts delivery to subscribed target URLs, handling failures with retries and providing visibility into the delivery status.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Setup Instructions](#setup-instructions)
- [How It Works](#how-it-works)
- [API Documentation](#api-documentation)
- [Estimated AWS Pricing](#estimated-aws-pricing)
- [Development Notes](#development-notes)

## Features

- **Subscription Management**: CRUD operations for webhook subscriptions
- **Webhook Ingestion**: Quick ingestion with asynchronous processing
- **Reliable Delivery**: Background workers process queued webhooks
- **Automatic Retries**: Exponential backoff retry mechanism
- **Delivery Logging**: Comprehensive logging of all delivery attempts
- **Analytics**: Endpoints to retrieve delivery statistics and history
- **Caching**: Redis-based caching for improved performance

## Architecture

![Architecture Diagram](https://github.com/user-attachments/assets/7f7f6ef8-5e8a-42e0-9a4b-e96c0a103048)

The system consists of several components:

- **API Server**: Handles HTTP requests for subscriptions, webhook ingestion, and analytics
- **Worker**: Processes the delivery queue and handles retries
- **PostgreSQL**: Stores subscription data, webhook payloads, and delivery logs
- **Redis**: Used for caching and task queuing

## Setup Instructions

### Local Development

1. Clone the repository
   ```bash
   git clone git@github.com:Unic-X/SegwiseAssg.git
   cd SegwiseAssg
   ```

2. Update the `.env` file with your local configuration
   ```
   # Server
   PORT=8080
   ENV=development
   
   # PostgreSQL
   POSTGRES_HOST=localhost
   POSTGRES_PORT=5432
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=postgres
   POSTGRES_DB=webhook_service
   
   # Redis
   REDIS_ADDR=localhost:6379
   REDIS_PASSWORD=
   REDIS_DB=0
   
   # Worker
   WORKER_CONCURRENCY=10
   RETRY_LIMIT=5
   LOG_RETENTION_HOURS=72
   ```

3. Create the database
   ```bash
   psql -U postgres
   CREATE DATABASE webhook_service;
   \q
   ```


4. Build and run the services
   ```bash
   # Build the API server
   go build -o bin/api ./cmd/api
   
   # Build the worker
   go build -o bin/worker ./cmd/worker
   
   # Run the API server
   ./bin/api
   
   # In another terminal, run the worker
   ./bin/worker
   ```

5. Access the UI at http://localhost:8080/swagger/index.html

### Docker Deployment

1. Run with Docker Compose
   ```bash
   docker-compose up
   ```

## How It Works

### Webhook Lifecycle

1. **Subscription Creation**
   - A client creates a webhook subscription specifying a target URL
   - Optionally, a secret key can be provided for signature verification
   - Event types can be specified to filter which events the subscription receives

2. **Webhook Ingestion**
   - A client sends a webhook payload to the ingestion endpoint
   - The system validates the subscription and signature (if provided)
   - The payload is stored and queued for delivery
   - The client receives an immediate acknowledgment (202 Accepted)

3. **Webhook Processing**
   - Background workers pick up queued webhooks
   - The payload is delivered to the subscription's target URL via HTTP POST
   - Headers include delivery ID, event type, and signature (if a secret key is set)

4. **Delivery Handling**
   - If delivery succeeds (2xx response), the webhook is marked as delivered
   - If delivery fails, it's scheduled for retry with exponential backoff
   - After all retry attempts, the webhook is marked as failed if still unsuccessful

5. **Monitoring & Analytics**
   - Clients can query the delivery status of any webhook
   - Delivery attempt history, including status codes and error details, is available
   - Recent deliveries for a subscription can be retrieved

### Working with Swagger

The API is fully documented using Swagger/OpenAPI. To access the Swagger UI:

1. Start the server as described in the setup instructions
2. Open your browser and navigate to http://localhost:8080/swagger/index.html
3. Explore the API endpoints:
   - **Step 1**: Create a new subscription using the POST /subscriptions endpoint
   - **Step 2**: Send a test webhook using POST /webhooks/ingest/{subscription_id}
   - **Step 3**: Check the delivery status using GET /webhooks/deliveries/{id}
   - **Step 4**: Retrieve recent deliveries for a subscription using GET /subscriptions/{id}/deliveries

Example of creating a subscription using Swagger:
1. Click on POST /subscriptions
2. Click "Try it out"
3. Enter the subscription details:
   ```json
   {
     "target_url": "https://example.com/webhook",
     "secret_key": "your-secret-key",
     "event_types": ["order.created", "user.updated"]
   }
   ```
4. Click "Execute"
5. Copy the subscription ID from the response for use in the next steps

## API Documentation

### Subscription Management

#### Create a Subscription
```
POST /api/v1/subscriptions/
```
Request:
```json
{
  "target_url": "https://example.com/webhook",
  "secret_key": "optional-secret-key",
  "event_types": ["order.created", "user.updated"]
}
```

#### List Subscriptions
```
GET /api/v1/subscriptions/
```

#### Get a Subscription
```
GET /api/v1/subscriptions/{id}
```

#### Update a Subscription
```
PUT /api/v1/subscriptions/{id}
```
Request: Same format as create

#### Delete a Subscription
```
DELETE /api/v1/subscriptions/{id}
```

### Webhook Operations

#### Ingest a Webhook
```
POST /api/v1/webhooks/ingest/{subscription_id}
```
Headers (optional):
```
X-Event-Type: order.created
X-Hub-Signature-256: sha256=computed-hmac-signature
```
Body:
```json
{
  "payload": {
    "order_id": "12345",
    "customer": "John Doe",
    "total": 99.99
  }
}
```

#### Get Delivery Status
```
GET /api/v1/webhooks/deliveries/{id}
```

#### Get Recent Deliveries for a Subscription
```
GET /api/v1/subscriptions/{id}/deliveries
```

## Estimated AWS Pricing

Assuming a requirement of handling 100,000 webhooks per day with a maximum payload size of 5KB, here's an estimated monthly cost breakdown for AWS services:

### Compute (ECS with EC2)

- **t3.medium instances** (2 vCPU, 4 GB RAM)
- 2 instances (1 for API, 1 for worker)
- On-demand pricing: ~$0.0416 per hour per instance
- Monthly cost: ~$60.70 per instance x 2 instances = **$121.40**

### Database (RDS PostgreSQL)

- **db.t3.small** (2 vCPU, 2 GB RAM)
- Storage: 20 GB gp2 storage
- Assuming 5KB per webhook × 100,000 per day × 3 days retention = ~1.5 GB raw data
- With indexes and overhead: ~10 GB used
- Monthly cost: ~**$40.30**

### Cache (Amazon ElastiCache for Redis)

- **cache.t3.small** (1 vCPU, 1.5 GB RAM)
- Monthly cost: ~**$32.85**

### Network Traffic

- **Inbound**: Free
- **Outbound**: Assuming 100,000 webhooks × 5KB payload × 1.2 delivery attempts = ~600 MB daily
- Monthly outbound: ~18 GB
- Cost: ~**$1.62**

### Storage (EBS volumes for EC2)

- 16 GB per instance × 2 instances
- Monthly cost: ~**$3.20**

### Application Load Balancer

- 1 ALB
- Monthly cost: ~**$16.20**

### CloudWatch (Monitoring)

- Basic monitoring
- Monthly cost: ~**$10.00**

### Total Estimated Monthly Cost

Approximately **$225.57** per month

#### Scaling Considerations

This setup can handle around 100,000 webhooks per day (approximately 1.16 webhooks per second). For higher throughput:

- Increase the number of worker instances
- Scale up the Redis cache

## Development Notes

### Database Schema

The service uses three main tables:

1. **subscriptions**: Stores webhook subscription details
2. **webhook_deliveries**: Stores incoming webhooks and their delivery status
3. **delivery_attempts**: Stores individual delivery attempts, including status codes and error details

### Technologies Used

- **Go**: Core programming language
- **Gin**: Web framework
- **Asynq**: Task queue for background processing
- **PostgreSQL**: Main database
- **Redis**: Caching and task queue
- **Swagger/OpenAPI**: API documentation

### Credits

- [Gin Web Framework](https://github.com/gin-gonic/gin)
- [Asynq](https://github.com/hibiken/asynq)
- [SQLx](https://github.com/jmoiron/sqlx)
- [Go-Redis](https://github.com/go-redis/redis)
- [UUID](https://github.com/google/uuid)
- [Swagger](https://github.com/swaggo/gin-swagger)
