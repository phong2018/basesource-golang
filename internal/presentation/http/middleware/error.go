package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
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
			switch e := err.(type) {
			case *apperror.AppError:
				appErr = e
			default:
				appErr = apperror.Internal(e)
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
