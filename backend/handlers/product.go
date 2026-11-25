package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/config"
	"shopgo/models"
)

type ProductHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type productRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=200"`
	Description string `json:"description" binding:"max=5000"`
	Price       int64  `json:"price" binding:"required,gt=0"` // in cents
	Stock       int    `json:"stock" binding:"gte=0"`
	CategoryID  uint   `json:"category_id" binding:"required"`
}

// Only these ORDER BY clauses are ever used. Mapping user input to a fixed
// list (instead of pasting it into SQL) prevents SQL injection via ?sort=.
var sortOptions = map[string]string{
	"newest":     "created_at DESC, id DESC",
	"price_asc":  "price ASC, id ASC",
	"price_desc": "price DESC, id ASC",
	"name":       "name ASC, id ASC",
}

const (
	maxImageSize = 5 << 20 // 5 MB
	maxInt       = 1<<31 - 1
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// GET /api/products
// Query: q, category_id, min_price, max_price, in_stock, sort, page, limit
func (h *ProductHandler) List(c *gin.Context) {
	page := queryInt(c, "page", 1, 1, 1_000_000)
	limit := queryInt(c, "limit", 12, 1, 50)
	categoryID := queryInt(c, "category_id", 0, 0, maxInt)
	minPrice := queryInt(c, "min_price", 0, 0, maxInt)
	maxPrice := queryInt(c, "max_price", 0, 0, maxInt)
	search := strings.ToLower(strings.TrimSpace(c.Query("q")))
	inStock := c.Query("in_stock") == "true"

	// A function returning a FRESH query each time: we need the same filters
	// once for COUNT and once for the page of results.
	filtered := func() *gorm.DB {
		q := h.DB.Model(&models.Product{})
		if search != "" {
			like := "%" + search + "%"
			q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
		}
		if categoryID > 0 {
			q = q.Where("category_id = ?", categoryID)
		}
		if minPrice > 0 {
			q = q.Where("price >= ?", minPrice)
		}
		if maxPrice > 0 {
			q = q.Where("price <= ?", maxPrice)
		}
		if inStock {
			q = q.Where("stock > 0")
		}
		return q
	}

	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	order, ok := sortOptions[c.DefaultQuery("sort", "newest")]
	if !ok {
		order = sortOptions["newest"]
	}

	products := []models.Product{}
	err := filtered().
		Preload("Category").
		Order(order).
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&products).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        products,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": (int(total) + limit - 1) / limit,
	})
}

// GET /api/products/:id
func (h *ProductHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var product models.Product
	if err := h.DB.Preload("Category").First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		}
		return
	}
	c.JSON(http.StatusOK, product)
}

// POST /api/admin/products
func (h *ProductHandler) Create(c *gin.Context) {
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "name, price (cents, > 0), stock (>= 0) and category_id are required",
		})
		return
	}
	if !h.categoryExists(req.CategoryID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category does not exist"})
		return
	}

	product := models.Product{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Price:       req.Price,
		Stock:       req.Stock,
		CategoryID:  req.CategoryID,
	}
	if err := h.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create product"})
		return
	}
	h.DB.Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusCreated, product)
}

// PUT /api/admin/products/:id
func (h *ProductHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "name, price (cents, > 0), stock (>= 0) and category_id are required",
		})
		return
	}

	var product models.Product
	if err := h.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		}
		return
	}
	if !h.categoryExists(req.CategoryID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category does not exist"})
		return
	}

	product.Name = strings.TrimSpace(req.Name)
	product.Description = strings.TrimSpace(req.Description)
	product.Price = req.Price
	product.Stock = req.Stock
	product.CategoryID = req.CategoryID
	product.Category = models.Category{} // avoid GORM re-saving a stale association

	if err := h.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update product"})
		return
	}
	h.DB.Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusOK, product)
}

// DELETE /api/admin/products/:id
// Old orders are safe: order items keep their own snapshot of name and price.
func (h *ProductHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var product models.Product
	if err := h.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	// A transaction makes both deletes succeed together or not at all.
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", id).Delete(&models.CartItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&product).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete product"})
		return
	}
	h.removeImageFile(product.ImageURL)
	c.Status(http.StatusNoContent)
}

// POST /api/admin/products/:id/image   (multipart form, field name "image")
func (h *ProductHandler) UploadImage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var product models.Product
	if err := h.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	// Hard cap on how much of the request body Go will read.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize+(1<<20))

	fileHeader, err := c.FormFile("image")
	if err != nil || fileHeader.Size > maxImageSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "an 'image' file up to 5MB is required"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read file"})
		return
	}
	defer f.Close()

	// Look at the real bytes, not the filename or the client's Content-Type.
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	ext, allowed := allowedImageTypes[http.DetectContentType(head[:n])]
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only jpg, png, gif and webp images are allowed"})
		return
	}

	// Random name: no collisions and no user-controlled path.
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not name file"})
		return
	}
	name := hex.EncodeToString(raw) + ext

	if err := c.SaveUploadedFile(fileHeader, filepath.Join(h.Cfg.UploadDir, name)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save file"})
		return
	}

	oldURL := product.ImageURL
	if err := h.DB.Model(&product).Update("image_url", "/uploads/"+name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update product"})
		return
	}
	h.removeImageFile(oldURL)

	h.DB.Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) categoryExists(id uint) bool {
	var count int64
	h.DB.Model(&models.Category{}).Where("id = ?", id).Count(&count)
	return count > 0
}

// removeImageFile deletes a previously uploaded file. filepath.Base strips any
// directory parts, so a bad value in the DB can't escape the uploads folder.
func (h *ProductHandler) removeImageFile(url string) {
	if strings.HasPrefix(url, "/uploads/") {
		_ = os.Remove(filepath.Join(h.Cfg.UploadDir, filepath.Base(url)))
	}
}
