package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	ops "github.com/go-faker/faker/v4/pkg/options"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func buildSanctionCheckUsecaseMock() (SanctionCheckUsecase, executor_factory.ExecutorFactoryStub) {
	enforceSecurity := sanctionCheckEnforcerMock{}
	mock := sanctionCheckRepositoryMock{}
	exec := executor_factory.NewExecutorFactoryStub()

	uc := SanctionCheckUsecase{
		enforceSecurityDecision: enforceSecurity,
		enforceSecurityCase:     enforceSecurity,
		organizationRepository:  mock,
		decisionRepository:      mock,
		inboxReader:             mock,
		repository:              &repositories.MarbleDbRepository{},
		executorFactory:         exec,
	}

	return uc, exec
}

func TestListSanctionChecksOnDecision(t *testing.T) {
	uc, exec := buildSanctionCheckUsecaseMock()
	mockSc, mockScRow := utils.FakeStruct[dbmodels.DBSanctionCheck](
		ops.WithRandomMapAndSliceMinSize(1))

	exec.Mock.ExpectQuery(`
		SELECT sc.id, sc.decision_id, sc.status, sc.search_input, sc.search_datasets, sc.search_threshold, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.created_at, sc.updated_at, sc.matches,
			ARRAY_AGG\(ROW\(\[scm.id scm.sanction_check_id scm.opensanction_entity_id scm.status scm.query_ids scm.payload scm.reviewed_by scm.created_at scm.updated_at\]\)\) AS matches
		FROM sanction_checks AS sc
		LEFT JOIN sanction_check_matches AS scm ON sc.id = scm.sanction_check_id
		WHERE sc.decision_id = \$1
			AND sc.is_archived = \$2
	`).
		WithArgs("decisionid", false).
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectSanctionChecksColumn).
				AddRow(mockScRow...),
		)

	scs, err := uc.ListSanctionChecks(context.TODO(), "decisionid")

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, scs, 1)
	assert.Equal(t, models.SanctionCheckStatusFrom(mockSc.Status), scs[0].Status)
	assert.NotEmpty(t, scs[0].Matches)
	assert.Equal(t, models.SanctionCheckMatchStatusFrom(scs[0].Matches[0].Status.String()), models.SanctionMatchStatusUnknown)
}

func TestListSanctionCheckOnMatchComments(t *testing.T) {
	uc, exec := buildSanctionCheckUsecaseMock()
	mockMatch, mockMatchRow := utils.FakeStruct[dbmodels.DBSanctionCheckMatch]()
	_, mockCheckRow := utils.FakeStruct[dbmodels.DBSanctionCheck]()
	mockComments, mockCommentsRows := utils.FakeStructs[dbmodels.DBSanctionCheckMatchComment](4)

	exec.Mock.ExpectQuery(`SELECT .* FROM sanction_check_matches WHERE id = \$1`).
		WithArgs("matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchesColumn).AddRow(mockMatchRow...))

	exec.Mock.ExpectQuery(`SELECT .* FROM sanction_checks WHERE id = \$1 `).
		WithArgs(mockMatch.SanctionCheckId).
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionChecksColumn).AddRow(mockCheckRow...))

	exec.Mock.ExpectQuery(`
		SELECT .*
		FROM sanction_check_match_comments
		WHERE sanction_check_match_id = \$1
		ORDER BY created_at ASC
	`).
		WithArgs("matchid").
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchCommentsColumn).
				AddRows(mockCommentsRows...),
		)

	comms, err := uc.MatchListComments(context.TODO(), "matchid")

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, comms, 4)
	assert.Equal(t, mockComments[0].Id, comms[0].Id)
	assert.Equal(t, mockComments[0].Comment, comms[0].Comment)
	assert.Equal(t, mockComments[0].CommentedBy, string(comms[0].CommenterId))
	assert.Equal(t, mockComments[0].CreatedAt, comms[0].CreatedAt)
	assert.Equal(t, mockComments[0].SanctionCheckMatchId, comms[0].MatchId)
}
