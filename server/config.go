package server

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/utils"
)

type Config struct {
	GlobalConfiguration models.GlobalConfiguration
	PGConfig            pg_repository.PGConfig
	Port                string
	Env                 string
}

func InitConfig() Config {
	return Config{
		Port: utils.GetRequiredStringEnv("PORT"),
		Env:  utils.GetStringEnv("ENV", "DEV"),
		GlobalConfiguration: models.GlobalConfiguration{
			TokenLifetimeMinute: utils.GetIntEnv("TOKEN_LIFETIME_MINUTE", 60*2),
			FakeAwsS3Repository: utils.GetBoolEnv("FAKE_AWS_S3", false),
		},
		PGConfig: pg_repository.PGConfig{
			Hostname:         utils.GetRequiredStringEnv("PG_HOSTNAME"),
			Port:             utils.GetStringEnv("PG_PORT", "5432"),
			User:             utils.GetRequiredStringEnv("PG_USER"),
			Password:         utils.GetRequiredStringEnv("PG_PASSWORD"),
			Database:         "marble",
			ConnectionString: "",
		},
	}
}
