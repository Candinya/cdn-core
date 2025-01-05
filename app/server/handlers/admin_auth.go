package handlers

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

func (a *App) authAdmin(c echo.Context, requireAdminRole bool, matchID *uint) (error, int) {
	// 提取 token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing auth token"), http.StatusUnauthorized
	}

	splits := strings.Split(authHeader, " ")
	if len(splits) != 2 {
		return fmt.Errorf("invalid auth header: %s", authHeader), http.StatusUnauthorized
	}

	if strings.ToLower(splits[0]) != "bearer" {
		return fmt.Errorf("unknown auth method: %s", splits[0]), http.StatusUnauthorized
	}

	// 验证 token
	jwtUser, err := a.jwt.ParseUser(splits[1])
	if err != nil {
		// 无效的 token
		return fmt.Errorf("failed to parse token: %w", err), http.StatusUnauthorized
	}

	// 验证权限
	if requireAdminRole && !jwtUser.IsAdmin {
		return fmt.Errorf("requires admin role"), http.StatusForbidden
	}

	// 对照 ID
	if matchID != nil && jwtUser.ID != *matchID && !jwtUser.IsAdmin {
		return fmt.Errorf("user id not match"), http.StatusForbidden
	}

	return nil, http.StatusOK
}
