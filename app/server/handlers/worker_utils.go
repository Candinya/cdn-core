package handlers

import (
	"caddy-delivery-network/app/server/models"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

func (a *App) getInstance(c echo.Context, id uint) (w *models.Instance, err error, httpStatus int) {
	// 提取 token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing auth token"), http.StatusUnauthorized
	}

	splits := strings.Split(authHeader, " ")
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid auth header: %s", authHeader), http.StatusUnauthorized
	}

	if strings.ToLower(splits[0]) != "bearer" {
		return nil, fmt.Errorf("unknown auth method: %s", splits[0]), http.StatusUnauthorized
	}

	uuidToken, err := uuid.Parse(splits[1])
	if err != nil {
		return nil, fmt.Errorf("invalid uuid token: %s", splits[1]), http.StatusUnauthorized
	}

	// 查询数据库
	if err = a.db.First(&w, "id = ? AND token = ?", id, uuidToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no such instance"), http.StatusNotFound
		} else {
			return nil, fmt.Errorf("error query instance: %w", err), http.StatusInternalServerError
		}
	}

	return w, nil, http.StatusOK
}
