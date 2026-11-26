package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/config"
	"shopgo/middleware"
	"shopgo/models"
)

type OrderHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type statusRequest struct {
	Status models.OrderStatus `json:"status" binding:"required,oneof=paid shipped delivered cancelled"`
}

// The order lifecycle. Anything not listed here is not allowed, so a
// delivered order can never go back to pending, for example.
var allowedTransitions = map[models.OrderStatus][]models.OrderStatus{
	models.StatusPending: {models.StatusPaid, models.StatusCancelled},
	models.StatusPaid:    {models.StatusShipped, models.StatusCancelled},
	models.StatusShipped: {models.StatusDelivered},
}

func canTransition(from, to models.OrderStatus) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// POST /api/orders   (checkout: turns the cart into an order)
//
// Everything happens inside ONE database transaction. If any step fails
// (not enough stock, DB error...) every change is rolled back, so we never
// end up with reduced stock but no order, or an order but a full cart.
func (h *OrderHandler) Checkout(c *gin.Context) {
	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please fill in all shipping details (address 3+ chars, 10-15 char phone, 6-digit pincode)"})
		return
	}
	userID := middleware.UserID(c)
	var order models.Order

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var cart []models.CartItem
		if err := tx.Preload("Product").Where("user_id = ?", userID).Find(&cart).Error; err != nil {
			return err
		}
		if len(cart) == 0 {
			return &apiError{http.StatusBadRequest, "your cart is empty"}
		}

		var total int64
		items := make([]models.OrderItem, 0, len(cart))

		for _, ci := range cart {
			// ATOMIC stock reduction. The "stock >= ?" check and the subtraction
			// happen in a single SQL statement, so two shoppers racing for the
			// last item cannot both succeed. (Reading stock in Go, checking it,
			// then writing it back WOULD be a race.)
			res := tx.Model(&models.Product{}).
				Where("id = ? AND stock >= ?", ci.ProductID, ci.Quantity).
				Update("stock", gorm.Expr("stock - ?", ci.Quantity))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return &apiError{http.StatusConflict, fmt.Sprintf("not enough stock for %q", ci.Product.Name)}
			}

			total += ci.Product.Price * int64(ci.Quantity)
			// Snapshot the name and price as they are right now.
			items = append(items, models.OrderItem{
				ProductID: ci.ProductID,
				Name:      ci.Product.Name,
				Price:     ci.Product.Price,
				Quantity:  ci.Quantity,
			})
		}

		order = models.Order{
			UserID: userID, Total: total, Status: models.StatusPending, Items: items,
			ShipName: strings.TrimSpace(req.Name), ShipPhone: strings.TrimSpace(req.Phone),
			ShipAddress: strings.TrimSpace(req.Address), ShipCity: strings.TrimSpace(req.City),
			ShipState: strings.TrimSpace(req.State), ShipPincode: req.Pincode,
		}
		if err := tx.Create(&order).Error; err != nil { // also inserts the items
			return err
		}

		return tx.Where("user_id = ?", userID).Delete(&models.CartItem{}).Error
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, order)
}

// GET /api/orders  (my orders, newest first)
func (h *OrderHandler) ListMine(c *gin.Context) {
	userID := middleware.UserID(c)
	orders, meta, err := h.page(c, func() *gorm.DB {
		return h.DB.Model(&models.Order{}).Where("user_id = ?", userID)
	})
	if err != nil {
		respondError(c, err)
		return
	}
	meta["data"] = orders
	c.JSON(http.StatusOK, meta)
}

// GET /api/orders/:id
// Someone else's order gives 404 (not 403), so ids can't be probed.
func (h *OrderHandler) GetMine(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var order models.Order
	err := h.DB.Preload("Items").
		Where("id = ? AND user_id = ?", id, middleware.UserID(c)).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		} else {
			respondError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, order)
}

type demoPayRequest struct {
	Method string `json:"method"`
}

// POST /api/orders/:id/pay
// Demo-mode payment only (disabled automatically when Razorpay keys are configured).
func (h *OrderHandler) Pay(c *gin.Context) {
	// With a real gateway configured, this free shortcut must be closed,
	// otherwise anyone could mark their own order as paid.
	if h.Cfg.PaymentsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "use online payment to pay for this order"})
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req demoPayRequest
	_ = c.ShouldBindJSON(&req) // optional body
	method := strings.ToLower(strings.TrimSpace(req.Method))
	switch method {
	case "upi", "card", "netbanking":
	default:
		method = "demo"
	}

	userID := middleware.UserID(c)
	var order *models.Order
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		order, txErr = h.changeStatusTx(tx, id, &userID, models.StatusPaid)
		if txErr != nil {
			return txErr
		}
		stamp := time.Now().UnixNano()
		return tx.Create(&models.Payment{
			OrderID:          order.ID,
			Provider:         "demo:" + method,
			GatewayOrderID:   fmt.Sprintf("demo_order_%d_%d", order.ID, stamp),
			GatewayPaymentID: fmt.Sprintf("demo_pay_%d_%d", order.ID, stamp),
			Amount:           order.Total,
			Status:           "paid",
		}).Error
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

// POST /api/orders/:id/cancel   (allowed while pending or paid)
func (h *OrderHandler) Cancel(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	userID := middleware.UserID(c)
	order, err := h.changeStatus(id, &userID, models.StatusCancelled)
	if err != nil {
		respondError(c, err)
		return
	}
	h.refundCancelledOrder(order)
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) customerTransition(c *gin.Context, to models.OrderStatus) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	userID := middleware.UserID(c)
	order, err := h.changeStatus(id, &userID, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

// ---------- Admin ----------

type adminOrderView struct {
	models.Order
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
}

// GET /api/admin/orders?status=paid&page=1&limit=10
func (h *OrderHandler) ListAll(c *gin.Context) {
	status := c.Query("status")
	orders, meta, err := h.page(c, func() *gorm.DB {
		q := h.DB.Model(&models.Order{})
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	})
	if err != nil {
		respondError(c, err)
		return
	}

	// Fetch all needed customers with ONE query instead of one per order.
	ids := make([]uint, 0, len(orders))
	seen := map[uint]bool{}
	for _, o := range orders {
		if !seen[o.UserID] {
			seen[o.UserID] = true
			ids = append(ids, o.UserID)
		}
	}
	byID := map[uint]models.User{}
	if len(ids) > 0 {
		var users []models.User
		if err := h.DB.Where("id IN ?", ids).Find(&users).Error; err != nil {
			respondError(c, err)
			return
		}
		for _, u := range users {
			byID[u.ID] = u
		}
	}

	views := make([]adminOrderView, 0, len(orders))
	for _, o := range orders {
		u := byID[o.UserID]
		views = append(views, adminOrderView{Order: o, CustomerName: u.Name, CustomerEmail: u.Email})
	}
	meta["data"] = views
	c.JSON(http.StatusOK, meta)
}

// PUT /api/admin/orders/:id/status  {status}
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req statusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be one of: paid, shipped, delivered, cancelled"})
		return
	}
	order, err := h.changeStatus(id, nil, req.Status)
	if err != nil {
		respondError(c, err)
		return
	}
	if req.Status == models.StatusCancelled {
		h.refundCancelledOrder(order)
	}
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) refundCancelledOrder(order *models.Order) {
	if !h.Cfg.PaymentsEnabled() {
		return
	}

	var payment models.Payment
	err := h.DB.Where(
		"order_id = ? AND provider = ? AND status IN ?",
		order.ID, "razorpay", []string{"paid", "refund_failed"},
	).Order("id DESC").First(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.Printf("could not find payment for cancelled order %d: %v", order.ID, err)
		return
	}
	if payment.GatewayPaymentID == "" {
		return
	}

	if err := h.DB.Model(&payment).Update("status", "refund_pending").Error; err != nil {
		log.Printf("could not mark payment %d for refund: %v", payment.ID, err)
		return
	}
	payment.Status = "refund_pending"
	paymentHandler := &PaymentHandler{DB: h.DB, Cfg: h.Cfg}
	if err := paymentHandler.refund(&payment); err != nil {
		log.Printf("automatic refund failed for payment %s: %v", payment.GatewayPaymentID, err)
	}
}

// ---------- Shared logic ----------

// changeStatus moves an order to a new status following allowedTransitions.
// ownerID is set for customers (they may only touch their own orders) and
// nil for admins. Cancelling puts the stock back.
func (h *OrderHandler) changeStatus(orderID uint, ownerID *uint, to models.OrderStatus) (*models.Order, error) {
	var order *models.Order
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		order, txErr = h.changeStatusTx(tx, orderID, ownerID, to)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (h *OrderHandler) changeStatusTx(tx *gorm.DB, orderID uint, ownerID *uint, to models.OrderStatus) (*models.Order, error) {
	var order models.Order

	q := tx.Preload("Items").Where("id = ?", orderID)
	if ownerID != nil {
		q = q.Where("user_id = ?", *ownerID)
	}
	if err := q.First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &apiError{http.StatusNotFound, "order not found"}
		}
		return nil, err
	}

	if !canTransition(order.Status, to) {
		return nil, &apiError{http.StatusConflict, fmt.Sprintf("cannot change an order from %s to %s", order.Status, to)}
	}

	// Compare-and-set: only update if the status is STILL what we read.
	// If two requests cancel the same order at once, only one wins, so
	// stock is never restored twice.
	res := tx.Model(&models.Order{}).
		Where("id = ? AND status = ?", order.ID, order.Status).
		Update("status", to)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, &apiError{http.StatusConflict, "order was just changed by someone else, please retry"}
	}

	if to == models.StatusCancelled {
		for _, it := range order.Items {
			err := tx.Model(&models.Product{}).
				Where("id = ?", it.ProductID).
				Update("stock", gorm.Expr("stock + ?", it.Quantity)).Error
			if err != nil {
				return nil, err
			}
		}
	}

	order.Status = to
	return &order, nil
}

// page runs a paginated query. scope is a function so we can build the same
// filtered query twice: once to COUNT and once to fetch the page.
func (h *OrderHandler) page(c *gin.Context, scope func() *gorm.DB) ([]models.Order, gin.H, error) {
	page := queryInt(c, "page", 1, 1, 1_000_000)
	limit := queryInt(c, "limit", 10, 1, 50)

	var total int64
	if err := scope().Count(&total).Error; err != nil {
		return nil, nil, err
	}

	orders := []models.Order{}
	err := scope().Preload("Items").
		Order("id DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&orders).Error
	if err != nil {
		return nil, nil, err
	}

	meta := gin.H{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": (int(total) + limit - 1) / limit,
	}
	return orders, meta, nil
}

type checkoutRequest struct {
	Name    string `json:"name" binding:"required,min=2,max=100"`
	Phone   string `json:"phone" binding:"required,min=10,max=15"`
	Address string `json:"address" binding:"required,min=3,max=300"`
	City    string `json:"city" binding:"required,min=2,max=100"`
	State   string `json:"state" binding:"required,min=2,max=100"`
	Pincode string `json:"pincode" binding:"required,len=6,numeric"`
}
