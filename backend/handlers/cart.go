package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/middleware"
	"shopgo/models"
)

const maxPerItem = 99

type CartHandler struct {
	DB *gorm.DB
}

type addToCartRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1,max=99"`
}

type quantityRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1,max=99"`
}

type cartResponse struct {
	Items     []models.CartItem `json:"items"`
	Total     int64             `json:"total"`      // cents
	ItemCount int               `json:"item_count"` // sum of quantities
}

// load builds the cart for a user. Totals are always computed on the server
// from current prices; the client never tells us what something costs.
func (h *CartHandler) load(userID uint) (cartResponse, error) {
	items := []models.CartItem{}
	err := h.DB.Preload("Product.Category").
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&items).Error
	if err != nil {
		return cartResponse{}, err
	}
	resp := cartResponse{Items: items}
	for _, it := range items {
		resp.Total += it.Product.Price * int64(it.Quantity)
		resp.ItemCount += it.Quantity
	}
	return resp, nil
}

func (h *CartHandler) respond(c *gin.Context, status int) {
	resp, err := h.load(middleware.UserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(status, resp)
}

// GET /api/cart
func (h *CartHandler) Get(c *gin.Context) {
	h.respond(c, http.StatusOK)
}

// POST /api/cart/items  {product_id, quantity}
// Adding a product that is already in the cart increases its quantity.
func (h *CartHandler) Add(c *gin.Context) {
	var req addToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product_id and quantity (1-99) are required"})
		return
	}
	userID := middleware.UserID(c)

	var product models.Product
	if err := h.DB.First(&product, req.ProductID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		} else {
			respondError(c, err)
		}
		return
	}

	var item models.CartItem
	err := h.DB.Where("user_id = ? AND product_id = ?", userID, req.ProductID).First(&item).Error
	exists := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, err)
		return
	}

	newQty := req.Quantity
	if exists {
		newQty += item.Quantity
	}
	if newQty > maxPerItem {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("maximum %d per item", maxPerItem)})
		return
	}
	if newQty > product.Stock {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("only %d of %q in stock", product.Stock, product.Name)})
		return
	}

	if exists {
		err = h.DB.Model(&item).Update("quantity", newQty).Error
	} else {
		err = h.DB.Create(&models.CartItem{UserID: userID, ProductID: req.ProductID, Quantity: newQty}).Error
	}
	if err != nil {
		respondError(c, err)
		return
	}
	h.respond(c, http.StatusOK)
}

// PUT /api/cart/items/:id  {quantity}      (:id is the PRODUCT id)
func (h *CartHandler) UpdateQuantity(c *gin.Context) {
	productID, ok := parseID(c)
	if !ok {
		return
	}
	var req quantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity (1-99) is required"})
		return
	}
	userID := middleware.UserID(c)

	var item models.CartItem
	err := h.DB.Preload("Product").
		Where("user_id = ? AND product_id = ?", userID, productID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "item is not in your cart"})
		} else {
			respondError(c, err)
		}
		return
	}
	if req.Quantity > item.Product.Stock {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("only %d of %q in stock", item.Product.Stock, item.Product.Name)})
		return
	}

	if err := h.DB.Model(&item).Update("quantity", req.Quantity).Error; err != nil {
		respondError(c, err)
		return
	}
	h.respond(c, http.StatusOK)
}

// DELETE /api/cart/items/:id     (:id is the PRODUCT id)
func (h *CartHandler) Remove(c *gin.Context) {
	productID, ok := parseID(c)
	if !ok {
		return
	}
	result := h.DB.Where("user_id = ? AND product_id = ?", middleware.UserID(c), productID).
		Delete(&models.CartItem{})
	if result.Error != nil {
		respondError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "item is not in your cart"})
		return
	}
	h.respond(c, http.StatusOK)
}

// DELETE /api/cart
func (h *CartHandler) Clear(c *gin.Context) {
	if err := h.DB.Where("user_id = ?", middleware.UserID(c)).Delete(&models.CartItem{}).Error; err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
