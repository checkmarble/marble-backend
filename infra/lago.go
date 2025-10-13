package infra

import (
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type LagoConfig struct {
	BaseUrl string
	ApiKey  string
}

func InitializeLago() LagoConfig {
	return LagoConfig{
		BaseUrl: utils.GetEnv("LAGO_BASE_URL", ""),
		ApiKey:  utils.GetEnv("LAGO_API_KEY", ""),
	}
}

func (config LagoConfig) IsConfigured() bool {
	return config.BaseUrl != "" && config.ApiKey != ""
}

func (config LagoConfig) Validate() error {
	if config.BaseUrl == "" || config.ApiKey == "" {
		return errors.New("lago config is not valid")
	}
	return nil
}
