package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
	"github.com/yourname/go-clean-base/pkg/helper"
)

type TodoHandler struct {
	usecase usecase.ITodoUsecase
}

func NewTodoHandler(uc usecase.ITodoUsecase) *TodoHandler {
	return &TodoHandler{usecase: uc}
}

func (h *TodoHandler) GetByID(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	out, err := h.usecase.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoHandler) List(c echo.Context) error {
	input := dto.ListTodoInput{
		Page:  1,
		Limit: 20,
	}
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid query params")
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Limit < 1 || input.Limit > 100 {
		input.Limit = 20
	}
	out, err := h.usecase.List(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoHandler) Create(c echo.Context) error {
	var input dto.CreateTodoInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	out, err := h.usecase.Create(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out)
}

func (h *TodoHandler) Update(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	var input dto.UpdateTodoInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	input.ID = id
	out, err := h.usecase.Update(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoHandler) Delete(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	if err := h.usecase.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func parseID(c echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

var _ = helper.Ptr[string]
