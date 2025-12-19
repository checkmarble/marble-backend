package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBCase struct {
	Id             pgtype.Text      `db:"id"`
	CreatedAt      pgtype.Timestamp `db:"created_at"`
	InboxId        uuid.UUID        `db:"inbox_id"`
	Name           pgtype.Text      `db:"name"`
	OrganizationId pgtype.Text      `db:"org_id"`
	AssignedTo     *string          `db:"assigned_to"`
	Status         pgtype.Text      `db:"status"`
	Outcome        pgtype.Text      `db:"outcome"`
	SnoozedUntil   *time.Time       `db:"snoozed_until"`
	Boost          *string          `db:"boost"`
	Type           pgtype.Text      `db:"type"`
}

type DBCaseWithContributorsAndTags struct {
	DBCase
	Contributors   []DBCaseContributor `db:"contributors"`
	Tags           []DBCaseTag         `db:"tags"`
	DecisionsCount int                 `db:"decisions_count"`
}

const TABLE_CASES = "cases"

var SelectCaseColumn = utils.ColumnList[DBCase]()

func AdaptCase(db DBCase) (models.Case, error) {
	var assigneeId *models.UserId
	if db.AssignedTo != nil {
		assigneeId = utils.Ptr(models.UserId(*db.AssignedTo))
	}

	var boostReason *models.BoostReason
	if db.Boost != nil {
		boostReason = utils.Ptr(models.BoostReason(*db.Boost))
	}

	return models.Case{
		Id:             db.Id.String,
		CreatedAt:      db.CreatedAt.Time,
		InboxId:        db.InboxId,
		Name:           db.Name.String,
		OrganizationId: db.OrganizationId.String,
		AssignedTo:     assigneeId,
		Status:         models.CaseStatus(db.Status.String),
		Outcome:        models.CaseOutcome(db.Outcome.String),
		SnoozedUntil:   db.SnoozedUntil,
		Boost:          boostReason,
		Type:           models.CaseTypeFromString(db.Type.String),
	}, nil
}

func AdaptCaseWithContributorsAndTags(db DBCaseWithContributorsAndTags) (models.Case, error) {
	caseModel, err := AdaptCase(db.DBCase)
	if err != nil {
		return models.Case{}, err
	}
	caseModel.DecisionsCount = db.DecisionsCount

	caseModel.Contributors = make([]models.CaseContributor, len(db.Contributors))
	for i, contributor := range db.Contributors {
		caseModel.Contributors[i], err = AdaptCaseContributor(contributor)
		if err != nil {
			return models.Case{}, err
		}
	}

	caseModel.Tags = make([]models.CaseTag, len(db.Tags))
	for i, tag := range db.Tags {
		caseModel.Tags[i], err = AdaptCaseTag(tag)
		if err != nil {
			return models.Case{}, err
		}
	}

	return caseModel, nil
}

type CaseReferents struct {
	Id       string        `db:"id"`
	Inbox    DBInbox       `db:"inbox"`
	Assignee *DBUserResult `db:"assignee"`
}

func AdaptCaseReferents(c CaseReferents) (models.CaseReferents, error) {
	var assignee *models.User

	if c.Assignee != nil {
		u, err := AdaptUser(*c.Assignee)
		if err != nil {
			return models.CaseReferents{}, err
		}

		assignee = &u
	}

	inbox, err := AdaptInbox(c.Inbox)
	if err != nil {
		return models.CaseReferents{}, err
	}

	return models.CaseReferents{
		Id:       c.Id,
		Inbox:    inbox,
		Assignee: assignee,
	}, nil
}
