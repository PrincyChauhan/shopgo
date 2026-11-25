package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/models"
)

type CategoryHandler struct {
	DB *gorm.DB
}

type categoryRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

// GET /api/categories (public)
func (h *CategoryHandler) List(c *gin.Context) {
	categories := []models.Category{}
	if err := h.DB.Order("name ASC").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	c.JSON(http.StatusOK, categories)
}

// POST /api/admin/categories
func (h *CategoryHandler) Create(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name (2-100 chars) is required"})
		return
	}
	name := strings.TrimSpace(req.Name)

	var count int64
	h.DB.Model(&models.Category{}).Where("LOWER(name) = ?", strings.ToLower(name)).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "category already exists"})
		return
	}

	category := models.Category{Name: name}
	if err := h.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create category"})
		return
	}
	c.JSON(http.StatusCreated, category)
}

// PUT /api/admin/categories/:id
func (h *CategoryHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name (2-100 chars) is required"})
		return
	}
	name := strings.TrimSpace(req.Name)

	var category models.Category
	if err := h.DB.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		}
		return
	}

	var dup int64
	h.DB.Model(&models.Category{}).
		Where("LOWER(name) = ? AND id <> ?", strings.ToLower(name), id).Count(&dup)
	if dup > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "another category has this name"})
		return
	}

	category.Name = name
	if err := h.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update category"})
		return
	}
	c.JSON(http.StatusOK, category)
}

// DELETE /api/admin/categories/:id
// Refused while products still use the category, so no product is left orphaned.
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var inUse int64
	h.DB.Model(&models.Product{}).Where("category_id = ?", id).Count(&inUse)
	if inUse > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "category still has products; move or delete them first"})
		return
	}

	result := h.DB.Delete(&models.Category{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete category"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
