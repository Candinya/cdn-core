package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/worker"
	"caddy-delivery-network/app/server/models"
	"caddy-delivery-network/app/server/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"net/http"
)

func (a *App) cacheInstanceFileListByModel(ctx context.Context, instance *models.Instance) error {
	filesMap, err := a.buildInstanceFileListByModel(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to build instance file list by model: %w", err)
	}

	bytesMap := make(map[string][]byte)
	for aFilePath, aFileMeta := range filesMap {
		aFileMetaBytes, err := json.Marshal(&aFileMeta)
		if err != nil {
			a.l.Error("build instance file list marshal aFile meta", zap.Any("meta", aFileMeta), zap.Error(err))
			return fmt.Errorf("failed to marshal aFile meta: %w", err)
		}

		bytesMap[aFilePath] = aFileMetaBytes
	}

	cacheKey := fmt.Sprintf(constants.CacheKeyInstanceFiles, instance.ID)

	if err = a.rdb.HSet(ctx, cacheKey, bytesMap).Err(); err != nil {
		return fmt.Errorf("failed to hset cache: %w", err)
	}

	return nil
}

func (a *App) buildInstanceFileListByModel(ctx context.Context, instance *models.Instance) (map[string]types.CacheInstanceFile, error) {
	filesMap := make(map[string]types.CacheInstanceFile)

	// 添加 additional files
	for _, fileID := range instance.AdditionalFileIDs {
		var aFile models.AdditionalFile
		if err := a.db.WithContext(ctx).First(&aFile, "id = ?", fileID).Error; err != nil {
			// 文件记录拉取出错
			a.l.Error("build instance file list get file", zap.Uint("fileID", fileID), zap.Error(err))
			return nil, fmt.Errorf("failed to get file: %w", err)
		}

		filesMap[constants.AFilePathPrefix+aFile.Filename] = types.CacheInstanceFile{
			Type: types.CacheInstanceFileAdditionalFile,
			ID:   aFile.ID,
		}
	}

	// 依据 site 添加 certs
	for _, siteID := range instance.SiteIDs {
		var site models.Site
		if err := a.db.WithContext(ctx).
			Model(&models.Site{}).
			Preload("Cert").
			First(&site, "id = ?", siteID).Error; err != nil {
			// 站点记录拉取出错
			a.l.Error("build instance file list get site", zap.Error(err))
			return nil, fmt.Errorf("heartbeat get site: %w", err)
		}

		certPathPrefix := fmt.Sprintf(constants.CertPathDir, site.Cert.ID)

		filesMap[certPathPrefix+constants.CertPathCertName] = types.CacheInstanceFile{
			Type:    types.CacheInstanceFileCert,
			Subtype: types.CacheInstanceFileSubtypeCertCertificate,
			ID:      site.Cert.ID,
		}
		filesMap[certPathPrefix+constants.CertPathKeyName] = types.CacheInstanceFile{
			Type:    types.CacheInstanceFileCert,
			Subtype: types.CacheInstanceFileSubtypeCertPrivateKey,
			ID:      site.Cert.ID,
		}

		if site.Cert.IntermediateCertificate != "" {
			filesMap[certPathPrefix+constants.CertPathIntermediateName] = types.CacheInstanceFile{
				Type:    types.CacheInstanceFileCert,
				Subtype: types.CacheInstanceFileSubtypeCertIntermediate,
				ID:      site.Cert.ID,
			}
		}
	}

	return filesMap, nil
}

func (a *App) getFileByMeta(ctx context.Context, fileMeta *types.CacheInstanceFile) ([]byte, error) {
	switch fileMeta.Type {
	case types.CacheInstanceFileAdditionalFile: // 是额外文件
		var aFile models.AdditionalFile
		if err := a.db.WithContext(ctx).First(&aFile, "id = ?", fileMeta.ID).Error; err != nil {
			a.l.Error("get additional file", zap.Any("meta", fileMeta), zap.Error(err))
			return nil, fmt.Errorf("failed to get additional file: %w", err)
		}

		return aFile.Content, nil

	case types.CacheInstanceFileCert: // 是证书
		var cert models.Cert
		if err := a.db.WithContext(ctx).First(&cert, "id = ?", fileMeta.ID).Error; err != nil {
			a.l.Error("get cert", zap.Any("meta", fileMeta), zap.Error(err))
			return nil, fmt.Errorf("failed to get cert: %w", err)
		}

		switch fileMeta.Subtype {
		case types.CacheInstanceFileSubtypeCertCertificate:
			return []byte(cert.Certificate), nil
		case types.CacheInstanceFileSubtypeCertPrivateKey:
			// 需要解密
			decryptedData, err := a.aesDecrypt(cert.PrivateKey)
			if err != nil {
				a.l.Error("decrypt cert", zap.Any("meta", fileMeta), zap.Error(err))
				return nil, fmt.Errorf("decrypt cert: %w", err)
			}
			return decryptedData, nil
		case types.CacheInstanceFileSubtypeCertIntermediate:
			if cert.IntermediateCertificate == "" {
				return nil, fmt.Errorf("intermediate certificate is empty")
			}
			return []byte(cert.IntermediateCertificate), nil

		default: // 这是个啥
			return nil, fmt.Errorf("unsupported subtype %d", fileMeta.Subtype)
		}

	default: // 这是个啥
		return nil, fmt.Errorf("unsupported type %d", fileMeta.Type)
	}
}

func (a *App) GetFiles(c echo.Context, id uint, params worker.GetFilesParams) error {
	// 抓取 worker 信息（认证）
	w, err, statusCode := a.authInstance(c, id)
	if err != nil {
		a.l.Error("getfiles get worker", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查请求是否有效
	if params.XFilePath == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	rctx := c.Request().Context()

	// 从缓存中读取文件列表，用于从文件路径反向计算出文件详情
	// Hash set 没法设置过期时间，只有在 instance 的 additional files 或 certs 发生变更时需要更新记录（清空整条记录重建）

	cacheKey := fmt.Sprintf(constants.CacheKeyInstanceFiles, w.ID)
	if exist, err := a.rdb.Exists(rctx, cacheKey).Result(); err != nil {
		a.l.Error("get instance file list", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	} else if exist == 0 {
		// 没有记录，那么需要重建
		if err = a.cacheInstanceFileListByModel(rctx, w); err != nil {
			// 重建失败
			a.l.Error("cache instance file list", zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		} // 否则就是重建成功了，有记录
	}

	dataBytes, err := a.rdb.HGet(rctx, cacheKey, *params.XFilePath).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("get file meta bytes", zap.String("filePath", *params.XFilePath), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	var fileMeta types.CacheInstanceFile
	if err := json.Unmarshal(dataBytes, &fileMeta); err != nil {
		a.l.Error("unmarshal file meta", zap.String("filePath", *params.XFilePath), zap.ByteString("dataBytes", dataBytes), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	fileBytes, err := a.getFileByMeta(rctx, &fileMeta)
	if err != nil {
		a.l.Error("get file by meta", zap.String("filePath", *params.XFilePath), zap.Any("fileMeta", fileMeta), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	} else if fileBytes == nil {
		// 不存在的文件
		return c.NoContent(http.StatusNotFound)
	}

	return c.Blob(http.StatusOK, echo.MIMEOctetStream, fileBytes)
}
