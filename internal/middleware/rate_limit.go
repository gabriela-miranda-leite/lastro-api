package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimit returns a Gin middleware that enforces a request rate limit per IP.
// The rate string follows the format "<limit>-<period>", e.g. "5-M" for 5 req/min.
// Valid periods: S (second), M (minute), H (hour), D (day).
func RateLimit(rate string) gin.HandlerFunc {
	r, err := limiter.NewRateFromFormatted(rate)
	if err != nil {
		// RateLimit is always called at startup with a literal string — an invalid
		// format is a programming error, not a runtime condition.
		panic("middleware.RateLimit: invalid rate format: " + rate) //nolint:forbidigo // startup-only, programming error not runtime condition
	}

	store := memory.NewStore()
	lim := limiter.New(store, r)

	return func(c *gin.Context) {
		ctx, err := lim.Get(c.Request.Context(), c.ClientIP())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if ctx.Reached {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, please try again later"})
			return
		}

		c.Next()
	}
}
