package handlers

import (
	"caddy-delivery-network/app/server/models"
	"fmt"
	"gorm.io/gorm"
	"net/http"
)

// 方法不能有类型形参，所以这个不能用 (a *App)
func validateIDs[M models.AdditionalFile | models.Site | models.Template | models.Cert](db *gorm.DB, ids []uint) (error, int) {
	if len(ids) > 0 {
		var (
			count int64
			model M
		)
		if err := db.
			Model(&model).
			Where("id IN ?", ids).
			Count(&count).Error; err != nil {
			// 查询失败
			return fmt.Errorf("count: %w", err), http.StatusInternalServerError
		} else if int(count) != len(ids) {
			// 数量对不上
			return fmt.Errorf("count ids mismatch"), http.StatusBadRequest
		}
	}

	return nil, http.StatusOK
}
