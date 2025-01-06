package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/utils"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (a *App) er(c echo.Context, statusCode int) error {
	return c.JSON(statusCode, &admin.ErrorMessage{
		Message: utils.P(http.StatusText(statusCode)),
	})
}
