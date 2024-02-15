package utils

import "fmt"

type PGConfig struct {
	Hostname         string
	Port             string
	User             string
	Password         string
	Database         string
	ConnectionString string
}

func (config PGConfig) GetConnectionString(env string) string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		config.Hostname, config.User, config.Password, config.Database)
	if env == "development" {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}
