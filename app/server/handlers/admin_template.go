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

func (a *App) templateMapFields(req *admin.TemplateInfoInput, template *models.Template) {
	if req.Name != nil {
		template.Name = *req.Name
	}
	if req.Description != nil {
		template.Description = *req.Description
	}

	if req.Content != nil {
		template.Content = *req.Content
	}
	if req.Variables != nil {
		template.Variables = *req.Variables
	}
}

func (a *App) TemplateCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.TemplateCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 创建
	var template models.Template
	a.templateMapFields(&req, &template)

	if err := a.db.WithContext(rctx).Create(&template).Error; err != nil {
		a.l.Error("failed to create template", zap.Any("template", template), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.TemplateInfoWithID{
		Id:          &template.ID,
		Name:        &template.Name,
		Description: &template.Description,
		Content:     &template.Content,
		Variables:   (*[]string)(&template.Variables),
	})
}

func (a *App) TemplateList(c echo.Context, params admin.TemplateListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	var (
		templates      []models.Template
		templatesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.Template{}).Limit(limit).Offset(page * limit).Find(&templates).Error; err != nil {
		a.l.Error("failed to get template list", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Template{}).Count(&templatesCount).Error; err != nil {
		a.l.Error("failed to count template", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	pageMax := templatesCount / int64(limit)
	if (templatesCount % int64(limit)) != 0 {
		pageMax++
	}

	var resTemplates []admin.TemplateInfoWithID
	for _, template := range templates {
		resTemplates = append(resTemplates, admin.TemplateInfoWithID{
			Id:          &template.ID,
			Name:        &template.Name,
			Description: &template.Description,
		})
	}

	return c.JSON(http.StatusOK, &admin.TemplateListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resTemplates,
	})
}

func (a *App) TemplateInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var template models.Template
	if err := a.db.WithContext(rctx).First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get template", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusCreated, &admin.TemplateInfoWithID{
		Id:          &template.ID,
		Name:        &template.Name,
		Description: &template.Description,
		Content:     &template.Content,
		Variables:   (*[]string)(&template.Variables),
	})
}

func (a *App) TemplateInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.TemplateInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得
	var template models.Template
	if err := a.db.WithContext(rctx).First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get template", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新
	a.templateMapFields(&req, &template)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&template).Error; err != nil {
		a.l.Error("failed to update template", zap.Any("template", template), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.TemplateInfoWithID{
		Id:          &template.ID,
		Name:        &template.Name,
		Description: &template.Description,
		Content:     &template.Content,
		Variables:   (*[]string)(&template.Variables),
	})
}

func (a *App) TemplateDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Template{}, id).Error; err != nil {
		a.l.Error("failed to delete template", zap.Uint("id", id), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
