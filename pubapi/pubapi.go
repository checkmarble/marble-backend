package pubapi

import (
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	DefaultTimeout  time.Duration
	DecisionTimeout time.Duration
}

func InitPublicApi() {
	if validator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		validator.RegisterTagNameFunc(fieldNameFromTag)
	}
}

func fieldNameFromTag(fld reflect.StructField) string {
	name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
	if len(name) > 0 {
		if name == "-" {
			return ""
		}
		return name
	}

	name = strings.SplitN(fld.Tag.Get("form"), ",", 2)[0]
	if len(name) > 0 {
		return name
	}

	return ""
}
