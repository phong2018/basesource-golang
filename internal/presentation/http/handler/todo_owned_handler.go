package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/internal/presentation/http/middleware"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type TodoOwnedHandler struct {
	uc usecase.ITodoOwnedUsecase
}

func NewTodoOwnedHandler(uc usecase.ITodoOwnedUsecase) *TodoOwnedHandler {
	return &TodoOwnedHandler{uc: uc}
}

func callerID(c echo.Context) int64 {
	id, _ := c.Get(middleware.ContextKeyUserID).(int64)
	return id
}

func callerRole(c echo.Context) string {
	role, _ := c.Get(middleware.ContextKeyRole).(string)
	return role
}

func (h *TodoOwnedHandler) ListMine(c echo.Context) error {
	var filter dto.ListTodoInput
	if err := c.Bind(&filter); err != nil {
		return apperror.BadRequest("invalid query params")
	}
	out, err := h.uc.ListMine(c.Request().Context(), callerID(c), filter)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoOwnedHandler) GetMine(c echo.Context) error {
	id, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	out, err := h.uc.GetMine(c.Request().Context(), id, callerID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoOwnedHandler) CreateMine(c echo.Context) error {
	var input dto.CreateOwnedTodoInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	input.OwnerID = callerID(c)
	out, err := h.uc.CreateMine(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out)
}

func (h *TodoOwnedHandler) UpdateMine(c echo.Context) error {
	id, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	var input dto.UpdateOwnedTodoInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	input.ID = id
	input.OwnerID = callerID(c)
	out, err := h.uc.UpdateMine(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h *TodoOwnedHandler) DeleteMine(c echo.Context) error {
	id, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	if err := h.uc.DeleteMine(c.Request().Context(), id, callerID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoOwnedHandler) Share(c echo.Context) error {
	id, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	var input dto.ShareTodoInput
	if err := c.Bind(&input); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := c.Validate(&input); err != nil {
		return apperror.BadRequest(err.Error())
	}
	input.TodoID = id
	input.OwnerID = callerID(c)
	if err := h.uc.ShareTodo(c.Request().Context(), input); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoOwnedHandler) RevokeShare(c echo.Context) error {
	todoID, err := parseUint(c, "id")
	if err != nil {
		return apperror.BadRequest("invalid id")
	}
	uid, err := parseInt64(c, "uid")
	if err != nil {
		return apperror.BadRequest("invalid uid")
	}
	if err := h.uc.RevokeShare(c.Request().Context(), todoID, callerID(c), uid); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoOwnedHandler) UploadAttachment(c echo.Context) error {
	return apperror.BadRequest("S3 upload not configured in this environment")
}

func (h *TodoOwnedHandler) DeleteAttachment(c echo.Context) error {
	return apperror.BadRequest("S3 upload not configured in this environment")
}

func parseUint(c echo.Context, param string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

func parseInt64(c echo.Context, param string) (int64, error) {
	v, err := strconv.ParseInt(c.Param(param), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}
