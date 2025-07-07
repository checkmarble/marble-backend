package infra

import (
	"context"

	"cloud.google.com/go/bigquery"
)

type BigQueryConfig struct {
	ProjectID string
}

type BigQueryClient struct {
	Client *bigquery.Client
	config BigQueryConfig
}

func NewBigQueryClient(ctx context.Context, config BigQueryConfig) (*BigQueryClient, error) {
	client, err := bigquery.NewClient(ctx, config.ProjectID)
	if err != nil {
		return nil, err
	}

	return &BigQueryClient{
		Client: client,
		config: config,
	}, nil
}

func (c *BigQueryClient) Close() error {
	return c.Client.Close()
}
