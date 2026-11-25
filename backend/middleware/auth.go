package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"shopgo/auth"
	"shopgo/models"
)

const (
	ctxUserID = "userID"
	ctxRole   = "role"
)

// AuthRequired checks the "Authorization: Bearer <token>" header.
// If the token is valid it stores the user id and role on the request context
// so later handlers can read them. Otherwise the request stops with 401.
func AuthRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString, found := strings.CutPrefix(header, "Bearer ")
		if !found || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		claims, err := auth.ParseToken(secret, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// AdminOnly must be used AFTER AuthRequired.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxRole)
		if r, ok := role.(models.Role); !ok || r != models.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// UserID is a helper for handlers to read the logged-in user's id.
func UserID(c *gin.Context) uint {
	id, _ := c.Get(ctxUserID)
	uid, _ := id.(uint)
	return uid
}
