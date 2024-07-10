package infra

import (
	convoy "github.com/frain-dev/convoy-go/v2"
)

type ConvoyRessources struct {
	convoyClient *convoy.Client
}

func InitializeConvoyRessources(config ConvoyConfiguration) ConvoyRessources {
	convoyClient := convoy.New(
		config.APIUrl,
		config.APIKey,
		config.ProjectID,
	)

	return ConvoyRessources{
		convoyClient: convoyClient,
	}
}

func (r ConvoyRessources) GetClient() (*convoy.Client, error) {
	return r.convoyClient, nil
}
