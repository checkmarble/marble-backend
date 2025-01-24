package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func buildUsecase() (SanctionCheckUsecase, executor_factory.ExecutorFactoryStub) {
	enforceSecurity := sanctionCheckEnforcerMock{}
	mock := sanctionCheckRepositoryMock{}
	exec := executor_factory.NewExecutorFactoryStub()

	uc := SanctionCheckUsecase{
		enforceSecurityDecision: enforceSecurity,
		enforceSecurityCase:     enforceSecurity,
		organizationRepository:  mock,
		decisionRepository:      mock,
		inboxRepository:         mock,
		repository:              &repositories.MarbleDbRepository{},
		executorFactory:         exec,
	}

	return uc, exec
}

func TestGetSanctionCheckOnDecision(t *testing.T) {
	uc, exec := buildUsecase()
	mockSc, mockScRow := utils.FakeStruct[dbmodels.DBSanctionCheck]()
	mockScMatch, mockScMatchRow := utils.FakeStruct[dbmodels.DBSanctionCheckMatchWithComments]()

	exec.Mock.ExpectQuery(`
		SELECT .*
		FROM sanction_checks
		WHERE decision_id = \$1 AND is_archived = \$2
	`).
		WithArgs("decisionid", false).
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectSanctionChecksColumn).
				AddRow(mockScRow...),
		)

	exec.Mock.ExpectQuery(`
		SELECT .*
		FROM sanction_check_matches matches
		LEFT JOIN sanction_check_match_comments comments ON matches.id = comments.sanction_check_match_id
		WHERE sanction_check_id = \$1
		GROUP BY matches.id
	`).
		WithArgs(mockSc.Id).
		WillReturnRows(
			pgxmock.NewRows(utils.ColumnList[dbmodels.DBSanctionCheckMatchWithComments]()).
				AddRow(mockScMatchRow...).
				AddRow(utils.FakeStructRow[dbmodels.DBSanctionCheckMatchWithComments]()...).
				AddRow(utils.FakeStructRow[dbmodels.DBSanctionCheckMatchWithComments]()...),
		)

	scs, err := uc.ListSanctionChecks(context.TODO(), "decisionid")

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, scs, 1)
	assert.Equal(t, mockSc.Status, scs[0].Status)
	assert.Len(t, scs[0].Matches, 3)
	assert.Equal(t, mockScMatch.Status, scs[0].Matches[0].Status)
	assert.Equal(t, mockScMatch.CommentCount, scs[0].Matches[0].CommentCount)
}

func TestListSanctionCheckOnMatchComments(t *testing.T) {
	uc, exec := buildUsecase()
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
	assert.Equal(t, mockComments[0].SanctionCheckMatchId, string(comms[0].MatchId))
}
