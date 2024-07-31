package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SNOOZE_GROUPS = "snooze_groups"

var SelectSnoozeGroupsColumn = utils.ColumnList[DBSnoozeGroup]()

type DBSnoozeGroup struct {
	Id             string    `db:"id"`
	OrganizationId string    `db:"org_id"`
	CreatedAt      time.Time `db:"created_at"`
}

func AdaptSnoozeGroup(s DBSnoozeGroup) (models.SnoozeGroup, error) {
	return models.SnoozeGroup{
		Id:             s.Id,
		OrganizationId: s.OrganizationId,
		CreatedAt:      s.CreatedAt,
	}, nil
}

const TABLE_RULE_SNOOZES = "rule_snoozes"

var SelectRuleSnoozesColumn = utils.ColumnList[DBRuleSnooze]()

type DBRuleSnooze struct {
	Id            string    `db:"id"`
	CreatedByUser string    `db:"created_by_user"`
	SnoozeGroupId string    `db:"snooze_group_id"`
	PivotValue    string    `db:"pivot_value"`
	StartsAt      time.Time `db:"starts_at"`
	ExpiresAt     time.Time `db:"expires_at"`
}

func AdaptRuleSnooze(s DBRuleSnooze) (models.RuleSnooze, error) {
	return models.RuleSnooze{
		Id:            s.Id,
		CreatedByUser: s.CreatedByUser,
		SnoozeGroupId: s.SnoozeGroupId,
		PivotValue:    s.PivotValue,
		StartsAt:      s.StartsAt,
		ExpiresAt:     s.ExpiresAt,
	}, nil
}
