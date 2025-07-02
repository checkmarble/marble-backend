package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// GlobalCollector is a collector that is not specific to an organization.
// It is used to collect metrics that are not specific to an organization.
// For example, the app version, the number of users
type GlobalCollector interface {
	Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error)
}

// Collector is a collector that is specific to an organization.
type Collector interface {
	Collect(ctx context.Context, orgId string, from time.Time, to time.Time) ([]models.MetricData, error)
}

type Collectors struct {
	version          string
	globalCollectors []GlobalCollector
	collectors       []Collector
}

func (c Collectors) GetGlobalCollectors() []GlobalCollector {
	return c.globalCollectors
}

func (c Collectors) GetCollectors() []Collector {
	return c.collectors
}

func (c Collectors) GetVersion() string {
	return c.version
}

// Use version to track the version of the collectors, could be used to track changes
// and tell the server which collectors is used by the client
func NewCollectorsV1() Collectors {
	return Collectors{
		version: "v1",
		collectors: []Collector{
			NewStubOrganizationCollector(),
		},
		globalCollectors: []GlobalCollector{
			NewStubGlobalCollector(),
		},
	}
}
