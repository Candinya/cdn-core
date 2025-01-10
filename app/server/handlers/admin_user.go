package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"caddy-delivery-network/app/server/utils"
	"errors"
	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func (a *App) userMapFields(req *admin.UserInfoInput, user *models.User) {
	if req.Name != nil {
		user.Name = *req.Name
	}
}

func (a *App) UserCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 处理密码
	passwordHash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		a.l.Error("failed to hash password", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 创建用户
	user := models.User{
		Username: req.Username,
		Password: passwordHash,
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	a.userMapFields(&admin.UserInfoInput{
		Name: req.Name,
	}, &user)

	if err := a.db.WithContext(rctx).Create(&user).Error; err != nil {
		a.l.Error("failed to create user", zap.Any("user", user), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserList(c echo.Context, params admin.UserListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	var (
		users      []models.User
		usersCount int64
	)

	showAll, page, limit := a.parsePagination(params.Page, params.Limit)
	queryBase := a.db.WithContext(rctx).Model(&models.User{}).Order("id ASC")
	if !showAll {
		queryBase = queryBase.Limit(limit).Offset(page * limit)
	}

	if err := queryBase.Find(&users).Error; err != nil {
		a.l.Error("failed to get user list", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.User{}).Count(&usersCount).Error; err != nil {
		a.l.Error("failed to count user", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	resUsers := []admin.UserInfoWithID{}
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
		PageMax: utils.P(a.calcMaxPage(usersCount, showAll, limit)),
		List:    &resUsers,
	})
}

func (a *App) UserInfoGetSelf(c echo.Context) error {
	// 抓取 user 信息（认证）
	//err, statusCode := a.authAdmin(c, false, nil)
	//if err != nil {
	//	a.l.Error("failed to get user", zap.Error(err))
	//	return a.er(c, statusCode)
	//}

	// 这里比较特殊，因为是对于用户本身的操作，没有指定 id ，所以需要用接口提取，所以不能用一般的认证中间件（像上面那种）
	jwtUser, err := a.getJwtUser(c)
	if err != nil {
		a.l.Error("failed to get jwt user", zap.Error(err))
		return a.er(c, http.StatusUnauthorized)
	}

	rctx := c.Request().Context()

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", jwtUser.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", jwtUser.ID), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, &id)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, &id)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	a.userMapFields(&req, &user)

	// 更新用户信息
	if err := a.db.WithContext(rctx).Updates(&user).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserUsernameUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, &id)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserUsernameUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}
	if req.Username == nil {
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("username", *req.Username).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}

func (a *App) UserPasswordUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, &id)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserPasswordUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}
	if req.Password == nil {
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	newPasswordHash, err := argon2id.CreateHash(*req.Password, argon2id.DefaultParams)
	if err != nil {
		a.l.Error("failed to hash password", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("password", newPasswordHash).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func (a *App) UserDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, &id)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 删除用户
	if err := a.db.WithContext(rctx).Delete(&models.User{}, id).Error; err != nil {
		a.l.Error("failed to delete user", zap.Uint("id", id), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func (a *App) UserRoleUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.UserRoleUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}
	if req.IsAdmin == nil {
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得指定的用户
	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get user", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 更新用户信息
	if err := a.db.WithContext(rctx).Model(&user).Update("is_admin", *req.IsAdmin).Error; err != nil {
		a.l.Error("failed to update user", zap.Any("user", user), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &admin.UserInfoWithID{
		Id:       &user.ID,
		Username: &user.Username,
		IsAdmin:  &user.IsAdmin,
		Name:     &user.Name,
	})
}
