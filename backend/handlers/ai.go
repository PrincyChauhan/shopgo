package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/ai"
	"shopgo/models"
)

type AIHandler struct {
	DB     *gorm.DB
	Client *ai.Client
}

type draftDescriptionRequest struct {
	Name       string  `json:"name" binding:"required,min=2,max=200"`
	CategoryID uint    `json:"category_id"`
	PriceINR   float64 `json:"price_inr"`
}

// POST /api/admin/products/draft-description
func (h *AIHandler) DraftDescription(c *gin.Context) {
	if h.Client == nil || !h.Client.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI is not configured. Set AI_API_KEY in backend/.env."})
		return
	}

	var req draftDescriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product name (at least 2 characters) is required"})
		return
	}

	catName := "General"
	if req.CategoryID > 0 {
		var cat models.Category
		if err := h.DB.First(&cat, req.CategoryID).Error; err == nil {
			catName = cat.Name
		}
	}

	priceNote := ""
	if req.PriceINR > 0 {
		priceNote = fmt.Sprintf(", priced at ₹%.2f", req.PriceINR)
	}

	systemPrompt := "You write concise, clear, factual e-commerce product descriptions. Return only plain text (2 short sentences, max 300 characters). Do not invent specific technical specs or warranty claims not implied by the title."
	userPrompt := fmt.Sprintf("Product: %s\nCategory: %s%s\nWrite a helpful store product description.", strings.TrimSpace(req.Name), catName, priceNote)

	desc, err := h.Client.Chat(c.Request.Context(), systemPrompt, userPrompt, 160)
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI is not configured. Set AI_API_KEY in backend/.env."})
			return
		}
		log.Printf("AI DraftDescription error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"description": strings.Trim(desc, `"`)})
}

// GET /api/admin/stats/summary
func (h *AIHandler) StatsSummary(c *gin.Context) {
	if h.Client == nil || !h.Client.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI is not configured. Set AI_API_KEY in backend/.env."})
		return
	}

	stats, err := computeStats(h.DB)
	if err != nil {
		respondError(c, err)
		return
	}

	lowItems := make([]string, 0, len(stats.LowStock))
	for _, p := range stats.LowStock {
		lowItems = append(lowItems, fmt.Sprintf("%s (%d left)", p.Name, p.Stock))
	}

	payload, _ := json.Marshal(gin.H{
		"revenue_inr":      float64(stats.Revenue) / 100.0,
		"orders_total":     stats.OrdersTotal,
		"orders_by_status": stats.OrdersByStatus,
		"customers":        stats.Customers,
		"products":         stats.Products,
		"low_stock_items":  lowItems,
	})

	systemPrompt := "You summarize store metrics for an e-commerce store owner. Write 2-3 concise, actionable sentences in plain text. Format currency as ₹. Only cite numbers present in the JSON."
	userPrompt := "Store metrics JSON:\n" + string(payload)

	summary, err := h.Client.Chat(c.Request.Context(), systemPrompt, userPrompt, 200)
	if err != nil {
		log.Printf("AI StatsSummary error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

type smartSearchParsed struct {
	Q           string `json:"q"`
	CategoryID  uint   `json:"category_id"`
	MinPriceINR int64  `json:"min_price_inr"`
	MaxPriceINR int64  `json:"max_price_inr"`
	InStock     bool   `json:"in_stock"`
	Sort        string `json:"sort"`
}

// GET /api/products/smart-search?q=...
// Parses natural-language shopping queries into the filter params that
// GET /api/products accepts. Falls back gracefully to plain keyword search
// if AI is unconfigured or returns invalid output.
func (h *AIHandler) SmartSearch(c *gin.Context) {
	rawQuery := strings.TrimSpace(c.Query("q"))
	if rawQuery == "" {
		c.JSON(http.StatusOK, gin.H{
			"q": "", "category_id": 0, "min_price": 0, "max_price": 0,
			"in_stock": false, "sort": "newest", "ai_used": false,
		})
		return
	}

	fallback := func() {
		c.JSON(http.StatusOK, gin.H{
			"q":           rawQuery,
			"category_id": 0,
			"min_price":   0,
			"max_price":   0,
			"in_stock":    false,
			"sort":        "newest",
			"ai_used":     false,
		})
	}

	if h.Client == nil || !h.Client.Enabled() {
		fallback()
		return
	}

	var cats []models.Category
	if err := h.DB.Order("id ASC").Find(&cats).Error; err != nil {
		fallback()
		return
	}

	validCats := map[uint]bool{}
	catList := make([]string, 0, len(cats))
	for _, cat := range cats {
		validCats[cat.ID] = true
		catList = append(catList, fmt.Sprintf(`{"id":%d,"name":%q}`, cat.ID, cat.Name))
	}

	systemPrompt := fmt.Sprintf(`Convert a natural-language store search query into strict JSON with these exact keys:
{"q":"","category_id":0,"min_price_inr":0,"max_price_inr":0,"in_stock":false,"sort":"newest"}
Rules:
- Available categories: [%s]. Use category_id=0 if none clearly matches.
- Put only the core product keyword in "q" (empty string if user only asked for a category/price, e.g. "electronics under 2000" -> q:"").
- Prices in the query are in Indian Rupees (INR). Put 0 if unspecified.
- "sort" must be one of: "newest", "price_asc", "price_desc", "name".
- Return ONLY valid JSON, no markdown or explanation.`, strings.Join(catList, ","))

	reply, err := h.Client.Chat(c.Request.Context(), systemPrompt, rawQuery, 150)
	if err != nil {
		log.Printf("AI SmartSearch error (falling back to keyword search): %v", err)
		fallback()
		return
	}

	parsed, ok := decodeSmartSearchJSON(reply)
	if !ok {
		fallback()
		return
	}

	if !validCats[parsed.CategoryID] {
		parsed.CategoryID = 0
	}
	if _, validSort := sortOptions[parsed.Sort]; !validSort {
		parsed.Sort = "newest"
	}
	if parsed.MinPriceINR < 0 {
		parsed.MinPriceINR = 0
	}
	if parsed.MaxPriceINR < 0 {
		parsed.MaxPriceINR = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"q":           strings.TrimSpace(parsed.Q),
		"category_id": parsed.CategoryID,
		"min_price":   parsed.MinPriceINR * 100, // rupees -> paise
		"max_price":   parsed.MaxPriceINR * 100, // rupees -> paise
		"in_stock":    parsed.InStock,
		"sort":        parsed.Sort,
		"ai_used":     true,
	})
}

func decodeSmartSearchJSON(raw string) (smartSearchParsed, bool) {
	var parsed smartSearchParsed
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return parsed, false
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return parsed, false
	}
	return parsed, true
}
