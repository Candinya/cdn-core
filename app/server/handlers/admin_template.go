package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"context"
	"errors"
	"fmt"
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

func (a *App) templateUpdateClearCache(ctx context.Context, id uint) error {
	// 寻找使用了这个模板的站点
	var sites []models.Site
	if err := a.db.WithContext(ctx).Find(&sites, "template_id = ?", id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			a.l.Error("failed to get sites", zap.Error(err))
			return fmt.Errorf("failed to get sites: %w", err)
		}
	}

	// 再对于每个站点寻找部署了这个站点的实例
	for _, site := range sites {
		var instances []models.Instance
		if err := a.db.WithContext(ctx).
			Find(&instances, "? = ANY(site_ids)", site.ID).
			Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				// 出问题了
				a.l.Error("failed to get instances", zap.Error(err))
				return fmt.Errorf("failed to get instances: %w", err)
			}
		}
		for _, instance := range instances {
			// 同 ID 模板更新不会涉及到文件变更，仅需清理配置和心跳数据缓存（心跳数据里包含了配置文件的更新时间）
			a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, instance.ID))
			a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, instance.ID))
		}
	}

	return nil
}

func (a *App) templateCheckAbleToDelete(ctx context.Context, id uint) (bool, error) {
	var siteCount int64
	if err := a.db.WithContext(ctx).
		Model(&models.Site{}).
		Where("template_id = ?", id).
		Count(&siteCount).
		Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// 出问题了
			a.l.Error("failed to get sites", zap.Error(err))
			return false, fmt.Errorf("failed to get sites: %w", err)
		}
	}

	return siteCount == 0, nil
}

func (a *App) TemplateCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.TemplateCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 创建
	var template models.Template
	a.templateMapFields(&req, &template)

	if err := a.db.WithContext(rctx).Create(&template).Error; err != nil {
		a.l.Error("failed to create template", zap.Any("template", template), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
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
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	var (
		templates      []models.Template
		templatesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.Template{}).Limit(limit).Offset(page * limit).Find(&templates).Error; err != nil {
		a.l.Error("failed to get template list", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Template{}).Count(&templatesCount).Error; err != nil {
		a.l.Error("failed to count template", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
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
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var template models.Template
	if err := a.db.WithContext(rctx).First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get template", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
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
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.TemplateInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得
	var template models.Template
	if err := a.db.WithContext(rctx).First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get template", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 如果模板发生变更，需要清理旧缓存（已知私钥会随着证书变化，所以没必要单独验证）
	if req.Content != nil && *req.Content != template.Content ||
		req.Variables != nil {
		if err := a.templateUpdateClearCache(rctx, template.ID); err != nil {
			a.l.Error("failed to clear cache", zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 更新
	a.templateMapFields(&req, &template)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&template).Error; err != nil {
		a.l.Error("failed to update template", zap.Any("template", template), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
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
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 检查是否可以被删除
	if ableToDelete, err := a.templateCheckAbleToDelete(rctx, id); err != nil {
		a.l.Error("failed to check able-to-delete", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	} else if !ableToDelete {
		return a.er(c, http.StatusPreconditionFailed)
	}

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Template{}, id).Error; err != nil {
		a.l.Error("failed to delete template", zap.Uint("id", id), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
