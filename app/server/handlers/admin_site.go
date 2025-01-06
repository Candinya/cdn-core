package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"context"
	"errors"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func (a *App) siteMapFields(req *admin.SiteInfoInput, site *models.Site) {
	if req.Name != nil {
		site.Name = *req.Name
	}
	if req.Origin != nil {
		site.Origin = *req.Origin
	}

	if req.TemplateId != nil {
		site.TemplateID = *req.TemplateId
	}
	if req.TemplateValues != nil {
		site.TemplateValues = *req.TemplateValues
	}

	if req.CertId != nil {
		site.CertID = req.CertId
	}
}

func (a *App) siteValidate(ctx context.Context, site *models.Site) (error, int) {
	// 检查 template id
	if err, statusCode := validateIDs[models.Template](a.db.WithContext(ctx), []uint{site.TemplateID}); err != nil {
		a.l.Error("failed to validate template", zap.Error(err))
		return err, statusCode
	}

	// 检查 cert id
	if site.CertID != nil {
		if err, statusCode := validateIDs[models.Site](a.db.WithContext(ctx), []uint{*site.CertID}); err != nil {
			a.l.Error("failed to validate site", zap.Error(err))
			return err, statusCode
		}
	}

	return nil, http.StatusOK
}

func (a *App) SiteCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.SiteCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 创建
	var site models.Site
	a.siteMapFields(&req, &site)

	// 验证
	if err, statusCode = a.siteValidate(rctx, &site); err != nil {
		a.l.Error("failed to validate site", zap.Error(err))
		return c.NoContent(statusCode)
	}

	if err := a.db.WithContext(rctx).Create(&site).Error; err != nil {
		a.l.Error("failed to create site", zap.Any("site", site), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.SiteInfoWithID{
		Id:             &site.ID,
		Name:           &site.Name,
		Origin:         &site.Origin,
		TemplateId:     &site.TemplateID,
		TemplateValues: (*[]string)(&site.TemplateValues),
		CertId:         site.CertID,
	})
}

func (a *App) SiteList(c echo.Context, params admin.SiteListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	var (
		sites      []models.Site
		sitesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.Site{}).Limit(limit).Offset(page * limit).Find(&sites).Error; err != nil {
		a.l.Error("failed to get site list", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Site{}).Count(&sitesCount).Error; err != nil {
		a.l.Error("failed to count site", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	pageMax := sitesCount / int64(limit)
	if (sitesCount % int64(limit)) != 0 {
		pageMax++
	}

	var resSites []admin.SiteInfoWithID
	for _, site := range sites {
		resSites = append(resSites, admin.SiteInfoWithID{
			Id:     &site.ID,
			Name:   &site.Name,
			Origin: &site.Origin,
		})
	}

	return c.JSON(http.StatusOK, &admin.SiteListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resSites,
	})
}

func (a *App) SiteInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var site models.Site
	if err := a.db.WithContext(rctx).First(&site, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get site", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusCreated, &admin.SiteInfoWithID{
		Id:             &site.ID,
		Name:           &site.Name,
		Origin:         &site.Origin,
		TemplateId:     &site.TemplateID,
		TemplateValues: (*[]string)(&site.TemplateValues),
		CertId:         site.CertID,
	})
}

func (a *App) SiteInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.SiteInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得
	var site models.Site
	if err := a.db.WithContext(rctx).First(&site, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get site", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 验证
	if err, statusCode = a.siteValidate(rctx, &site); err != nil {
		a.l.Error("failed to validate site", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 更新信息
	if err := a.db.WithContext(rctx).Model(&site).Updates(&req).Error; err != nil {
		a.l.Error("failed to update site", zap.Any("site", site), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.SiteInfoWithID{
		Id:             &site.ID,
		Name:           &site.Name,
		Origin:         &site.Origin,
		TemplateId:     &site.TemplateID,
		TemplateValues: (*[]string)(&site.TemplateValues),
		CertId:         site.CertID,
	})
}

func (a *App) SiteDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Site{}, id).Error; err != nil {
		a.l.Error("failed to delete site", zap.Uint("id", id), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
