package main

import (
	"log"

	"github.com/Unic-X/webhook-delivery/config"
	"github.com/Unic-X/webhook-delivery/routes"
	"github.com/gin-gonic/gin"
)

func main() {
	producer, err := config.KafkaInit()
	if err != nil {
		log.Fatalf("Kafka initialization failed: %v", err)
	}
	defer producer.Close()

	router := gin.Default()

	router.POST("/injest", routes.CreateWebhookRequest(producer))

	router.Run("localhost:8080")
}
