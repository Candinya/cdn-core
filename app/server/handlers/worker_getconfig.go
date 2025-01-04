package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"net/http"
)

func (a *App) GetConfig(c echo.Context, id uint) error {
	// 抓取 worker 信息（认证）
	w, err, statusCode := a.getInstance(c, id)
	if err != nil {
		a.l.Error("getconfig get worker", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 检查是否有缓存结果
	var resString string
	if data, err := a.rdb.Get(rctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, w.ID)).Result(); err != nil {
		if !errors.Is(err, redis.Nil) {
			a.l.Error("getconfig check cache", zap.Error(err))
		}

		// 产生结果
		resString, err = a.buildInstanceConfigByModel(rctx, w)
		if err != nil {
			a.l.Error("getconfig build config", zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}

		// 加入缓存
		a.rdb.Set(rctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, w.ID), resString, constants.CacheExpireInstanceHeartbeat)
	} else {
		resString = data
	}

	// 使用结果响应
	return c.String(http.StatusOK, resString)
}
