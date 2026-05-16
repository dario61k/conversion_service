package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLoggerMiddleware(c *gin.Context) { //sa
	start := time.Now()
	c.Next() // Proceed to the next handler
	duration := time.Since(start)
	fmt.Printf("Request: %s %s %s %s\n", c.Request.Method, c.Request.URL.Path, c.ClientIP(), duration) // Log the request details
}

func VerifyAccess(authURL string) gin.HandlerFunc {
	return func(c *gin.Context) {

		token := c.GetHeader("Authorization")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing token",
			})
			return
		}

		payload := map[string]string{
			"id":      c.Param("id"),
			"quality": c.Param("quality"),
		}
		if lang := c.Param("lang"); lang != "" {
			payload["lang"] = lang
		}

		body, err := json.Marshal(payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "failed to encode payload",
			})
			return
		}

		req, err := http.NewRequestWithContext(
			c.Request.Context(),
			http.MethodPost,
			authURL,
			bytes.NewBuffer(body),
		)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "failed to create request",
			})
			return
		}

		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"error": "auth service unavailable",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "access denied",
			})
			return
		}

		c.Next()
	}
}
