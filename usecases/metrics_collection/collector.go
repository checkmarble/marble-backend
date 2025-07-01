package metrics_collection

import "context"

// GlobalCollector is a collector that is not specific to an organization.
// It is used to collect metrics that are not specific to an organization.
// For example, the app version, the number of users
type GlobalCollector interface {
	Name() string
	Collect(ctx context.Context) ([]MetricData, error)
}

// Collector is a collector that is specific to an organization.
type Collector interface {
	Name() string
	Collect(ctx context.Context, orgId string) ([]MetricData, error)
}

type Collectors struct {
	Version          string
	GlobalCollectors []GlobalCollector
	Collectors       []Collector
}

func (c Collectors) GetGlobalCollectors() []GlobalCollector {
	return c.GlobalCollectors
}

func (c Collectors) GetCollectors() []Collector {
	return c.Collectors
}

func NewCollectorV1() Collectors {
	return Collectors{
		Version: "v1",
		Collectors: []Collector{
			NewStubCollector(),
		},
	}
}
