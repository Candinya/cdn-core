package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"errors"
	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func (a *App) UserCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	_, err, statusCode := a.authUser(c, true)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 处理密码
	passwordHash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		a.l.Error("failed to hash password", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	// 创建用户
	user := models.User{
		Username: req.Username,
		Password: passwordHash,
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}

	if err := a.db.WithContext(rctx).Create(&user).Error; err != nil {
		a.l.Error("failed to create user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserList(c echo.Context, params admin.UserListParams) error {
	// 列出用户信息
	_, err, statusCode := a.authUser(c, true)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	var (
		users      []models.User
		usersCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.User{}).Limit(limit).Offset(page * limit).Find(&users); err != nil {
		a.l.Error("failed to get user list")
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.User{}).Count(&usersCount).Error; err != nil {
		a.l.Error("failed to count user")
		return c.NoContent(http.StatusInternalServerError)
	}

	pageMax := usersCount / int64(limit)
	if (usersCount % int64(limit)) != 0 {
		pageMax++
	}

	var resUsers []admin.UserInfoWithID
	for _, user := range users {
		resUsers = append(resUsers, admin.UserInfoWithID{
			Id:       &user.ID,
			Username: &user.Username,
			IsAdmin:  &user.IsAdmin,
			Name:     &user.Name,
		})
	}

	return c.JSON(http.StatusOK, &admin.UserListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resUsers,
	})
}

func (a *App) UserInfoGet(c echo.Context, id uint) error {
	// 列出用户信息
	jwtUser, err, statusCode := a.authUser(c, false)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查权限
	if jwtUser.ID != id && !jwtUser.IsAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	rctx := c.Request().Context()

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusCreated, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserInfoUpdate(c echo.Context, id uint) error {
	// 列出用户信息
	jwtUser, err, statusCode := a.authUser(c, false)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查权限
	if jwtUser.ID != id && !jwtUser.IsAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Updates(&req).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserUsernameUpdate(c echo.Context, id uint) error {
	// 列出用户信息
	jwtUser, err, statusCode := a.authUser(c, false)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查权限
	if jwtUser.ID != id && !jwtUser.IsAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserUsernameUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Username == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("username", *req.Username).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserPasswordUpdate(c echo.Context, id uint) error {
	// 列出用户信息
	jwtUser, err, statusCode := a.authUser(c, false)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查权限
	if jwtUser.ID != id && !jwtUser.IsAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserPasswordUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Password == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	newPasswordHash, err := argon2id.CreateHash(*req.Password, argon2id.DefaultParams)
	if err != nil {
		a.l.Error("failed to hash password", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("password", newPasswordHash).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func (a *App) UserDelete(c echo.Context, id uint) error {
	// 列出用户信息
	jwtUser, err, statusCode := a.authUser(c, false)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	// 检查权限
	if jwtUser.ID != id && !jwtUser.IsAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	rctx := c.Request().Context()

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 删除用户
	if err := a.db.WithContext(rctx).Delete(&user).Error; err != nil {
		a.l.Error("failed to delete user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func (a *App) UserRoleUpdate(c echo.Context, id uint) error {
	// 列出用户信息
	_, err, statusCode := a.authUser(c, true)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserRoleUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}
	if req.IsAdmin == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("is_admin", *req.IsAdmin).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}