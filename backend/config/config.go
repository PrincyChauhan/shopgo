package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// Config holds every setting the app needs.
// Values come from environment variables (or the backend/.env file).
type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	UploadDir      string
	AdminEmail     string
	AdminPassword  string
	TrustedProxies string

	// Razorpay (leave empty to use the free demo payment instead)
	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string

	// AI (OpenAI-compatible endpoint: works with Gemini, Groq, OpenRouter, or Ollama)
	AIBaseURL string
	AIAPIKey  string
	AIModel   string
}

func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		UploadDir:      getEnv("UPLOAD_DIR", "uploads"),
		AdminEmail:     getEnv("ADMIN_EMAIL", "admin@shopgo.local"),
		AdminPassword:  getEnv("ADMIN_PASSWORD", "admin12345"),
		TrustedProxies: getEnv("TRUSTED_PROXIES", ""),

		RazorpayKeyID:         getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),

		AIBaseURL: getEnv("AI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		AIAPIKey:  getEnv("AI_API_KEY", ""),
		AIModel:   getEnv("AI_MODEL", "gemini-2.5-flash"),
	}

	if cfg.JWTSecret == "change-me-in-production" {
		log.Println("WARNING: using default JWT_SECRET. Set JWT_SECRET before deploying.")
	}
	return cfg
}

// loadDotEnv reads KEY=VALUE lines from a .env file (if there is one) and puts
// them into the environment. Variables already set in the real environment win.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file: that's fine
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff")) // \ufeff = Windows Notepad BOM
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// PaymentsEnabled is true when real Razorpay keys are configured.
// Otherwise the app uses the free demo payment.
func (c *Config) PaymentsEnabled() bool {
	return c.RazorpayKeyID != "" && c.RazorpayKeySecret != ""
}

// AIEnabled is true when an AI key is set or a local OpenAI-compatible server is configured.
func (c *Config) AIEnabled() bool {
	if c.AIBaseURL == "" || c.AIModel == "" {
		return false
	}
	if c.AIAPIKey != "" {
		return true
	}
	return strings.Contains(c.AIBaseURL, "localhost") || strings.Contains(c.AIBaseURL, "127.0.0.1")
}
