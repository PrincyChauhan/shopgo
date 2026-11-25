package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"shopgo/ai"
)

func TestDecodeSmartSearchJSON(t *testing.T) {
	raw := "```json\n{\"q\":\"headphones\",\"category_id\":1,\"min_price_inr\":500,\"max_price_inr\":2000,\"in_stock\":true,\"sort\":\"price_asc\"}\n```"
	got, ok := decodeSmartSearchJSON(raw)
	if !ok {
		t.Fatal("expected fenced JSON to decode")
	}
	if got.Q != "headphones" || got.CategoryID != 1 || got.MinPriceINR != 500 || got.MaxPriceINR != 2000 || !got.InStock || got.Sort != "price_asc" {
		t.Fatalf("unexpected decoded struct: %+v", got)
	}

	if _, ok := decodeSmartSearchJSON("not json"); ok {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestSmartSearchFallbackWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AIHandler{Client: ai.NewClient("https://example.com", "", "gemini-2.5-flash")}
	r.GET("/api/products/smart-search", h.SmartSearch)

	req := httptest.NewRequest(http.MethodGet, "/api/products/smart-search?q=headphones", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["q"] != "headphones" || body["ai_used"] != false {
		t.Fatalf("unexpected fallback body: %+v", body)
	}
}
