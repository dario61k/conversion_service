package middlewares

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
)

func RequestLoggerMiddleware(c *gin.Context) {
	start := time.Now()
	c.Next() // Proceed to the next handler
	duration := time.Since(start)
	fmt.Printf("Request: %s %s %s %s\n", c.Request.Method, c.Request.URL.Path, c.ClientIP(), duration) // Log the request details
}
