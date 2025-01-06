package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"errors"
	"github.com/labstack/echo/v4"
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

func (a *App) AdditionalFileCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileCreateMultipartBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
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
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := a.db.WithContext(rctx).Create(&aFile).Error; err != nil {
		a.l.Error("failed to create file", zap.Any("file", aFile), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	var (
		aFiles      []models.AdditionalFile
		aFilesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.AdditionalFile{}).Limit(limit).Offset(page * limit).Find(&aFiles).Error; err != nil {
		a.l.Error("failed to get file list", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.AdditionalFile{}).Count(&aFilesCount).Error; err != nil {
		a.l.Error("failed to count file", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	pageMax := aFilesCount / int64(limit)
	if (aFilesCount % int64(limit)) != 0 {
		pageMax++
	}

	var resFiles []admin.AdditionalFileInfoWithID
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusCreated, &admin.AdditionalFileInfoWithID{
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新
	a.additionalFileMapFields(&req, &aFile)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&aFile).Error; err != nil {
		a.l.Error("failed to update file", zap.Any("file", aFile), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.AdditionalFileInfoWithID{
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AdditionalFileReplaceMultipartBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	newContentBytes, err := req.Content.Bytes()
	if err != nil {
		a.l.Error("failed to read file content", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	// 更新信息
	if err := a.db.WithContext(rctx).Model(&aFile).Update("content", newContentBytes).Error; err != nil {
		a.l.Error("failed to update file", zap.Any("file", aFile), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.AdditionalFileInfoWithID{
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var aFile models.AdditionalFile
	if err := a.db.WithContext(rctx).First(&aFile, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get file", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.Blob(http.StatusOK, echo.MIMEOctetStream, aFile.Content)
}

func (a *App) AdditionalFileDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.AdditionalFile{}, id).Error; err != nil {
		a.l.Error("failed to delete file", zap.Uint("id", id), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
