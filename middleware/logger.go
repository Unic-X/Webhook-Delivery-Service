package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func SimpleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("Before request")

		// Process request
		c.Next()

		fmt.Println("After request")
	}
}
