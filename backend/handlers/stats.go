package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shopgo/models"
)

type StatsHandler struct {
	DB *gorm.DB
}

type storeStats struct {
	Revenue        int64            `json:"revenue"`
	OrdersTotal    int64            `json:"orders_total"`
	OrdersByStatus map[string]int64 `json:"orders_by_status"`
	Customers      int64            `json:"customers"`
	Products       int64            `json:"products"`
	LowStock       []models.Product `json:"low_stock"`
}

func computeStats(db *gorm.DB) (storeStats, error) {
	var out storeStats
	var firstErr error
	check := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Revenue counts only orders that were actually paid for.
	check(db.Model(&models.Order{}).
		Where("status IN ?", []string{"paid", "shipped", "delivered"}).
		Select("CAST(COALESCE(SUM(total), 0) AS BIGINT)").Scan(&out.Revenue).Error)
	check(db.Model(&models.Order{}).Count(&out.OrdersTotal).Error)
	check(db.Model(&models.User{}).Where("role = ?", models.RoleCustomer).Count(&out.Customers).Error)
	check(db.Model(&models.Product{}).Count(&out.Products).Error)

	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	check(db.Model(&models.Order{}).Select("status, COUNT(*) AS count").Group("status").Scan(&rows).Error)
	out.OrdersByStatus = map[string]int64{}
	for _, r := range rows {
		out.OrdersByStatus[r.Status] = r.Count
	}

	out.LowStock = []models.Product{}
	check(db.Where("stock <= ?", 5).Order("stock ASC, id ASC").Limit(5).Find(&out.LowStock).Error)

	return out, firstErr
}

// GET /api/admin/stats: numbers for the admin dashboard.
func (h *StatsHandler) Get(c *gin.Context) {
	stats, err := computeStats(h.DB)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}
