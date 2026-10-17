package middleware

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Auth returns a Gin middleware that validates JWT tokens.
// It injects "user_id" into the request context on success,
// and returns 401 for missing, invalid, expired, or revoked tokens.
func Auth(db *sql.DB, jwtSecret string) gin.HandlerFunc {
	secret := []byte(jwtSecret)

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		if err := checkUserActive(c.Request.Context(), db, userID); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account not found or deactivated"})
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

func checkUserActive(ctx context.Context, db *sql.DB, userID string) error {
	const q = `SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL`

	qctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var exists int
	err := db.QueryRowContext(qctx, q, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("user not found or deleted")
	}
	if err != nil {
		return fmt.Errorf("middleware.checkUserActive: %w", err)
	}
	return nil
}
