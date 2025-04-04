package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBCase struct {
	Id             pgtype.Text      `db:"id"`
	CreatedAt      pgtype.Timestamp `db:"created_at"`
	InboxId        pgtype.Text      `db:"inbox_id"`
	Name           pgtype.Text      `db:"name"`
	OrganizationId pgtype.Text      `db:"org_id"`
	AssignedTo     *string          `db:"assigned_to"`
	Status         pgtype.Text      `db:"status"`
	Outcome        pgtype.Text      `db:"outcome"`
	SnoozedUntil   *time.Time       `db:"snoozed_until"`
}

type DBCaseWithContributorsAndTags struct {
	DBCase
	Contributors   []DBCaseContributor `db:"contributors"`
	Tags           []DBCaseTag         `db:"tags"`
	DecisionsCount int                 `db:"decisions_count"`
}

type DBPaginatedCases struct {
	DBCaseWithContributorsAndTags
	RankNumber int `db:"rank_number"`
}

const TABLE_CASES = "cases"

var SelectCaseColumn = []string{"id", "created_at", "inbox_id", "name", "org_id", "assigned_to", "status", "outcome", "snoozed_until"}

func AdaptCase(db DBCase) (models.Case, error) {
	var assigneeId *models.UserId
	if db.AssignedTo != nil {
		assigneeId = utils.Ptr(models.UserId(*db.AssignedTo))
	}

	return models.Case{
		Id:             db.Id.String,
		CreatedAt:      db.CreatedAt.Time,
		InboxId:        db.InboxId.String,
		Name:           db.Name.String,
		OrganizationId: db.OrganizationId.String,
		AssignedTo:     assigneeId,
		Status:         models.CaseStatus(db.Status.String),
		Outcome:        models.CaseOutcome(db.Outcome.String),
		SnoozedUntil:   db.SnoozedUntil,
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

func AdaptCaseWithRank(db DBCaseWithContributorsAndTags, rankNumber int) (models.CaseWithRank, error) {
	c, err := AdaptCaseWithContributorsAndTags(db)
	if err != nil {
		return models.CaseWithRank{}, err
	}
	return models.CaseWithRank{
		Case:       c,
		RankNumber: rankNumber,
	}, nil
}
