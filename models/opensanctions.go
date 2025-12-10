package models

import (
	"time"

	"github.com/adhocore/gronx"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

const OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY = 1 * time.Hour

// Row structure representing a dataset in the OpenSanctions catalog
// Note: I didn't declare all fields, only those needed for our logic, don't hesitate to add more if needed
type OpenSanctionsRawDataset struct {
	Name     string
	Version  string
	Load     bool
	DeltaUrl *string
}

// Define a raw catalog structure, close to the OpenSanctions API response
// compared to the OpenSanctionsCatalog struct
type OpenSanctionsRawCatalog struct {
	Datasets map[string]OpenSanctionsRawDataset
	Current  []string
	Outdated []string
}

type OpenSanctionsCatalog struct {
	Sections []OpenSanctionsCatalogSection
	Tags     *expirable.LRU[string, []string]
}

type OpenSanctionsCatalogSection struct {
	Name     string
	Title    string
	Datasets []OpenSanctionsCatalogDataset
}

type OpenSanctionsCatalogDataset struct {
	Name  string
	Title string
	Tags  []string
	Path  set.Set[string]
}

type OpenSanctionsQuery struct {
	IsRefinement       bool
	EffectiveThreshold int
	LimitIncrease      int
	Queries            []OpenSanctionsCheckQuery
	InitialQuery       []OpenSanctionsCheckQuery
	Config             ScreeningConfig
	OrgConfig          OrganizationOpenSanctionsConfig
	// cf: `exclude_entity_ids` in the OpenSanctions query
	WhitelistedEntityIds []string
}

type OpenSanctionsCheckQuery struct {
	Type    string              `json:"schema"`     //nolint:tagliatelle
	Filters OpenSanctionsFilter `json:"properties"` //nolint:tagliatelle
}

func (q OpenSanctionsCheckQuery) GetName() string {
	if names, ok := q.Filters["name"]; ok {
		if len(names) > 0 {
			return names[0]
		}
	}

	return ""
}

func (q *OpenSanctionsCheckQuery) SetName(name string) {
	q.Filters["name"] = []string{name}
}

// func (q OpenSanctionsCheckQuery) String() string {
// 	m := make(map[string][]string, len(q.Filters))

// 	for k := range q.Filters {
// 		m[k] = make([]string, len(q.Filters[k]))

// 		for idx := range q.Filters[k] {
// 			m[k][idx] = "[redacted]"
// 		}
// 	}

// 	return fmt.Sprintf("%s (%s)", q.Type, m)
// }

type OpenSanctionsFilter map[string][]string

var OPEN_SANCTIONS_ABSTRACT_TYPES_MAPPING = map[string][]string{
	"Vehicle": {"Airplane", "Vessel"},
}

func AdaptRefineRequestToMatchable(refine ScreeningRefineRequest) []OpenSanctionsCheckQuery {
	switch mappings, abstract := OPEN_SANCTIONS_ABSTRACT_TYPES_MAPPING[refine.Type]; abstract {
	case true:
		return pure_utils.Map(mappings, func(m string) OpenSanctionsCheckQuery {
			return OpenSanctionsCheckQuery{Type: m, Filters: refine.Query}
		})

	default:
		return []OpenSanctionsCheckQuery{
			{
				Type:    refine.Type,
				Filters: refine.Query,
			},
		}
	}
}

type OpenSanctionsUpstreamDatasetFreshness struct {
	Version    string
	Name       string
	LastExport time.Time
	Schedule   string
}

type OpenSanctionsDatasetFreshness struct {
	Upstream OpenSanctionsUpstreamDatasetFreshness
	Version  string
	UpToDate bool

	// TODO: this is not the date at which the data was pulled, it is when the data was published
	LastExport time.Time
}

type TimeProvider func() time.Time

// CheckIsUpToDate marks a dataset as outdated if it was not updated in a
// reasonable window of time after the upstream dataset was.
//
// Considering that for an upstream update time of T, the duration for which
// we consider the dataset as "up to date" is:
//
//   - Grace Period = Time until the next scheduled update + Leeway
//
// For example, with a leeway of 1 hour, if a dataset is set to be pulled every
// two hours, and the upstream dataset is updated at 7am, we will consider the
// dataset as outdated if it is not updated at 9am.
//
//   - Outdated if (now() > Local Dataset Export Date + Grace Period)
//
// Since the upstream dataset is continuously updated, this rule alone is not
// enough, so we also consider a dataset as oudated if the upstream export date
// is after that of the local dataset + the update formula above.
//
//   - Outdated if (Local Dataset Export Date + Grace Period < Upstream Dataset Export Date)
//
// The local dataset is always considered up to date if its version matches
// that of its upstream counterpart.
func (dataset *OpenSanctionsDatasetFreshness) CheckIsUpToDate(tp TimeProvider) error {
	if dataset == nil {
		return errors.New("trying to check freshness on a nil dataset")
	}

	if dataset.Upstream.Version == dataset.Version {
		dataset.UpToDate = true
		return nil
	}

	if !gronx.New().IsValid(dataset.Upstream.Schedule) {
		return errors.New("could not parse dataset schedule")
	}

	// TODO: this check is not very relevant, since we do not have the date the data was pulled.
	tickAfterLastUpdate, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.LastExport, false)

	if tickAfterLastUpdate.Add(OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY).Before(dataset.Upstream.LastExport) {
		dataset.UpToDate = false
		return nil
	}

	tickAfterLastChange, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.Upstream.LastExport, false)

	if tp().After(tickAfterLastChange.Add(OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY)) {
		dataset.UpToDate = false
		return nil
	}

	dataset.UpToDate = true

	return nil
}

type OpenSanctionAlgorithm struct {
	Name        string
	Description string
}

type OpenSanctionAlgorithms struct {
	Algorithms []OpenSanctionAlgorithm
	Best       string
	Default    string
}

func (algorithms OpenSanctionAlgorithms) GetAlgorithm(name string) (OpenSanctionAlgorithm, error) {
	for _, algorithm := range algorithms.Algorithms {
		if algorithm.Name == name {
			return algorithm, nil
		}
	}
	return OpenSanctionAlgorithm{}, errors.Newf("algorithm %s not found", name)
}
