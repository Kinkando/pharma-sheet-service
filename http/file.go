package http

import (
	"bytes"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/labstack/echo/v4"
)

type driveHandler struct {
	drive    google.Drive
	validate *validator.Validate
}

func NewDriveHandler(e *echo.Echo, validate *validator.Validate, drive google.Drive) {
	handler := &driveHandler{
		drive:    drive,
		validate: validate,
	}

	route := e.Group("/file")
	route.GET("/:fileID", handler.getFile)
}

func (dh *driveHandler) getFile(c echo.Context) error {
	ctx := c.Request().Context()

	fileID := c.Param("fileID")
	result, err := dh.drive.Get(ctx, fileID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	return c.Stream(http.StatusOK, result.Metadata.ContentType, bytes.NewReader(result.Data))
}
