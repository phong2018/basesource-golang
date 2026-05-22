package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type CustomValidator struct {
	v *validator.Validate
}

func New() *CustomValidator {
	return &CustomValidator{v: validator.New()}
}

func (cv *CustomValidator) Validate(i any) error {
	return cv.v.Struct(i)
}

func BindAndValidate(c echo.Context, req any) error {
	if err := c.Bind(req); err != nil {
		return err
	}
	return c.Validate(req)
}
