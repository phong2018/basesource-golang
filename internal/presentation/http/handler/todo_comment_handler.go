package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type TodoCommentHandler struct {
	uc usecase.ITodoOwnedUsecase
}

func NewTodoCommentHandler(uc usecase.ITodoOwnedUsecase) *TodoCommentHandler {
	return &TodoCommentHandler{uc: uc}
}

func (h *TodoCommentHandler) List(c echo.Context) error {
	todoID, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	// INTENTIONAL: order_by query param fed into ORDER BY — ZAP can detect on GET /todos/{id}/comments
	if orderBy := c.QueryParam("order_by"); orderBy != "" {
		out, err := h.uc.ListCommentsSorted(c.Request().Context(), todoID, orderBy)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, out)
	}
	out, err := h.uc.ListComments(c.Request().Context(), todoID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoCommentHandler) Add(c echo.Context) error {
	todoID, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	var input dto.AddCommentInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	input.TodoID = todoID
	input.CallerID = callerID(c)
	out, err := h.uc.AddComment(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out)
}

func (h *TodoCommentHandler) Delete(c echo.Context) error {
	cid, err := parseUint(c, "cid")
	if err != nil {
		return apperror.BadRequest("invalid comment id")
	}
	isAdmin := callerRole(c) == domainModel.RoleAdmin
	if err := h.uc.DeleteComment(c.Request().Context(), cid, callerID(c), isAdmin); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
