package utils

import "fmt"

type PGConfig struct {
	ConnectionString    string
	Database            string
	DbConnectWithSocket bool
	Hostname            string
	Password            string
	Port                string
	User                string
}

func (config PGConfig) GetConnectionString() string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		config.Hostname, config.User, config.Password, config.Database)
	if !config.DbConnectWithSocket {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}
