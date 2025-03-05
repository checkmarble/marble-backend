package pubapi

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	entrans "github.com/go-playground/validator/v10/translations/en"
)

func InitPublicApi() {
	if validator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		en := en.New()
		uni := ut.New(en, en)
		validationTranslator, _ = uni.GetTranslator("en")
		_ = entrans.RegisterDefaultTranslations(validator, validationTranslator)
	}
}
