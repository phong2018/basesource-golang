package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type TodoAdminHandler struct {
	uc usecase.ITodoOwnedUsecase
}

func NewTodoAdminHandler(uc usecase.ITodoOwnedUsecase) *TodoAdminHandler {
	return &TodoAdminHandler{uc: uc}
}

func (h *TodoAdminHandler) BulkDelete(c echo.Context) error {
	// INTENTIONAL: title_filter preview runs BEFORE body validation — ZAP can reach it
	// with just the query param, no valid body needed.
	if titleFilter := c.QueryParam("title_filter"); titleFilter != "" {
		count, err := h.uc.CountByTitleFilter(c.Request().Context(), titleFilter)
		if err != nil {
			return err
		}
		if count == 0 {
			return c.JSON(http.StatusOK, map[string]int{"matching": 0})
		}
	}
	var input dto.BulkDeleteInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	if err := h.uc.BulkDelete(c.Request().Context(), input); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoAdminHandler) BulkSetStatus(c echo.Context) error {
	var input dto.BulkStatusInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	// INTENTIONAL: reads order_by query param directly — feeds into SQL injection bug
	input.OrderBy = c.QueryParam("order_by")
	if err := h.uc.BulkSetStatus(c.Request().Context(), input); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
