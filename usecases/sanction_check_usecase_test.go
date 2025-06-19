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
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func buildScreeningUsecaseMock() (ScreeningUsecase, executor_factory.ExecutorFactoryStub) {
	enforceSecurity := screeningEnforcerMock{}
	repoMock := screeningRepositoryMock{}
	exec := executor_factory.NewExecutorFactoryStub()
	txFac := executor_factory.NewTransactionFactoryStub(exec)

	caseUsecaseMock := ScreeningCaseUsecaseMock{}
	caseUsecaseMock.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	uc := ScreeningUsecase{
		enforceSecurityDecision:   enforceSecurity,
		enforceSecurityCase:       enforceSecurity,
		caseUsecase:               &caseUsecaseMock,
		organizationRepository:    repoMock,
		externalRepository:        repoMock,
		inboxReader:               repoMock,
		repository:                &repositories.MarbleDbRepository{},
		screeningConfigRepository: &repositories.MarbleDbRepository{},
		executorFactory:           exec,
		transactionFactory:        txFac,
	}

	return uc, exec
}

func TestListScreeningOnDecision(t *testing.T) {
	uc, exec := buildScreeningUsecaseMock()
	sccId := uuid.NewString()

	sccRow, mockSccRow := utils.FakeStruct[dbmodels.DBScreeningConfigs](
		ops.WithCustomFieldProvider("Id", func() (interface{}, error) {
			return sccId, nil
		}),
		ops.WithCustomFieldProvider("TriggerRule", func() (interface{}, error) {
			return []byte(`{}`), nil
		}),
		ops.WithCustomFieldProvider("Query", func() (interface{}, error) {
			return []byte(`{}`), nil
		}),
		ops.WithCustomFieldProvider("CounterpartyIdExpr", func() (interface{}, error) {
			return []byte(`{}`), nil
		}),
	)

	mockSc, mockScRow := utils.FakeStruct[dbmodels.DBScreeningWithMatches](
		ops.WithRandomMapAndSliceMinSize(1),
		ops.WithRandomMapAndSliceMaxSize(1),
		ops.WithCustomFieldProvider("ScreeningConfigId", func() (interface{}, error) {
			return sccId, nil
		}),
	)

	mockComments, mockCommentsRows := utils.FakeStructs[dbmodels.DBScreeningMatchComment](
		4,
		ops.WithCustomFieldProvider("ScreeningMatchId", func() (interface{}, error) {
			return mockSc.Matches[0].Id, nil
		}),
	)

	exec.Mock.ExpectQuery(escapeSql(`
		SELECT
			sc.id, sc.decision_id, sc.sanction_check_config_id, sc.status, sc.search_input, sc.search_datasets, sc.match_threshold, sc.match_limit, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.initial_has_matches, sc.whitelisted_entities, sc.error_codes, sc.created_at, sc.updated_at,
			ARRAY_AGG(ROW(scm.id,scm.sanction_check_id,scm.opensanction_entity_id,scm.status,scm.query_ids,scm.counterparty_id,scm.payload,scm.enriched,scm.reviewed_by,scm.created_at,scm.updated_at) ORDER BY array_position(.+, scm.status), scm.payload->>'score' DESC) FILTER (WHERE scm.id IS NOT NULL) AS matches
		FROM sanction_checks AS sc
		LEFT JOIN sanction_check_matches AS scm ON sc.id = scm.sanction_check_id
		WHERE sc.decision_id = $1 AND sc.is_archived = $2
		GROUP BY sc.id
	`)).
		WithArgs("decisionid", false).
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectScreeningWithMatchesColumn).
				AddRow(mockScRow...),
		)

	exec.Mock.
		ExpectQuery(`SELECT id, .+ FROM sanction_check_configs WHERE scenario_iteration_id = \$1`).
		WithArgs("").
		WillReturnRows(
			pgxmock.NewRows(dbmodels.ScreeningConfigColumnList).
				AddRow(mockSccRow...),
		)

	exec.Mock.ExpectQuery(`SELECT .* FROM sanction_check_match_comments WHERE sanction_check_match_id = ANY\(\$1\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "sanction_check_match_id", "commented_by", "comment", "created_at"}).
				AddRows(mockCommentsRows...),
		)

	scs, err := uc.ListScreenings(context.TODO(), "decisionid", false)

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, scs, 1)
	assert.Equal(t, models.ScreeningStatusFrom(mockSc.Status), scs[0].Status)
	assert.NotEmpty(t, scs[0].Matches)
	assert.Equal(t, models.ScreeningMatchStatusFrom(scs[0].Matches[0].Status.String()), models.ScreeningMatchStatusUnknown)
	assert.Len(t, scs[0].Matches[0].Comments, 4)
	assert.Equal(t, scs[0].Matches[0].Comments[0].Comment, mockComments[0].Comment)
	assert.Equal(t, scs[0].Config.Name, sccRow.Name)
}

func TestUpdateMatchStatus(t *testing.T) {
	uc, exec := buildScreeningUsecaseMock()
	userId := models.UserId(uuid.NewString())

	_, mockScmRow := utils.FakeStruct[dbmodels.DBScreeningMatch](ops.WithCustomFieldProvider(
		"ScreeningId", func() (interface{}, error) {
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

	mockOtherScms, mockOtherScmRows := utils.FakeStructs[dbmodels.DBScreeningMatch](3, ops.WithCustomFieldProvider(
		"ScreeningId", func() (interface{}, error) {
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

	_, mockScRow := utils.FakeStruct[dbmodels.DBScreening](ops.WithCustomFieldProvider(
		"Id", func() (interface{}, error) {
			return "sanction_check_id", nil
		}),
		ops.WithCustomFieldProvider("IsArchived", func() (interface{}, error) {
			return false, nil
		}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "in_review", nil
		}))

	exec.Mock.
		ExpectQuery(`SELECT id, sanction_check_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, enriched, reviewed_by, created_at, updated_at FROM sanction_check_matches WHERE id = \$1`).
		WithArgs("matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...),
		)
	exec.Mock.
		ExpectQuery(`SELECT id, decision_id, sanction_check_config_id, status, search_input, search_datasets, match_threshold, match_limit, is_manual, requested_by, is_partial, is_archived, initial_has_matches, whitelisted_entities, error_codes, created_at, updated_at FROM sanction_checks WHERE id = \$1`).
		WithArgs("sanction_check_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningColumn).
			AddRow(mockScRow...),
		)
	exec.Mock.ExpectQuery(`SELECT id, sanction_check_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, enriched, reviewed_by, created_at, updated_at FROM sanction_check_matches WHERE sanction_check_id = \$1`).
		WithArgs("sanction_check_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...).
			AddRows(mockOtherScmRows...),
		)
	exec.Mock.ExpectQuery(`UPDATE sanction_check_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,sanction_check_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,enriched,reviewed_by,created_at,updated_at`).
		WithArgs(&userId, models.ScreeningMatchStatusConfirmedHit, "NOW()", "matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...),
		)

	for i := range 3 {
		exec.Mock.ExpectQuery(`UPDATE sanction_check_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,sanction_check_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,enriched,reviewed_by,created_at,updated_at`).
			WithArgs(&userId, models.ScreeningMatchStatusSkipped, "NOW()", mockOtherScms[i].Id).
			WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
				AddRow(mockOtherScmRows[i]...),
			)
	}

	exec.Mock.ExpectExec(`UPDATE sanction_checks SET status = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(models.ScreeningStatusConfirmedHit.String(), "NOW()", "sanction_check_id").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	_, err := uc.UpdateMatchStatus(context.TODO(), models.ScreeningMatchUpdate{
		MatchId:    "matchid",
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &userId,
	})
	assert.NoError(t, err)
	assert.NoError(t, exec.Mock.ExpectationsWereMet())
}
