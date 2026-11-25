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

// GET /api/admin/stats: numbers for the admin dashboard.
func (h *StatsHandler) Get(c *gin.Context) {
	var revenue, ordersTotal, customers, products int64
	var firstErr error
	check := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Revenue counts only orders that were actually paid for.
	check(h.DB.Model(&models.Order{}).
		Where("status IN ?", []string{"paid", "shipped", "delivered"}).
		Select("CAST(COALESCE(SUM(total), 0) AS BIGINT)").Scan(&revenue).Error)
	check(h.DB.Model(&models.Order{}).Count(&ordersTotal).Error)
	check(h.DB.Model(&models.User{}).Where("role = ?", models.RoleCustomer).Count(&customers).Error)
	check(h.DB.Model(&models.Product{}).Count(&products).Error)

	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	check(h.DB.Model(&models.Order{}).Select("status, COUNT(*) AS count").Group("status").Scan(&rows).Error)
	byStatus := map[string]int64{}
	for _, r := range rows {
		byStatus[r.Status] = r.Count
	}

	low := []models.Product{}
	check(h.DB.Where("stock <= ?", 5).Order("stock ASC, id ASC").Limit(5).Find(&low).Error)

	if firstErr != nil {
		respondError(c, firstErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"revenue":          revenue,
		"orders_total":     ordersTotal,
		"orders_by_status": byStatus,
		"customers":        customers,
		"products":         products,
		"low_stock":        low,
	})
}
