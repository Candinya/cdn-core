package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

func (a *App) getInstance(c echo.Context, id uint) (*models.Instance, error, int) {
	var instance models.Instance

	rctx := c.Request().Context()

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

	// 格式化 UUID
	uuidToken, err := uuid.Parse(splits[1])
	if err != nil {
		return nil, fmt.Errorf("invalid uuid token: %s", splits[1]), http.StatusUnauthorized
	}

	// 查询缓存
	cacheKey := fmt.Sprintf(constants.CacheKeyInstanceInfo, id)
	if cacheBytes, err := a.rdb.Get(rctx, cacheKey).Bytes(); err != nil {
		if !errors.Is(err, redis.Nil) {
			a.l.Error("failed to query cache for instance info", zap.Uint("id", id), zap.Error(err))
		}
	} else if err = json.Unmarshal(cacheBytes, &instance); err != nil {
		a.l.Error("failed to unmarshal instance info", zap.Uint("id", id), zap.ByteString("cacheBytes", cacheBytes), zap.Error(err))
		// 可能是无效的缓存，清理掉
		a.rdb.Del(rctx, cacheKey)
	} else {
		// 成功拉取到并格式化
		if instance.Token == uuidToken {
			return &instance, nil, http.StatusOK
		} else {
			return nil, fmt.Errorf("no such instance"), http.StatusNotFound
		}
	}

	// 查询数据库
	if err = a.db.WithContext(rctx).First(&instance, "id = ? AND token = ?", id, uuidToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no such instance"), http.StatusNotFound
		} else {
			return nil, fmt.Errorf("error query instance: %w", err), http.StatusInternalServerError
		}
	}

	// 格式化并加入缓存，方便下一次查询
	if cacheBytes, err := json.Marshal(&instance); err != nil {
		a.l.Error("failed to marshal instance info", zap.Uint("id", id), zap.Error(err))
	} else {
		a.rdb.Set(rctx, cacheKey, cacheBytes, constants.CacheExpireInstanceInfo)
	}

	return &instance, nil, http.StatusOK
}
