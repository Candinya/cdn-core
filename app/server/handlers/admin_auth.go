package handlers

import (
	"caddy-delivery-network/app/server/jwt"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

func (a *App) authUser(c echo.Context, requireAdminRole bool) (*jwt.User, error, int) {
	// 提取 token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing auth token"), http.StatusUnauthorized
	}

	splits := strings.Split(authHeader, " ")
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid auth header: %s", authHeader), http.StatusUnauthorized
	}

	if strings.ToLower(splits[0]) != "bearer" {
		return nil, fmt.Errorf("unknown auth method: %s", splits[0]), http.StatusUnauthorized
	}

	// 验证 token
	jwtUser, err := a.jwt.ParseUser(splits[1])
	if err != nil {
		// 无效的 token
		return nil, fmt.Errorf("failed to parse token: %w", err), http.StatusUnauthorized
	}

	// 验证权限
	if requireAdminRole && !jwtUser.IsAdmin {
		return nil, fmt.Errorf("requires admin role"), http.StatusForbidden
	}

	return jwtUser, nil, http.StatusOK
}
