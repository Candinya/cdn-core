package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"caddy-delivery-network/app/server/utils"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func (a *App) instanceGetLastSeen(ctx context.Context, isManualMode bool, id uint) *int64 {
	// 获取 last seen 数据
	if !isManualMode {
		cacheKey := fmt.Sprintf(constants.CacheKeyInstanceLastseen, id)
		if lastSeenTs, err := a.rdb.Get(ctx, cacheKey).Int64(); err != nil {
			a.l.Error("failed to get instance last seen", zap.Uint("id", id), zap.Error(err))
		} else {
			return &lastSeenTs
		}
	}

	return nil
}

func (a *App) instanceMapFields(req *admin.InstanceInfoInput, instance *models.Instance) {
	if req.Name != nil {
		instance.Name = *req.Name
	}
	if req.PreConfig != nil {
		instance.PreConfig = *req.PreConfig
	}
	if req.IsManualMode != nil {
		instance.IsManualMode = *req.IsManualMode
	}
	if req.AdditionalFileIds != nil {
		instance.AdditionalFileIDs = utils.UintArray2int64(*req.AdditionalFileIds)
	}
	if req.SiteIds != nil {
		instance.SiteIDs = utils.UintArray2int64(*req.SiteIds)
	}
}

func (a *App) instanceValidate(ctx context.Context, instance *models.Instance) (error, int) {
	// 检查 additional file ids
	if err, statusCode := validateIDs[models.AdditionalFile](a.db.WithContext(ctx), utils.Int64Array2uint(instance.AdditionalFileIDs)); err != nil {
		a.l.Error("failed to validate additional file", zap.Error(err))
		return err, statusCode
	}

	// 检查 site ids
	if err, statusCode := validateIDs[models.Site](a.db.WithContext(ctx), utils.Int64Array2uint(instance.SiteIDs)); err != nil {
		a.l.Error("failed to validate site", zap.Error(err))
		return err, statusCode
	}

	return nil, http.StatusOK
}

func (a *App) instanceUpdateClearDataCache(ctx context.Context, id uint) {
	// 清理配置项
	a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceConfig, id))

	// 清理心跳数据
	a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, id))

	// 清理文件列表缓存
	a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceFiles, id))
}

func (a *App) instanceUpdateClearAuthCache(ctx context.Context, id uint) {
	// 清理信息（包含认证用的 token ）
	a.rdb.Del(ctx, fmt.Sprintf(constants.CacheKeyInstanceInfo, id))
}

func (a *App) InstanceCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.InstanceCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 创建
	instance := models.Instance{
		Token: uuid.New(),
	}
	a.instanceMapFields(&req, &instance)

	// 验证
	if err, statusCode = a.instanceValidate(rctx, &instance); err != nil {
		a.l.Error("failed to validate instance", zap.Error(err))
		return a.er(c, statusCode)
	}

	if err := a.db.WithContext(rctx).Create(&instance).Error; err != nil {
		a.l.Error("failed to create instance", zap.Any("instance", instance), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.InstanceInfoWithToken{
		Id:                &instance.ID,
		Name:              &instance.Name,
		Token:             utils.P(instance.Token.String()),
		PreConfig:         &instance.PreConfig,
		IsManualMode:      &instance.IsManualMode,
		AdditionalFileIds: utils.P(utils.Int64Array2uint(instance.AdditionalFileIDs)),
		SiteIds:           utils.P(utils.Int64Array2uint(instance.SiteIDs)),
	})
}

func (a *App) InstanceList(c echo.Context, params admin.InstanceListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	var (
		instances      []models.Instance
		instancesCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.Instance{}).Limit(limit).Offset(page * limit).Find(&instances).Error; err != nil {
		a.l.Error("failed to get instance list", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Instance{}).Count(&instancesCount).Error; err != nil {
		a.l.Error("failed to count instance", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	pageMax := instancesCount / int64(limit)
	if (instancesCount % int64(limit)) != 0 {
		pageMax++
	}

	var resInstances []admin.InstanceInfoWithID
	for _, instance := range instances {
		resInstances = append(resInstances, admin.InstanceInfoWithID{
			Id:       &instance.ID,
			Name:     &instance.Name,
			LastSeen: a.instanceGetLastSeen(rctx, instance.IsManualMode, instance.ID),
		})
	}

	return c.JSON(http.StatusOK, &admin.InstanceListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resInstances,
	})
}

func (a *App) InstanceInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var instance models.Instance
	if err := a.db.WithContext(rctx).First(&instance, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get instance", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusCreated, &admin.InstanceInfoWithToken{
		Id:                &instance.ID,
		Name:              &instance.Name,
		PreConfig:         &instance.PreConfig,
		IsManualMode:      &instance.IsManualMode,
		AdditionalFileIds: utils.P(utils.Int64Array2uint(instance.AdditionalFileIDs)),
		SiteIds:           utils.P(utils.Int64Array2uint(instance.SiteIDs)),
		LastSeen:          a.instanceGetLastSeen(rctx, instance.IsManualMode, instance.ID),
	})
}

func (a *App) InstanceInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.InstanceInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得
	var instance models.Instance
	if err := a.db.WithContext(rctx).First(&instance, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get instance", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 清理缓存
	a.instanceUpdateClearDataCache(rctx, instance.ID)

	// 更新信息
	a.instanceMapFields(&req, &instance)

	// 验证
	if err, statusCode = a.instanceValidate(rctx, &instance); err != nil {
		a.l.Error("failed to validate instance", zap.Error(err))
		return a.er(c, statusCode)
	}

	// 更新
	if err := a.db.WithContext(rctx).Updates(&instance).Error; err != nil {
		a.l.Error("failed to update instance", zap.Any("instance", instance), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.InstanceInfoWithID{
		Id:                &instance.ID,
		Name:              &instance.Name,
		PreConfig:         &instance.PreConfig,
		IsManualMode:      &instance.IsManualMode,
		AdditionalFileIds: utils.P(utils.Int64Array2uint(instance.AdditionalFileIDs)),
		SiteIds:           utils.P(utils.Int64Array2uint(instance.SiteIDs)),
		LastSeen:          a.instanceGetLastSeen(rctx, instance.IsManualMode, instance.ID),
	})
}

func (a *App) InstanceRotateToken(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var instance models.Instance
	if err := a.db.WithContext(rctx).First(&instance, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get instance", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 清理缓存
	a.instanceUpdateClearAuthCache(rctx, instance.ID)

	// 更新信息
	newToken := uuid.New()
	if err := a.db.WithContext(rctx).Model(&instance).Update("token", newToken).Error; err != nil {
		a.l.Error("failed to update instance", zap.Any("instance", instance), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.InstanceInfoWithToken{
		Id:                &instance.ID,
		Name:              &instance.Name,
		Token:             utils.P(instance.Token.String()),
		PreConfig:         &instance.PreConfig,
		IsManualMode:      &instance.IsManualMode,
		AdditionalFileIds: utils.P(utils.Int64Array2uint(instance.AdditionalFileIDs)),
		SiteIds:           utils.P(utils.Int64Array2uint(instance.SiteIDs)),
		LastSeen:          a.instanceGetLastSeen(rctx, instance.IsManualMode, instance.ID),
	})
}

func (a *App) InstanceDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Instance{}, id).Error; err != nil {
		a.l.Error("failed to delete instance", zap.Uint("id", id), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 清理缓存
	a.instanceUpdateClearDataCache(rctx, id)
	a.instanceUpdateClearAuthCache(rctx, id)

	return c.NoContent(http.StatusOK)
}
