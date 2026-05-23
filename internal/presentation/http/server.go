package http

import (
	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/internal/presentation/http/handler"
	"github.com/yourname/go-clean-base/internal/presentation/http/middleware"
	"github.com/yourname/go-clean-base/internal/presentation/http/validator"
	"github.com/yourname/go-clean-base/internal/usecase"
)

// Dependencies holds all usecase interfaces injected into the HTTP server.
// Add new usecases here without changing the NewServer signature.
type Dependencies struct {
	TodoUsecase usecase.ITodoUsecase
}

func NewServer(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	e.Use(middleware.ErrorHandler())
	e.Use(middleware.RequestLogger())

	e.GET("/health", handler.NewHealthHandler().Check)

	v1 := e.Group("/api/v1")
	todos := v1.Group("/todos")
	todoHandler := handler.NewTodoHandler(deps.TodoUsecase)
	todos.GET("", todoHandler.List)
	todos.GET("/:id", todoHandler.GetByID)
	todos.POST("", todoHandler.Create)
	todos.PUT("/:id", todoHandler.Update)
	todos.DELETE("/:id", todoHandler.Delete)

	return e
}
