package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// PaymentHandler talks to Razorpay.
//
// The flow:
//  1. Server creates a Razorpay order for the exact amount (Create).
//  2. The browser opens Razorpay Checkout and the customer pays.
//  3. Razorpay gives the browser a signature. The server re-computes it with
//     the SECRET key; only if it matches is the order marked paid (Verify).
//  4. As a safety net, Razorpay also calls our webhook, so an order is still
//     marked paid if the customer closes the tab right after paying (Webhook).
//
// The browser is never trusted to say "I paid". Only a valid signature counts.
type PaymentHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

const razorpayOrdersURL = "https://api.razorpay.com/v1/orders"

// sign returns the hex HMAC-SHA256 of data using secret (what Razorpay uses).
func sign(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// validSignature compares in constant time, so timing can't leak the secret.
func validSignature(secret, data, given string) bool {
	return hmac.Equal([]byte(sign(secret, data)), []byte(given))
}

// POST /api/orders/:id/payment
// Starts a payment. In demo mode it just says so, and the frontend uses /pay.
func (h *PaymentHandler) Create(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var order models.Order
	err := h.DB.Where("id = ? AND user_id = ?", id, middleware.UserID(c)).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		} else {
			respondError(c, err)
		}
		return
	}
	if order.Status != models.StatusPending {
		c.JSON(http.StatusConflict, gin.H{"error": "this order is not waiting for payment"})
		return
	}

	if !h.Cfg.PaymentsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"demo":     true,
			"amount":   order.Total,
			"currency": "INR",
		})
		return
	}

	var existing models.Payment
	err = h.DB.Where("order_id = ? AND provider = ? AND status = ?", order.ID, "razorpay", "created").
		First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"demo":              false,
			"key_id":            h.Cfg.RazorpayKeyID,
			"razorpay_order_id": existing.GatewayOrderID,
			"amount":            existing.Amount,
			"currency":          "INR",
		})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, err)
		return
	}

	rzID, err := h.createRazorpayOrder(order)
	if err != nil {
		log.Printf("razorpay create order failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "payment gateway error, please try again"})
		return
	}

	payment := models.Payment{
		OrderID: order.ID, Provider: "razorpay", GatewayOrderID: rzID,
		Amount: order.Total, Status: "created",
	}
	if err := h.DB.Create(&payment).Error; err != nil {
		respondError(c, err)
		return
	}

	// key_id is the PUBLIC key and is meant for the browser. The secret never leaves the server.
	c.JSON(http.StatusOK, gin.H{
		"demo":              false,
		"key_id":            h.Cfg.RazorpayKeyID,
		"razorpay_order_id": rzID,
		"amount":            order.Total,
		"currency":          "INR",
	})
}

func (h *PaymentHandler) createRazorpayOrder(order models.Order) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"amount":   order.Total, // paise: the amount comes from OUR database, not from the browser
		"currency": "INR",
		"receipt":  fmt.Sprintf("order_%d", order.ID),
	})
	req, err := http.NewRequest(http.MethodPost, razorpayOrdersURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(h.Cfg.RazorpayKeyID, h.Cfg.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		ID    string `json:"id"`
		Error struct {
			Description string `json:"description"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 || out.ID == "" {
		return "", fmt.Errorf("razorpay returned %d: %s", resp.StatusCode, out.Error.Description)
	}
	return out.ID, nil
}

type verifyRequest struct {
	OrderID   string `json:"razorpay_order_id" binding:"required"`
	PaymentID string `json:"razorpay_payment_id" binding:"required"`
	Signature string `json:"razorpay_signature" binding:"required"`
}

// POST /api/orders/:id/payment/verify
func (h *PaymentHandler) Verify(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil || !h.Cfg.PaymentsEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment details"})
		return
	}

	// The payment record must belong to THIS user's order.
	var payment models.Payment
	err := h.DB.Joins("JOIN orders ON orders.id = payments.order_id").
		Where("payments.gateway_order_id = ? AND payments.order_id = ? AND orders.user_id = ?",
			req.OrderID, id, middleware.UserID(c)).
		First(&payment).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	// Razorpay's rule: signature = HMAC_SHA256(order_id + "|" + payment_id, key_secret)
	if !validSignature(h.Cfg.RazorpayKeySecret, req.OrderID+"|"+req.PaymentID, req.Signature) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment signature is invalid"})
		return
	}

	paid, err := h.markPaid(req.OrderID, req.PaymentID)
	if err != nil {
		respondError(c, err)
		return
	}
	if !paid {
		c.JSON(http.StatusConflict, gin.H{"error": "this order is no longer waiting for payment"})
		return
	}

	var order models.Order
	h.DB.Preload("Items").First(&order, id)
	c.JSON(http.StatusOK, order)
}

// POST /api/webhooks/razorpay  (public: Razorpay calls it, so no login token)
// Set the webhook in the Razorpay dashboard (event: payment.captured) and put
// its secret in RAZORPAY_WEBHOOK_SECRET. Needs a public URL (e.g. ngrok) to test locally.
func (h *PaymentHandler) Webhook(c *gin.Context) {
	secret := h.Cfg.RazorpayWebhookSecret
	if secret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook not configured"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad body"})
		return
	}
	// The signature covers the RAW body, so verify before parsing anything.
	if !validSignature(secret, string(body), c.GetHeader("X-Razorpay-Signature")) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "bad signature"})
		return
	}

	var ev struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string `json:"id"`
					OrderID string `json:"order_id"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad json"})
		return
	}

	e := ev.Payload.Payment.Entity
	if (ev.Event == "payment.captured" || ev.Event == "order.paid") && e.OrderID != "" {
		_, err := h.markPaid(e.OrderID, e.ID)
		if err != nil {
			log.Printf("webhook markPaid failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "try again"}) // Razorpay will retry
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true}) // other events: acknowledge and ignore
}

// markPaid is safe to call twice (browser verify AND webhook both arrive):
// the first call flips created -> paid, the second finds nothing to do.
func (h *PaymentHandler) markPaid(gatewayOrderID, paymentID string) (bool, error) {
	paid := false
	var refundPayment *models.Payment

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Payment{}).
			Where("gateway_order_id = ? AND status = ?", gatewayOrderID, "created").
			Updates(map[string]any{"status": "paid", "gateway_payment_id": paymentID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var existing models.Payment
			if err := tx.Where("gateway_order_id = ?", gatewayOrderID).First(&existing).Error; err != nil {
				return err
			}
			var order models.Order
			if err := tx.First(&order, existing.OrderID).Error; err != nil {
				return err
			}
			if existing.Status == "paid" && order.Status == models.StatusPaid {
				paid = true
			} else if existing.Status == "refund_pending" || existing.Status == "refund_failed" {
				paid = false
			} else if existing.Status == "refunded" {
				paid = false
			} else if order.Status != models.StatusPending {
				refundPayment = &existing
			}
			return nil
		}

		var p models.Payment
		if err := tx.Where("gateway_order_id = ?", gatewayOrderID).First(&p).Error; err != nil {
			return err
		}
		res = tx.Model(&models.Order{}).
			Where("id = ? AND status = ?", p.OrderID, models.StatusPending).
			Update("status", models.StatusPaid)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			tx.Model(&models.Payment{}).
				Where("id = ?", p.ID).
				Updates(map[string]any{"status": "refund_pending", "gateway_payment_id": paymentID})
			refundPayment = &p
			return nil
		}
		paid = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if refundPayment != nil {
		if err := h.refund(refundPayment); err != nil {
			log.Printf("automatic refund failed for payment %s: %v", paymentID, err)
		}
		return false, nil
	}
	return paid, nil
}

// refund attempts a full refund when money arrived for a cancelled or otherwise
// non-pending order. Razorpay is retried only by explicit status tooling; failures
// remain visible on the payment record instead of being silently discarded.
func (h *PaymentHandler) refund(payment *models.Payment) error {
	if !h.Cfg.PaymentsEnabled() || payment.GatewayPaymentID == "" {
		return nil
	}

	payload, _ := json.Marshal(map[string]any{"amount": payment.Amount})
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.razorpay.com/v1/payments/%s/refund", payment.GatewayPaymentID),
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.SetBasicAuth(h.Cfg.RazorpayKeyID, h.Cfg.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		h.DB.Model(payment).Update("status", "refund_failed")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		h.DB.Model(payment).Update("status", "refund_failed")
		return fmt.Errorf("razorpay refund returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return h.DB.Model(payment).Update("status", "refunded").Error
}
