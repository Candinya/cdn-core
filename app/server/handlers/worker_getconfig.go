package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/models"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"net/http"
)

func (a *App) GetConfig(c echo.Context, id uint) error {
	w := c.Get("instance").(*models.Instance)

	rctx := c.Request().Context()

	// 检查是否有缓存结果
	var resBytes []byte
	if data, err := a.rdb.Get(rctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, w.ID)).Bytes(); err != nil {
		if !errors.Is(err, redis.Nil) {
			a.l.Error("getconfig check cache", zap.Error(err))
		}

		// 产生结果
		resString, err := a.buildInstanceConfigByModel(rctx, w)
		if err != nil {
			a.l.Error("getconfig build config", zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
		resBytes = []byte(resString)

		// 加入缓存
		a.rdb.Set(rctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, w.ID), resString, constants.CacheExpireInstanceConfig)
	} else {
		resBytes = data
	}

	// 使用结果响应
	return c.Blob(http.StatusOK, "text/caddyfile", resBytes)
}
