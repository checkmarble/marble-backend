package infra

import (
	"net/url"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type LagoConfig struct {
	BaseUrl string
	ApiKey  string

	ParsedUrl *url.URL
}

// Initialize Lago config, return a nil parsed url if the base url is not valid
func InitializeLago() LagoConfig {
	baseUrl := utils.GetEnv("LAGO_BASE_URL", "")
	var parsedUrl *url.URL
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		parsedUrl = nil
	}

	return LagoConfig{
		BaseUrl: baseUrl,
		ApiKey:  utils.GetEnv("LAGO_API_KEY", ""),

		ParsedUrl: parsedUrl,
	}
}

func (config LagoConfig) IsConfigured() bool {
	return config.BaseUrl != "" && config.ApiKey != ""
}

func (config LagoConfig) Validate() error {
	if config.BaseUrl == "" || config.ApiKey == "" {
		return errors.New("lago config is not valid")
	}
	if config.ParsedUrl == nil {
		return errors.New("lago parsed url is not valid")
	}
	return nil
}
