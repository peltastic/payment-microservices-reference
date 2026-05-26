package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

func JSON(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

func Error(c *gin.Context, status int, code string, message string) {
	JSON(c, status, ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: Header(c, "x-request-id"),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

func ValidationError(c *gin.Context, err error) {
	Error(c, http.StatusBadRequest, "validation_error", ValidationMessage(err))
}

func ValidationMessage(err error) string {
	if err == nil {
		return "request body is invalid"
	}

	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return fmt.Sprintf("request body contains malformed JSON near byte %d", syntaxError.Offset)
	}

	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		if typeError.Field != "" {
			return fmt.Sprintf("%s must be a valid %s", fieldName(typeError.Field), typeError.Type.String())
		}
		return "request body contains an invalid field type"
	}

	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "request body contains malformed JSON"
	case strings.Contains(err.Error(), "http: request body too large"):
		return "request body must be 1MB or smaller"
	case strings.Contains(err.Error(), "unknown field"):
		return strings.Replace(err.Error(), "json: ", "", 1)
	}

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) && len(validationErrors) > 0 {
		return describeValidationError(validationErrors[0])
	}

	return "request body is invalid"
}

func describeValidationError(err validator.FieldError) string {
	field := fieldName(err.Field())
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, err.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, err.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, err.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, err.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, err.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

func fieldName(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, ".", "_"))
}
