package models

import (
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
)

const OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY = 1 * time.Hour

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter
	Config    SanctionCheckConfig
	OrgConfig OrganizationOpenSanctionsConfig
}

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsUpstreamDataset struct {
	Version    string
	Name       string
	LastExport time.Time
	Schedule   string
}

type OpenSanctionsDataset struct {
	Upstream OpenSanctionsUpstreamDataset
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
func (dataset *OpenSanctionsDataset) CheckIsUpToDate(tp TimeProvider) error {
	if dataset.Upstream.Version == dataset.Version {
		(*dataset).UpToDate = true
		return nil
	}

	if !gronx.New().IsValid(dataset.Upstream.Schedule) {
		return errors.New("could not parse dataset schedule")
	}

	// TODO: this check is not very relevant, since we do not have the date the data was pulled.
	tickAfterLastUpdate, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.LastExport, false)

	if tickAfterLastUpdate.Add(OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY).Before(dataset.Upstream.LastExport) {
		(*dataset).UpToDate = false
		return nil
	}

	tickAfterLastChange, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.Upstream.LastExport, false)

	if tp().After(tickAfterLastChange.Add(OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY)) {
		(*dataset).UpToDate = false
		return nil
	}

	(*dataset).UpToDate = true

	return nil
}
