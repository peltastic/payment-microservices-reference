package httpx

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestValidationMessageSanitizesUnknownField(t *testing.T) {
	err := errors.New(`json: unknown field "secret"`)

	if got := ValidationMessage(err); got != `unknown field "secret"` {
		t.Fatalf("unexpected validation message: %q", got)
	}
}

func TestValidationMessageDescribesJSONTypeError(t *testing.T) {
	err := &json.UnmarshalTypeError{Field: "Amount", Type: reflect.TypeOf("")}

	if got := ValidationMessage(err); got != "amount must be a valid string" {
		t.Fatalf("unexpected validation message: %q", got)
	}
}

func TestValidationMessageDescribesValidatorError(t *testing.T) {
	type request struct {
		Amount int `validate:"gt=0"`
	}

	err := validator.New().Struct(request{})

	if got := ValidationMessage(err); got != "amount must be greater than 0" {
		t.Fatalf("unexpected validation message: %q", got)
	}
}
