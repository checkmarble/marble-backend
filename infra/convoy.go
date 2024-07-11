package infra

import (
	"context"
	"fmt"
	"net/http"

	convoy "github.com/checkmarble/marble-backend/api-clients/convoy"
)

type ConvoyRessources struct {
	projectID    string
	convoyClient *convoy.ClientWithResponses
}

func InitializeConvoyRessources(config ConvoyConfiguration) ConvoyRessources {
	convoyClient, err := convoy.NewClientWithResponses(config.APIUrl,
		convoy.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.APIKey))
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Errorf("error initializing convoy client: %w", err))
	}

	return ConvoyRessources{
		convoyClient: convoyClient,
		projectID:    config.ProjectID,
	}
}

func (r ConvoyRessources) GetClient() (convoy.ClientWithResponses, error) {
	client := r.convoyClient
	if client == nil {
		return convoy.ClientWithResponses{}, fmt.Errorf("convoy client is not initialized")
	}
	return *client, nil
}

func (r ConvoyRessources) GetProjectID() string {
	return r.projectID
}
