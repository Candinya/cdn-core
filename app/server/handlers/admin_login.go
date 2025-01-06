package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/jwt"
	"caddy-delivery-network/app/server/models"
	"errors"
	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"time"
)

func (a *App) AuthLogin(c echo.Context) error {
	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.AuthLoginJSONRequestBody
	if err := c.Bind(&req); err != nil {
		a.l.Error("failed to bind json body", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 没有写用户名或密码
	if req.Username == nil || req.Password == nil {
		return a.er(c, http.StatusBadRequest)
	}

	// 计算密码 hash 并进行检查
	passwordHash, err := argon2id.CreateHash(*req.Password, argon2id.DefaultParams)
	if err != nil {
		a.l.Error("failed to hash password", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	var user models.User
	if err = a.db.WithContext(rctx).First(&user, "username = ? AND password = ?", *req.Username, passwordHash).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to find user", zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 签出 JWT
	expires := time.Now().Add(constants.AuthTokenDuration)
	token, err := a.jwt.SignToken(&jwt.User{
		ID:      user.ID,
		IsAdmin: user.IsAdmin,
		Expires: expires.Unix(),
	})
	if err != nil {
		a.l.Error("failed to sign token", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 返回
	return c.JSON(http.StatusOK, &admin.LoginToken{
		Token: &token,
	})
}
