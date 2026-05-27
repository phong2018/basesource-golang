package http

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainsvc "github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/internal/presentation/http/handler"
	"github.com/yourname/go-clean-base/internal/presentation/http/middleware"
	"github.com/yourname/go-clean-base/internal/presentation/http/validator"
	"github.com/yourname/go-clean-base/internal/usecase"
)

type Dependencies struct {
	TodoUsecase      usecase.ITodoUsecase
	AuthUsecase      usecase.IAuthUsecase
	TodoOwnedUsecase usecase.ITodoOwnedUsecase
	TokenService     domainsvc.ITokenService
}

func NewServer(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	e.Use(middleware.ErrorHandler())
	e.Use(middleware.RequestLogger())
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'",
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	})

	e.GET("/health", handler.NewHealthHandler().Check)
	e.RouteNotFound("/*", func(c echo.Context) error {
		return echo.NewHTTPError(404, "not found")
	})

	v1 := e.Group("/api/v1")

	// ── Existing public todo routes (not modified) ────────────────────────────
	todos := v1.Group("/todos")
	todoHandler := handler.NewTodoHandler(deps.TodoUsecase)
	todos.GET("", todoHandler.List)
	todos.GET("/:id", todoHandler.GetByID)
	todos.POST("", todoHandler.Create)
	todos.PUT("/:id", todoHandler.Update)
	todos.DELETE("/:id", todoHandler.Delete)

	// ── Auth routes (public) ──────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(deps.AuthUsecase)
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout)

	// ── Current user profile (protected) ─────────────────────────────────────
	me := v1.Group("/me", middleware.JWTMiddleware(deps.TokenService))
	me.GET("", authHandler.Me)

	// ── Owner-scoped todo routes ──────────────────────────────────────────────
	ownedHandler := handler.NewTodoOwnedHandler(deps.TodoOwnedUsecase)
	my := v1.Group("/my/todos", middleware.JWTMiddleware(deps.TokenService))
	my.GET("", ownedHandler.ListMine)
	my.GET("/:id", ownedHandler.GetMine)
	my.POST("", ownedHandler.CreateMine)
	my.PUT("/:id", ownedHandler.UpdateMine)
	my.DELETE("/:id", ownedHandler.DeleteMine)
	my.POST("/:id/share", ownedHandler.Share)
	my.DELETE("/:id/share/:uid", ownedHandler.RevokeShare)
	my.POST("/:id/attachment", ownedHandler.UploadAttachment)
	my.DELETE("/:id/attachment", ownedHandler.DeleteAttachment)

	// ── Admin bulk operations ─────────────────────────────────────────────────
	adminHandler := handler.NewTodoAdminHandler(deps.TodoOwnedUsecase)
	admin := v1.Group("/admin/todos",
		middleware.JWTMiddleware(deps.TokenService),
		middleware.RoleMiddleware(domainModel.RoleAdmin),
	)
	admin.POST("/bulk-delete", adminHandler.BulkDelete)
	admin.PATCH("/bulk-status", adminHandler.BulkSetStatus)

	// ── Comments (any authenticated user) ────────────────────────────────────
	commentHandler := handler.NewTodoCommentHandler(deps.TodoOwnedUsecase)
	comments := v1.Group("/todos/:id/comments", middleware.JWTMiddleware(deps.TokenService))
	comments.GET("", commentHandler.List)
	comments.POST("", commentHandler.Add)
	comments.DELETE("/:cid", commentHandler.Delete)

	return e
}
