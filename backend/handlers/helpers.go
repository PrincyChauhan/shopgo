package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// apiError is a business-rule failure that knows its own HTTP status.
// Code deep inside a database transaction can return one, and the handler
// turns it into the right response with respondError.
type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string { return e.Msg }

// respondError sends an apiError as-is. Anything else is an unexpected
// failure, so we return a generic 500 and never leak internal details.
func respondError(c *gin.Context, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		c.JSON(ae.Status, gin.H{"error": ae.Msg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
}

// parseID reads :id from the URL. On failure it writes a 400 and returns false.
func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}

// queryInt reads an integer query parameter and clamps it into [min, max].
// Missing or invalid values fall back to def.
func queryInt(c *gin.Context, key string, def, min, max int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
