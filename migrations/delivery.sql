CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY,
    target_url TEXT NOT NULL,
    secret_key TEXT,
    event_types TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY,
    subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    event_type TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'PENDING',
    next_retry_at TIMESTAMP WITH TIME ZONE,
    retry_count INT DEFAULT 0,
    max_retries INT NOT NULL,
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('PENDING', 'PROCESSING', 'DELIVERED', 'FAILED'))
);

CREATE TABLE IF NOT EXISTS delivery_attempts (
    id UUID PRIMARY KEY,
    delivery_id UUID NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    status TEXT NOT NULL,
    status_code INT,
    error_details TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT delivery_attempts_status_check CHECK (status IN ('SUCCESS', 'FAILED'))
);

-- Indexes for performance
CREATE INDEX idx_subscriptions_event_types ON subscriptions USING GIN(event_types);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_next_retry_at ON webhook_deliveries(next_retry_at) 
    WHERE status = 'PENDING' AND next_retry_at IS NOT NULL;
CREATE INDEX idx_webhook_deliveries_subscription_id ON webhook_deliveries(subscription_id);
CREATE INDEX idx_delivery_attempts_delivery_id ON delivery_attempts(delivery_id);
CREATE INDEX idx_delivery_attempts_created_at ON delivery_attempts(created_at);
