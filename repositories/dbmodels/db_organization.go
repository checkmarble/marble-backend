package dbmodels

import (
	"net"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBOrganizationResult struct {
	Id                      uuid.UUID   `db:"id"`
	PublicId                uuid.UUID   `db:"public_id"`
	DeletedAt               *int        `db:"deleted_at"`
	Name                    string      `db:"name"`
	AllowedNetworks         []net.IPNet `db:"allowed_networks"`
	TransferCheckScenarioId *string     `db:"transfer_check_scenario_id"`
	AiCaseReviewEnabled     bool        `db:"ai_case_review_enabled"`
	DefaultScenarioTimezone *string     `db:"default_scenario_timezone"`
	ScreeningThreshold      int         `db:"sanctions_threshold"`
	ScreeningLimit          int         `db:"sanctions_limit"`
	AutoAssignQueueLimit    int         `db:"auto_assign_queue_limit"`
	SentryReplayEnabled     bool        `db:"sentry_replay_enabled"`
	Environment             string      `db:"environment"`
}

const TABLE_ORGANIZATION = "organizations"

var ColumnsSelectOrganization = utils.ColumnList[DBOrganizationResult]()

func AdaptOrganization(db DBOrganizationResult) (models.Organization, error) {
	return models.Organization{
		Id:                      db.Id,
		PublicId:                db.PublicId,
		Name:                    db.Name,
		WhitelistedSubnets:      db.AllowedNetworks,
		TransferCheckScenarioId: db.TransferCheckScenarioId,
		AiCaseReviewEnabled:     db.AiCaseReviewEnabled,
		DefaultScenarioTimezone: db.DefaultScenarioTimezone,
		OpenSanctionsConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: db.ScreeningThreshold,
			MatchLimit:     db.ScreeningLimit,
		},
		AutoAssignQueueLimit: db.AutoAssignQueueLimit,
		SentryReplayEnabled:  db.SentryReplayEnabled,
		Environment:          models.ParseOrganizationEnvironment(db.Environment),
	}, nil
}

type DbOrganizationWhitelistedSubnets struct {
	AllowedNetworks []net.IPNet `db:"allowed_networks"`
}

func AdaptOrganizationWhitelistedSubnets(db DbOrganizationWhitelistedSubnets) ([]net.IPNet, error) {
	return db.AllowedNetworks, nil
}
