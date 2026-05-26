package httpx

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const MaxJSONBodyBytes int64 = 1 << 20

func BindJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	Sanitize(target)
	return binding.Validator.ValidateStruct(target)
}

func Header(c *gin.Context, name string) string {
	return strings.TrimSpace(c.GetHeader(name))
}

func Param(c *gin.Context, name string) string {
	return strings.TrimSpace(c.Param(name))
}

func RequiredHeader(c *gin.Context, name string) (string, bool) {
	value := Header(c, name)
	return value, value != ""
}

func Sanitize(target any) {
	sanitizeValue(reflect.ValueOf(target))
}

func sanitizeValue(value reflect.Value) {
	if !value.IsValid() {
		return
	}

	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.String:
		if value.CanSet() {
			value.SetString(strings.TrimSpace(value.String()))
		}
	case reflect.Struct:
		if value.Type().PkgPath() == "time" && value.Type().Name() == "Time" {
			return
		}
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath != "" {
				continue
			}
			sanitizeValue(value.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			sanitizeValue(value.Index(i))
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			item := value.MapIndex(key)
			if !item.IsValid() {
				continue
			}
			cleaned := reflect.New(item.Type()).Elem()
			cleaned.Set(item)
			sanitizeValue(cleaned)
			value.SetMapIndex(key, cleaned)
		}
	}
}
