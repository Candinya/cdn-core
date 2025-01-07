package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func (a *App) additionalFileMapFields(req *admin.AdditionalFileInfoInput, aFile *models.AdditionalFile) {
	if req.Name != nil {
		aFile.Name = *req.Name
	}
	if req.Filename != nil {
		aFile.Filename = *req.Filename
	}
}

func (a *App) additionalFileUpdateClearCache(ctx context.Context, id uint, oldFilename string, newFilename string) error {
	var instances []models.Instance
	if err := a.db.WithContext(ctx).
		Find(&instances, "? = ANY(additional_file_ids)", id).
		Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// 出问题了
			a.l.Error("failed to get instances", zap.Error(err))
			return fmt.Errorf("failed to get instances: %w", err)
		}
	}

	// 清理心跳数据
	for _, instance := range instances {
		// 清理心跳数据缓存（这里包含了文件和对应的更新时间）
		heartbeatCacheKey := fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, instance.ID)
		a.rdb.Del(ctx, heartbeatCacheKey)
	}

	// 如果文件名变化
	if oldFilename != newFilename {
		oldFilePath := constants.AFilePathPrefix + oldFilename
		newFilePath := constants.AFilePathPrefix + newFilename

		for _, instance := range instances {
			// 清理文件列表缓存
			filesCacheKey := fmt.Sprintf(constants.CacheKeyInstanceFiles, instance.ID)

			if fileData, err := a.rdb.HGet(ctx, filesCacheKey, oldFilePath).Bytes(); err != nil {
				if !errors.Is(err, redis.Nil) {
					// 处理不了，直接清空整个 hash set
					a.l.Error("failed to get cached file data", zap.Error(err))
					a.rdb.Del(ctx, filesCacheKey)
				}
			} else {
				// 把数据搬到新的路径，清理掉老的缓存
				a.rdb.HSet(ctx, filesCacheKey, newFilePath, fileData)
				a.rdb.HDel(ctx, filesCacheKey, oldFilePath)
			}
		}
	}

	return nil
}

func (a *App) additionalFileCheckAbleToDelete(ctx context.Context, id uint) (bool, error) {
	var instanceCount int64
	if err := a.db.WithContext(ctx).
		Model(&models.Instance{}).
		Where("? = ANY(additional_file_ids)", id).
		Count(&instanceCount).
		Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// 出问题了
			a.l.Error("failed to get instances", zap.Error(err))
			return false, fmt.Errorf("failed to get instances: %w", err)
		}
	}

	return instanceCount == 0, nil
}

func (a *App) AdditionalFileCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileCreateMultipartBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 创建
	var aFile models.AdditionalFile
	var filename string
	if req.Filename != nil {
		filename = *req.Filename
	} else {
		filename = req.Content.Filename()
	}
	a.additionalFileMapFields(&admin.AdditionalFileInfoInput{
		Name:     req.Name,
		Filename: &filename,
	}, &aFile)

	if aFile.Content, err = req.Content.Bytes(); err != nil {
		a.l.Error("failed to read file content", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	if err := a.db.WithContext(rctx).Create(&aFile).Error; err != nil {
		a.l.Error("failed to create file", zap.Any("file", aFile), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.AdditionalFileInfoWithID{
		Id:       &aFile.ID,
		Name:     &aFile.Name,
		Filename: &aFile.Filename,
	})
}

func (a *App) AdditionalFileList(c echo.Context, params admin.AdditionalFileListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	var (
		aFiles      []models.AdditionalFile
		aFilesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.AdditionalFile{}).Limit(limit).Offset(page * limit).Find(&aFiles).Error; err != nil {
		a.l.Error("failed to get file list", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.AdditionalFile{}).Count(&aFilesCount).Error; err != nil {
		a.l.Error("failed to count file", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	pageMax := aFilesCount / int64(limit)
	if (aFilesCount % int64(limit)) != 0 {
		pageMax++
	}

	resFiles := []admin.AdditionalFileInfoWithID{}
	for _, aFile := range aFiles {
		resFiles = append(resFiles, admin.AdditionalFileInfoWithID{
			Id:       &aFile.ID,
			Name:     &aFile.Name,
			Filename: &aFile.Filename,
		})
	}

	return c.JSON(http.StatusOK, &admin.AdditionalFileListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resFiles,
	})
}

func (a *App) AdditionalFileInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusOK, &admin.AdditionalFileInfoWithID{
		Id:       &aFile.ID,
		Name:     &aFile.Name,
		Filename: &aFile.Filename,
	})
}

func (a *App) AdditionalFileInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 如果文件名称发生变更，需要清理旧缓存
	if req.Filename != nil && *req.Filename != aFile.Filename {
		if err := a.additionalFileUpdateClearCache(rctx, aFile.ID, aFile.Filename, *req.Filename); err != nil {
			a.l.Error("failed to clear cache", zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 更新
	a.additionalFileMapFields(&req, &aFile)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&aFile).Error; err != nil {
		a.l.Error("failed to update file", zap.Any("file", aFile), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.AdditionalFileInfoWithID{
		Id:       &aFile.ID,
		Name:     &aFile.Name,
		Filename: &aFile.Filename,
	})
}

func (a *App) AdditionalFileReplace(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileReplaceMultipartBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 清理缓存（文件名没有变化，只需要清理心跳数据）
	if err := a.additionalFileUpdateClearCache(rctx, aFile.ID, aFile.Filename, aFile.Filename); err != nil {
		a.l.Error("failed to clear cache", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	newContentBytes, err := req.Content.Bytes()
	if err != nil {
		a.l.Error("failed to read file content", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 更新信息
	if err := a.db.WithContext(rctx).Model(&aFile).Update("content", newContentBytes).Error; err != nil {
		a.l.Error("failed to update file", zap.Any("file", aFile), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.AdditionalFileInfoWithID{
		Id:       &aFile.ID,
		Name:     &aFile.Name,
		Filename: &aFile.Filename,
	})
}

func (a *App) AdditionalFileDownload(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.Blob(http.StatusOK, echo.MIMEOctetStream, aFile.Content)
}

func (a *App) AdditionalFileDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 检查是否可以被删除
	if ableToDelete, err := a.additionalFileCheckAbleToDelete(rctx, id); err != nil {
		a.l.Error("failed to check able-to-delete", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	} else if !ableToDelete {
		return a.er(c, http.StatusPreconditionFailed)
	}

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.AdditionalFile{}, id).Error; err != nil {
		a.l.Error("failed to delete file", zap.Uint("id", id), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
