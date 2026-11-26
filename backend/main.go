package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/ai"
	"shopgo/config"
	"shopgo/database"
	"shopgo/handlers"
	"shopgo/middleware"
)

func main() {
	cfg := config.Load()

	if strings.EqualFold(os.Getenv("GIN_MODE"), "release") {
		gin.SetMode(gin.ReleaseMode)
	}

	if gin.Mode() == gin.ReleaseMode {
		if cfg.JWTSecret == "change-me-in-production" {
			log.Fatal("JWT_SECRET must be changed before running in production")
		}
		if cfg.AdminPassword == "admin12345" {
			log.Fatal("ADMIN_PASSWORD must be changed before running in production")
		}
	}

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("cannot create upload dir: %v", err)
	}

	db := database.Connect(cfg)
	database.SeedAdmin(db, cfg.AdminEmail, cfg.AdminPassword)
	database.SeedCatalog(db)

	if cfg.PaymentsEnabled() {
		log.Println("Payments: Razorpay is ON")
	} else {
		log.Println("Payments: DEMO mode (add RAZORPAY_KEY_ID / RAZORPAY_KEY_SECRET to .env for real payments)")
	}
	if cfg.AIEnabled() {
		log.Printf("AI: ON (%s)", cfg.AIModel)
	} else {
		log.Println("AI: OFF (add AI_API_KEY to .env to enable AI features)")
	}

	r := setupRouter(db, cfg)

	log.Printf("ShopGo API running on :%s", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}

func setupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default() // Logger + Recovery middleware included
	if proxies := strings.TrimSpace(cfg.TrustedProxies); proxies != "" {
		list := make([]string, 0)
		for _, proxy := range strings.Split(proxies, ",") {
			if proxy = strings.TrimSpace(proxy); proxy != "" {
				list = append(list, proxy)
			}
		}
		if err := r.SetTrustedProxies(list); err != nil {
			log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
		}
	} else {
		if err := r.SetTrustedProxies(nil); err != nil {
			log.Fatalf("cannot disable trusted proxies: %v", err)
		}
	}

	// Basic security headers on every response.
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	authH := &handlers.AuthHandler{DB: db, Cfg: cfg}
	productH := &handlers.ProductHandler{DB: db, Cfg: cfg}
	categoryH := &handlers.CategoryHandler{DB: db}
	cartH := &handlers.CartHandler{DB: db}
	orderH := &handlers.OrderHandler{DB: db, Cfg: cfg}
	payH := &handlers.PaymentHandler{DB: db, Cfg: cfg}
	statsH := &handlers.StatsHandler{DB: db}
	aiH := &handlers.AIHandler{DB: db, Client: ai.NewClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)}

	// Serve uploaded product images at /uploads/<file>
	r.Static("/uploads", cfg.UploadDir)

	api := r.Group("/api")

	// ---- Public routes ----
	api.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 10 login/register attempts per minute per IP.
	authLimit := middleware.RateLimit(10, time.Minute)
	api.POST("/auth/register", authLimit, authH.Register)
	api.POST("/auth/login", authLimit, authH.Login)

	api.GET("/products", productH.List)
	api.GET("/products/smart-search", middleware.RateLimit(20, time.Minute), aiH.SmartSearch)
	api.GET("/products/:id", productH.Get)
	api.GET("/categories", categoryH.List)
	api.POST("/webhooks/razorpay", payH.Webhook)

	// ---- Logged-in routes ----
	protected := api.Group("", middleware.AuthRequired(cfg.JWTSecret))
	protected.GET("/auth/me", authH.Me)

	protected.GET("/cart", cartH.Get)
	protected.POST("/cart/items", cartH.Add)
	protected.PUT("/cart/items/:id", cartH.UpdateQuantity)
	protected.DELETE("/cart/items/:id", cartH.Remove)
	protected.DELETE("/cart", cartH.Clear)

	protected.POST("/orders", orderH.Checkout)
	protected.GET("/orders", orderH.ListMine)
	protected.GET("/orders/:id", orderH.GetMine)
	protected.POST("/orders/:id/pay", orderH.Pay) // demo mode only
	protected.POST("/orders/:id/cancel", orderH.Cancel)
	protected.POST("/orders/:id/payment", payH.Create)
	protected.POST("/orders/:id/payment/verify", payH.Verify)

	// ---- Admin-only routes (login + admin role) ----
	admin := protected.Group("/admin", middleware.AdminOnly())
	admin.GET("/stats", statsH.Get)
	admin.GET("/stats/summary", aiH.StatsSummary)
	admin.POST("/products/draft-description", aiH.DraftDescription)
	admin.POST("/products", productH.Create)
	admin.PUT("/products/:id", productH.Update)
	admin.DELETE("/products/:id", productH.Delete)
	admin.POST("/products/:id/image", productH.UploadImage)
	admin.POST("/categories", categoryH.Create)
	admin.PUT("/categories/:id", categoryH.Update)
	admin.DELETE("/categories/:id", categoryH.Delete)
	admin.GET("/orders", orderH.ListAll)
	admin.PUT("/orders/:id/status", orderH.UpdateStatus)

	return r
}
