package handlers

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func (a *App) HealthCheck(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}
