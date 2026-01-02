package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Uuenu/taskchat/internal/errors"
	"github.com/Uuenu/taskchat/internal/service"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	userEmailKey        = "userEmail"
)

// AuthMiddleware creates JWT authentication middleware
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{Error: "authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, bearerPrefix) {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{Error: "invalid authorization format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
		email, err := authService.ValidateToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			message := "invalid token"

			if err == errors.ErrTokenExpired {
				message = "token expired"
			}

			c.JSON(status, errors.ErrorResponse{Error: message})
			c.Abort()
			return
		}

		c.Set(userEmailKey, email)
		c.Next()
	}
}

// GetUserEmail extracts user email from context
func GetUserEmail(c *gin.Context) string {
	if email, exists := c.Get(userEmailKey); exists {
		return email.(string)
	}
	return ""
}

// CORS middleware for development
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
