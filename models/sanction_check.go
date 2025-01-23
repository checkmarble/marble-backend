package models

import (
	"encoding/json"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
)

var OPEN_SACTIONS_OUTDATED_DATASET_LEEWAY = 1 * time.Hour

type OpenSanctionsUpstreamDataset struct {
	Version   string    `json:"version"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Schedule  string    `json:"-"`
}

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsDataset struct {
	Upstream OpenSanctionsUpstreamDataset `json:"upstream"`
	Version  string                       `json:"version"`
	UpToDate bool                         `json:"up_to_date"`

	// TODO: this is not the date at which the data was pulled, it is when the data was published
	UpdatedAt time.Time `json:"-"`
}

func (dataset *OpenSanctionsDataset) CheckIsUpToDate() error {
	if dataset.Upstream.Version == dataset.Version {
		(*dataset).UpToDate = true
		return nil
	}

	if !gronx.New().IsValid(dataset.Upstream.Schedule) {
		return errors.New("could not parse dataset schedule")
	}

	// TODO: this check is not very relevant, since we do not have the date the data was pulled.
	tickAfterLastUpdate, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.UpdatedAt, false)

	if tickAfterLastUpdate.Add(OPEN_SACTIONS_OUTDATED_DATASET_LEEWAY).Before(dataset.Upstream.UpdatedAt) {
		(*dataset).UpToDate = false
		return nil
	}

	tickAfterLastChange, _ := gronx.NextTickAfter(dataset.Upstream.Schedule, dataset.Upstream.UpdatedAt, false)

	if time.Now().After(tickAfterLastChange.Add(OPEN_SACTIONS_OUTDATED_DATASET_LEEWAY)) {
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
