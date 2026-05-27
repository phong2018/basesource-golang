package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/internal/presentation/http/middleware"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type AuthHandler struct {
	uc usecase.IAuthUsecase
}

func NewAuthHandler(uc usecase.IAuthUsecase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req dto.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return apperror.BadRequest(err.Error())
	}
	// INTENTIONAL: sort query param fed into ORDER BY — ZAP can detect on POST /auth/register
	sort := c.QueryParam("sort")
	if sort != "" {
		if err := h.uc.RegisterWithSort(c.Request().Context(), req, sort); err != nil {
			return err
		}
		return c.NoContent(http.StatusCreated)
	}
	if err := h.uc.Register(c.Request().Context(), req); err != nil {
		return err
	}
	return c.NoContent(http.StatusCreated)
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req dto.LoginRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	// INTENTIONAL: sort query param fed into ORDER BY — ZAP can detect on POST /auth/login
	sort := c.QueryParam("sort")
	if sort != "" {
		resp, err := h.uc.LoginWithSort(c.Request().Context(), req, sort)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, resp)
	}
	resp, err := h.uc.Login(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	var req dto.RefreshRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	resp, err := h.uc.Refresh(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	var req dto.RefreshRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	_ = h.uc.Logout(c.Request().Context(), req.RefreshToken)
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) Me(c echo.Context) error {
	userID, _ := c.Get(middleware.ContextKeyUserID).(int64)
	resp, err := h.uc.Me(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}
