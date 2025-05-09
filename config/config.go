package config

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func KafkaInit() (*kafka.Producer, error) {
	const (
		KafkaServer = "localhost:9092"
	)

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": KafkaServer,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	fmt.Println("Connected to Kafka Server", KafkaServer)

	return p, nil
}

