// Provide a BigQuery client to send data to BigQuery.
// Usage:
// config := infra.BigQueryConfig{
// 	ProjectID: "your-project-id",
// 	MetricsDataset: "your-metrics-dataset",
// 	MetricsTable:   "your-metrics-table",
// }
// bigQueryClient, err := infra.NewBigQueryClient(ctx, config)
// if err != nil {
// 	return err
// }
// defer bigQueryClient.Close()
// ...

package infra

import (
	"context"

	"cloud.google.com/go/bigquery"
)

// Could be moved to a config file
const (
	MetricsDataset = "metrics"
	MetricsTable   = "metrics_raw"
)

type BigQueryConfig struct {
	ProjectID string

	// For metrics
	MetricsDataset string
	MetricsTable   string
}

type BigQueryClient struct {
	Client *bigquery.Client
	Config BigQueryConfig
}

func NewBigQueryClient(ctx context.Context, config BigQueryConfig) (*BigQueryClient, error) {
	client, err := bigquery.NewClient(ctx, config.ProjectID)
	if err != nil {
		return nil, err
	}

	return &BigQueryClient{
		Client: client,
		Config: config,
	}, nil
}

func (c *BigQueryClient) Close() error {
	return c.Client.Close()
}
