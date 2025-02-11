package usecases

import (
	"context"
	"fmt"
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
	txFac := executor_factory.NewTransactionFactoryStub(exec)

	uc := SanctionCheckUsecase{
		enforceSecurityDecision: enforceSecurity,
		enforceSecurityCase:     enforceSecurity,
		organizationRepository:  mock,
		externalRepository:      mock,
		inboxReader:             mock,
		repository:              &repositories.MarbleDbRepository{},
		executorFactory:         exec,
		transactionFactory:      txFac,
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
			sc.id, sc.decision_id, sc.status, sc.search_input, sc.search_datasets, sc.match_threshold, sc.match_limit, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.initial_has_matches, sc.whitelisted_entities, sc.created_at, sc.updated_at,
			ARRAY_AGG\(ROW\(scm.id,scm.sanction_check_id,scm.opensanction_entity_id,scm.status,scm.query_ids,scm.counterparty_id,scm.payload,scm.reviewed_by,scm.created_at,scm.updated_at\)\) FILTER \(WHERE scm.id IS NOT NULL\) AS matches
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

func TestUpdateMatchStatus(t *testing.T) {
	uc, exec := buildSanctionCheckUsecaseMock()

	_, mockScmRow := utils.FakeStruct[dbmodels.DBSanctionCheckMatch](ops.WithCustomFieldProvider(
		"SanctionCheckId", func() (interface{}, error) {
			return "sanction_check_id", nil
		}),
		ops.WithCustomFieldProvider(
			"Id", func() (interface{}, error) {
				return "matchid", nil
			}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "pending", nil
		}))

	i := 0

	mockOtherScms, mockOtherScmRows := utils.FakeStructs[dbmodels.DBSanctionCheckMatch](3, ops.WithCustomFieldProvider(
		"SanctionCheckId", func() (interface{}, error) {
			return "sanction_check_id", nil
		}),
		ops.WithCustomFieldProvider(
			"Id", func() (interface{}, error) {
				i += 1
				return fmt.Sprintf("otherMatchId_%d", i), nil
			}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "pending", nil
		}))

	_, mockScRow := utils.FakeStruct[dbmodels.DBSanctionCheck](ops.WithCustomFieldProvider(
		"Id", func() (interface{}, error) {
			return "sanction_check_id", nil
		}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "in_review", nil
		}))

	exec.Mock.
		ExpectQuery(`SELECT id, sanction_check_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, reviewed_by, created_at, updated_at FROM sanction_check_matches WHERE id = \$1`).
		WithArgs("matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchesColumn).
			AddRow(mockScmRow...),
		)
	exec.Mock.
		ExpectQuery(`SELECT id, decision_id, status, search_input, search_datasets, match_threshold, match_limit, is_manual, requested_by, is_partial, is_archived, initial_has_matches, whitelisted_entities, created_at, updated_at FROM sanction_checks WHERE id = \$1`).
		WithArgs("sanction_check_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionChecksColumn).
			AddRow(mockScRow...),
		)
	exec.Mock.ExpectQuery(`SELECT id, sanction_check_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, reviewed_by, created_at, updated_at FROM sanction_check_matches WHERE sanction_check_id = \$1`).
		WithArgs("sanction_check_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchesColumn).
			AddRow(mockScmRow...).
			AddRows(mockOtherScmRows...),
		)
	exec.Mock.ExpectQuery(`UPDATE sanction_check_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,sanction_check_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,reviewed_by,created_at,updated_at`).
		WithArgs(models.UserId(""), models.SanctionMatchStatusConfirmedHit, "NOW()", "matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchesColumn).
			AddRow(mockScmRow...),
		)

	for i := range 3 {
		exec.Mock.ExpectQuery(`UPDATE sanction_check_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,sanction_check_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,reviewed_by,created_at,updated_at`).
			WithArgs(models.UserId(""), models.SanctionMatchStatusSkipped, "NOW()", mockOtherScms[i].Id).
			WillReturnRows(pgxmock.NewRows(dbmodels.SelectSanctionCheckMatchesColumn).
				AddRow(mockOtherScmRows[i]...),
			)
	}

	exec.Mock.ExpectExec(`UPDATE sanction_checks SET status = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(models.SanctionStatusConfirmedHit.String(), "NOW()", "sanction_check_id").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	_, err := uc.UpdateMatchStatus(context.TODO(), models.SanctionCheckMatchUpdate{
		MatchId: "matchid",
		Status:  models.SanctionMatchStatusConfirmedHit,
	})
	assert.NoError(t, err)
	assert.NoError(t, exec.Mock.ExpectationsWereMet())
}
