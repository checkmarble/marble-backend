// Provide a BigQuery client to send data to BigQuery.
// Usage:
// config := infra.BigQueryConfig{
// 	ProjectID: "your-project-id",
// 	MetricsDataset: "your-metrics-dataset",
// 	MetricsTable:   "your-metrics-table",
// }
// bigQueryInfra, err := infra.InitializeBigQueryInfra(ctx, config)
// if err != nil {
// 	return err
// }
// defer bigQueryClient.Close()
// ...

package infra

import (
	"context"

	"cloud.google.com/go/bigquery"
	"github.com/cockroachdb/errors"
)

// Could be moved to a config file
const (
	MetricsDataset = "metrics"
	MetricsTable   = "metrics_raw"
)

type BigQueryInfra struct {
	client *bigquery.Client
	// Use directly the table, don't need to manage the client
	MetricsTable *bigquery.Table
}

type BigQueryConfig struct {
	ProjectID      string
	MetricsDataset string
	MetricsTable   string
}

func InitializeBigQueryInfra(ctx context.Context, config BigQueryConfig) (*BigQueryInfra, error) {
	// Init BigQuery client only for marble saas projects
	if !IsMarbleSaasProject() {
		return nil, errors.New("project id is not a marble saas project")
	}

	client, err := bigquery.NewClient(ctx, config.ProjectID)
	if err != nil {
		return nil, err
	}

	return &BigQueryInfra{
		client:       client,
		MetricsTable: client.Dataset(config.MetricsDataset).Table(config.MetricsTable),
	}, nil
}

func (c *BigQueryInfra) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}
