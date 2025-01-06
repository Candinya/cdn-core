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

	var user models.User
	if err := a.db.WithContext(rctx).First(&user, "username = ?", *req.Username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusUnauthorized)
		} else {
			a.l.Error("failed to find user", zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 提取密码 hash 并进行校验
	if match, _, err := argon2id.CheckHash(*req.Password, user.Password); err != nil {
		a.l.Error("failed to check password", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	} else if !match {
		// 密码不一致
		return a.er(c, http.StatusUnauthorized)
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
