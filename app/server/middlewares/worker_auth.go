package middlewares

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
	"strconv"
	"strings"
)

func WorkerAuth(db *gorm.DB, rdb *redis.Client, l *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 提取 ID
			idStr := c.Param("id")
			idUint64, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil {
				return c.NoContent(http.StatusBadRequest)
			}

			id := uint(idUint64)

			var instance models.Instance

			rctx := c.Request().Context()

			// 提取 token
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.NoContent(http.StatusUnauthorized)
			}

			splits := strings.Split(authHeader, " ")
			if len(splits) != 2 {
				return c.NoContent(http.StatusUnauthorized)
			}

			if strings.ToLower(splits[0]) != "bearer" {
				return c.NoContent(http.StatusUnauthorized)
			}

			// 格式化 UUID
			uuidToken, err := uuid.Parse(splits[1])
			if err != nil {
				return c.NoContent(http.StatusUnauthorized)
			}

			// 查询缓存
			cacheKey := fmt.Sprintf(constants.CacheKeyInstanceInfo, id)
			if cacheBytes, err := rdb.Get(rctx, cacheKey).Bytes(); err != nil {
				if !errors.Is(err, redis.Nil) {
					l.Error("failed to query cache for instance info", zap.Uint("id", id), zap.Error(err))
				}
			} else if err = json.Unmarshal(cacheBytes, &instance); err != nil {
				l.Error("failed to unmarshal instance info", zap.Uint("id", id), zap.ByteString("cacheBytes", cacheBytes), zap.Error(err))
				// 可能是无效的缓存，清理掉
				rdb.Del(rctx, cacheKey)
			} else {
				// 成功拉取到并格式化
				if instance.Token == uuidToken {
					// 设置 context
					c.Set("instance", &instance)

					// 继续处理
					return next(c)
				} else {
					return c.NoContent(http.StatusNotFound)
				}
			}

			// 查询数据库
			if err = db.WithContext(rctx).First(&instance, "id = ? AND token = ?", id, uuidToken).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.NoContent(http.StatusNotFound)
				} else {
					return c.NoContent(http.StatusInternalServerError)
				}
			}

			// 格式化并加入缓存，方便下一次查询
			if cacheBytes, err := json.Marshal(&instance); err != nil {
				l.Error("failed to marshal instance info", zap.Uint("id", id), zap.Error(err))
			} else {
				rdb.Set(rctx, cacheKey, cacheBytes, constants.CacheExpireInstanceInfo)
			}

			// 设置 context
			c.Set("instance", &instance)

			// 继续处理
			return next(c)
		}
	}
}
