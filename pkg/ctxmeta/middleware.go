package ctxmeta

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Middleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		userID := ""

		if v, exists := c.Get("user_id"); exists {
			userID = v.(string)
		}

		meta := Metadata{
			TraceID:       traceID,
			RequestID:     requestID,
			CorrelationID: correlationID,
			UserID:        userID,
		}

		ctx := WithMetadata(
			c.Request.Context(),
			meta,
		)

		c.Request = c.Request.WithContext(ctx)

		c.Writer.Header().Set("X-Trace-ID", traceID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Correlation-ID", correlationID)

		c.Next()
	}
}
