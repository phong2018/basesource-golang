package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func ErrorHandler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err == nil {
				return nil
			}
			var appErr *apperror.AppError
			var echoErr *echo.HTTPError
			switch {
			case errors.As(err, &appErr):
				// already an AppError
			case errors.Is(err, domainModel.ErrTodoNotFound):
				appErr = apperror.NotFound(err.Error())
			case errors.As(err, &echoErr):
				msg, _ := echoErr.Message.(string)
				if msg == "" {
					msg = http.StatusText(echoErr.Code)
				}
				appErr = apperror.New(echoErr.Code, msg, nil)
			default:
				appErr = apperror.Internal(err)
			}
			return c.JSON(appErr.Code, errorResponse{
				Error: errorBody{Code: appErr.Code, Message: appErr.Message},
			})
		}
	}
}

func NotFoundHandler(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errorResponse{
		Error: errorBody{Code: 404, Message: "route not found"},
	})
}
