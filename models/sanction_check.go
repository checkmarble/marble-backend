package models

import (
	"encoding/json"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
)

var OPEN_SANCTIONS_OUTDATED_DATASET_LEEWAY = 1 * time.Hour

type OpenSanctionsUpstreamDataset struct {
	Version    string    `json:"version"`
	Name       string    `json:"name"`
	LastExport time.Time `json:"last_export"`
	Schedule   string    `json:"-"`
}

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsDataset struct {
	Upstream OpenSanctionsUpstreamDataset `json:"upstream"`
	Version  string                       `json:"version"`
	UpToDate bool                         `json:"up_to_date"`

	// TODO: this is not the date at which the data was pulled, it is when the data was published
	LastExport time.Time `json:"-"`
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

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter
	OrgConfig OrganizationOpenSanctionsConfig
}

type SanctionCheck struct {
	Id          string
	DecisionId  string
	Status      string
	Query       json.RawMessage
	OrgConfig   OrganizationOpenSanctionsConfig
	IsManual    bool
	RequestedBy *string
	Partial     bool
	Count       int
	Matches     []SanctionCheckMatch
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SanctionCheckMatch struct {
	Id              string
	SanctionCheckId string
	EntityId        string
	Status          string
	QueryIds        []string
	Payload         []byte
	ReviewedBy      *string
	CommentCount    int
}

type SanctionCheckMatchUpdate struct {
	MatchId    string
	ReviewerId UserId
	Status     string
}

type SanctionCheckMatchComment struct {
	Id          string
	MatchId     string
	CommenterId UserId
	Comment     string
	CreatedAt   time.Time
}
