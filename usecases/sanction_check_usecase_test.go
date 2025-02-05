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
	mockSc, mockScRow := utils.FakeStruct[dbmodels.DBSanctionCheckWithMatches](
		ops.WithRandomMapAndSliceMinSize(1), ops.WithRandomMapAndSliceMaxSize(1))

	mockComments, mockCommentsRows := utils.FakeStructs[dbmodels.DBSanctionCheckMatchComment](
		4,
		ops.WithCustomFieldProvider("SanctionCheckMatchId", func() (interface{}, error) {
			return mockSc.Matches[0].Id, nil
		}),
	)

	exec.Mock.ExpectQuery(`
		SELECT
			sc.id, sc.decision_id, sc.status, sc.search_input, sc.search_datasets, sc.search_threshold, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.created_at, sc.updated_at,
			ARRAY_AGG\(ROW\(scm.id,scm.sanction_check_id,scm.opensanction_entity_id,scm.status,scm.query_ids,scm.payload,scm.reviewed_by,scm.created_at,scm.updated_at\)\) AS matches
		FROM sanction_checks AS sc
		LEFT JOIN sanction_check_matches AS scm ON sc.id = scm.sanction_check_id
		WHERE sc.decision_id = \$1 AND sc.is_archived = \$2
		GROUP BY sc.id
	`).
		WithArgs("decisionid", false).
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectSanctionChecksWithMatchesColumn).
				AddRow(mockScRow...),
		)

	exec.Mock.ExpectQuery(`SELECT .* FROM sanction_check_match_comments WHERE sanction_check_match_id = ANY\(\$1\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "sanction_check_match_id", "commented_by", "comment", "created_at"}).
				AddRows(mockCommentsRows...),
		)

	scs, err := uc.ListSanctionChecks(context.TODO(), "decisionid")

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, scs, 1)
	assert.Equal(t, models.SanctionCheckStatusFrom(mockSc.Status), scs[0].Status)
	assert.NotEmpty(t, scs[0].Matches)
	assert.Equal(t, models.SanctionCheckMatchStatusFrom(scs[0].Matches[0].Status.String()), models.SanctionMatchStatusUnknown)
	assert.Len(t, scs[0].Matches[0].Comments, 4)
	assert.Equal(t, scs[0].Matches[0].Comments[0].Comment, mockComments[0].Comment)
}
