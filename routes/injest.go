package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Subscription struct {
	ID     string `json:"id"`     //Random UUID
	URL    string `json:"URL"`    //Target URL
	Secret string `json:"Secret"` //Target Secret
}

func CreateWebhookRequest(producer *kafka.Producer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s Subscription

		if err := c.BindJSON(&s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		s.ID = uuid.New().String()
		fmt.Println(s)

		value, err := json.Marshal(s)

		topic := "webhook-subscription-v1"
		msg := &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          value,
		}

		err = producer.Produce(msg, nil)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Produced to Kafka"})
	}
}

func ReadWebhookRequest(c *gin.Context) {
	//This will get the webhook request from the kafka queue
}

func UpdateWebhookRequest(c *gin.Context) {
	//This will update the webhook request in the kafka queue
}

func DeleteWebhookRequest(c *gin.Context) {
	//This will delete the webhook request in the kafka queue
}
