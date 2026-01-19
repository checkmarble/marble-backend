package pubapi

import (
	"embed"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed openapi/*
	OPENAPI_SOURCES embed.FS
	OPENAPI_SPECS   = expirable.NewLRU[string, *openapi3.T](10, nil, 0)
)

type Config struct {
	DefaultTimeout  time.Duration
	DecisionTimeout time.Duration
}

func InitPublicApi() {
	if validator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		validator.RegisterTagNameFunc(types.FieldNameFromTag)
	}
}

func GetOpenApiForVersion(version string) (*openapi3.T, error) {
	if spec, ok := OPENAPI_SPECS.Get(version); ok {
		return spec, nil
	}

	var yamlSpec map[string]any

	b, err := OPENAPI_SOURCES.ReadFile("openapi/" + version + ".yml")
	if err != nil {
		return nil, errors.Wrapf(models.NotFoundError,
			"could not find OpenAPI spec for version '%s'", version)
	}

	if err := yaml.Unmarshal(b, &yamlSpec); err != nil {
		return nil, errors.Wrap(err, "could not parse OpenAPI YAML file")
	}

	jsonSpec, err := json.Marshal(yamlSpec)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert OpenAPI YAML to JSON")
	}

	spec, err := openapi3.NewLoader().LoadFromData(jsonSpec)
	if err != nil {
		return nil, err
	}

	OPENAPI_SPECS.Add(version, spec)

	return spec, nil
}
