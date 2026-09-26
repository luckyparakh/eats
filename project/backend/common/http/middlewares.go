package http

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func useMiddlewares(e *echo.Echo) {
	e.Use(
		middleware.ContextTimeout(10*time.Second),
		middleware.Recover(),
	)
}
