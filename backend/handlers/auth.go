package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shopgo/auth"
	"shopgo/config"
	"shopgo/middleware"
	"shopgo/models"
)

// AuthHandler groups the auth endpoints and the things they depend on.
type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type registerRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt only reads 72 bytes
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "name (2+ chars), valid email and password (8+ chars) are required",
		})
		return
	}
	email := normalizeEmail(req.Email)

	var existing models.User
	err := h.DB.Where("email = ?", email).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process password"})
		return
	}

	// Role is ALWAYS customer here. Never trust a role sent by the client.
	user := models.User{
		Name:     strings.TrimSpace(req.Name),
		Email:    email,
		Password: string(hash),
		Role:     models.RoleCustomer,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
		return
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, &user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	var user models.User
	err := h.DB.Where("email = ?", normalizeEmail(req.Email)).First(&user).Error

	// Same message for "no such user" and "wrong password", so attackers
	// can't use this endpoint to discover which emails are registered.
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, &user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

// GET /api/auth/me  (protected)
func (h *AuthHandler) Me(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, middleware.UserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}
