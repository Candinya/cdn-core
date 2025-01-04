package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/worker"
	"caddy-delivery-network/app/server/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func (a *App) buildHeartbeatData(ctx context.Context, w *models.Instance) ([]byte, error) {
	var res worker.HeartbeatRes           // 准备结果对象
	configUpdatedAt := w.UpdatedAt.Unix() // 暂存为实例更新时间，但如果站点有更新，那么这个时间也将会被后移

	// 检查直接追加的附加文件
	for _, fileID := range w.AdditionalFileIDs {
		var aFile models.AdditionalFile
		if err := a.db.WithContext(ctx).First(&aFile, "id = ?", fileID).Error; err != nil {
			// 文件记录拉取出错
			a.l.Error("heartbeat get file", zap.Uint("fileID", fileID), zap.Error(err))
			return nil, fmt.Errorf("heartbeat get file: %w", err)
		}

		// 追加站点文件
		aFileUpdatedAt := aFile.UpdatedAt.Unix()

		res.FilesUpdatedAt = append(res.FilesUpdatedAt, worker.FileUpdateRecord{
			Path:      aFile.Path,
			UpdatedAt: aFileUpdatedAt,
		})
	}

	// 检查站点对应的证书文件
	for _, siteID := range w.SiteIDs {
		var site models.Site
		if err := a.db.WithContext(ctx).
			Model(&models.Site{}).
			Preload("Cert").
			Preload("Template").
			First(&site, "id = ?", siteID).Error; err != nil {
			// 站点记录拉取出错
			a.l.Error("heartbeat get site", zap.Error(err))
			return nil, fmt.Errorf("heartbeat get site: %w", err)
		}

		// 追加证书文件
		if site.Cert != nil {
			certPathPrefix := fmt.Sprintf(constants.CertPathDir, site.CertID)
			// 基础信息（证书与私钥）
			res.FilesUpdatedAt = append(res.FilesUpdatedAt, worker.FileUpdateRecord{
				Path:      certPathPrefix + constants.CertPathCertName,
				UpdatedAt: site.Cert.UpdatedAt.Unix(),
			})
			res.FilesUpdatedAt = append(res.FilesUpdatedAt, worker.FileUpdateRecord{
				Path:      certPathPrefix + constants.CertPathKeyName,
				UpdatedAt: site.Cert.UpdatedAt.Unix(),
			})
			// 中间证书
			if site.Cert.IntermediateCertificate != nil {
				res.FilesUpdatedAt = append(res.FilesUpdatedAt, worker.FileUpdateRecord{
					Path:      certPathPrefix + constants.CertPathIntermediateName,
					UpdatedAt: site.Cert.UpdatedAt.Unix(),
				})
			}
		}

		// 检查更新时间
		siteUpdatedAt := site.UpdatedAt.Unix()
		if siteUpdatedAt > configUpdatedAt {
			configUpdatedAt = siteUpdatedAt
		}
		siteTemplateUpdatedAt := site.Template.UpdatedAt.Unix()
		if siteTemplateUpdatedAt > configUpdatedAt {
			configUpdatedAt = siteTemplateUpdatedAt
		}
	}

	// 确认时间
	res.ConfigUpdatedAt = configUpdatedAt

	resBytes, err := json.Marshal(res)
	if err != nil {
		a.l.Error("heartbeat json marshal", zap.Any("res", res), zap.Error(err))
		return nil, fmt.Errorf("heartbeat parse data: %w", err)
	}

	return resBytes, nil
}

func (a *App) Heartbeat(c echo.Context, id uint) error {
	// 抓取 worker 信息（认证）
	w, err, statusCode := a.getInstance(c, id)
	if err != nil {
		a.l.Error("heartbeat get worker", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 更新实例心跳时间
	a.rdb.Set(rctx, fmt.Sprintf(constants.CacheKeyInstanceLastseen, w.ID), time.Now().Unix(), constants.CacheExpireInstanceLastseen)

	// 检查是否有缓存结果
	var resBytes []byte
	if data, err := a.rdb.Get(rctx, fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, w.ID)).Bytes(); err != nil {
		if !errors.Is(err, redis.Nil) {
			a.l.Error("heartbeat check cache", zap.Error(err))
		}

		// 产生结果
		resBytes, err = a.buildHeartbeatData(rctx, w)
		if err != nil {
			a.l.Error("heartbeat build heartbeat", zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}

		// 加入缓存
		a.rdb.Set(rctx, fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, w.ID), resBytes, constants.CacheExpireInstanceHeartbeat)
	} else {
		resBytes = data
	}

	// 使用结果响应
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, resBytes)
}
