package handlers

import (
	"caddy-delivery-network/app/server/jwt"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

func (a *App) authAdmin(c echo.Context, requireAdminRole bool, matchID *uint) (error, int) {
	jwtUser, err := a.getJwtUser(c)
	if err != nil {
		return fmt.Errorf("failed to get jwt user: %w", err), http.StatusUnauthorized
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

func (a *App) getJwtUser(c echo.Context) (*jwt.User, error) {
	// 提取 token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing auth token")
	}

	splits := strings.Split(authHeader, " ")
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid auth header: %s", authHeader)
	}

	if strings.ToLower(splits[0]) != "bearer" {
		return nil, fmt.Errorf("unknown auth method: %s", splits[0])
	}

	// 验证 token
	jwtUser, err := a.jwt.ParseUser(splits[1])
	if err != nil {
		// 无效的 token
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return jwtUser, nil
}
