package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/usecases/continuous_screening"
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
		repository:                repositories.NewMarbleDbRepository(false, 0.3),
		screeningConfigRepository: repositories.NewMarbleDbRepository(false, 0.3),
		openSanctionsProvider:     nil, // Will be overridden in tests
		executorFactory:           exec,
		transactionFactory:        txFac,
	}

	return uc, exec
}

func TestListScreeningOnDecision(t *testing.T) {
	uc, exec := buildScreeningUsecaseMock()
	sccId := uuid.NewString()

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
			sc.id, sc.decision_id, sc.org_id, sc.screening_config_id, sc.status, sc.search_input, sc.initial_query, sc.search_datasets, sc.match_threshold, sc.match_limit, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.initial_has_matches, sc.whitelisted_entities, sc.error_codes, sc.number_of_matches, sc.created_at, sc.updated_at,
			scc.id AS config_id, stable_id, scc.name,
			ARRAY_AGG(ROW(scm.id,scm.screening_id,scm.opensanction_entity_id,scm.status,scm.query_ids,scm.counterparty_id,scm.payload,scm.enriched,scm.reviewed_by,scm.created_at,scm.updated_at)
				ORDER BY array_position(.+, scm.status), scm.payload->>'score' DESC) FILTER (WHERE scm.id IS NOT NULL)
				AS matches
		FROM screenings AS sc
		INNER JOIN screening_configs AS scc ON sc.screening_config_id=scc.id
		LEFT JOIN screening_matches AS scm ON (sc.id = scm.screening_id)
		WHERE sc.decision_id = $1
			AND sc.is_archived = $2
		GROUP BY sc.id, sc.decision_id, sc.org_id, sc.screening_config_id, sc.status, sc.search_input, sc.initial_query, sc.search_datasets, sc.match_threshold, sc.match_limit, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.initial_has_matches, sc.whitelisted_entities, sc.error_codes, sc.number_of_matches, sc.created_at, sc.updated_at, config_id, stable_id, name
		ORDER BY sc.created_at
	`)).
		WithArgs(utils.TextToUUID("decisionid").String(), false).
		WillReturnRows(
			pgxmock.NewRows(dbmodels.SelectScreeningWithMatchesColumn).
				AddRow(mockScRow...),
		)

	exec.Mock.ExpectQuery(`SELECT .* FROM screening_match_comments WHERE screening_match_id = ANY\(\$1\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "screening_match_id", "commented_by", "comment", "created_at"}).
				AddRows(mockCommentsRows...),
		)

	scs, err := uc.ListScreenings(context.TODO(), utils.TextToUUID("decisionid").String(), false)

	assert.NoError(t, exec.Mock.ExpectationsWereMet())
	assert.NoError(t, err)
	assert.Len(t, scs, 1)
	assert.Equal(t, models.ScreeningStatusFrom(mockSc.Status), scs[0].Status)
	assert.NotEmpty(t, scs[0].Matches)
	assert.Equal(t, models.ScreeningMatchStatusFrom(scs[0].Matches[0].Status.String()), models.ScreeningMatchStatusUnknown)
	assert.Len(t, scs[0].Matches[0].Comments, 4)
	assert.Equal(t, scs[0].Matches[0].Comments[0].Comment, mockComments[0].Comment)
}

func TestUpdateMatchStatus(t *testing.T) {
	uc, exec := buildScreeningUsecaseMock()
	userId := models.UserId(uuid.NewString())

	_, mockScmRow := utils.FakeStruct[dbmodels.DBScreeningMatch](ops.WithCustomFieldProvider(
		"ScreeningId", func() (interface{}, error) {
			return "screening_id", nil
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
			return "screening_id", nil
		}),
		ops.WithCustomFieldProvider(
			"Id", func() (interface{}, error) {
				i += 1
				return fmt.Sprintf("otherMatchId_%d", i), nil
			}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "pending", nil
		}))

	_, mockScRow := utils.FakeStruct[dbmodels.DBScreeningAndConfig](ops.WithCustomFieldProvider(
		"Id", func() (interface{}, error) {
			return "screening_id", nil
		}),
		ops.WithCustomFieldProvider("IsArchived", func() (interface{}, error) {
			return false, nil
		}),
		ops.WithCustomFieldProvider("Status", func() (interface{}, error) {
			return "in_review", nil
		}))

	exec.Mock.
		ExpectQuery(`SELECT id, screening_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, enriched, reviewed_by, created_at, updated_at FROM screening_matches WHERE id = \$1`).
		WithArgs("matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...),
		)
	exec.Mock.
		ExpectQuery(`SELECT sc.id, sc.decision_id, sc.org_id, sc.screening_config_id, sc.status, sc.search_input, sc.initial_query, sc.search_datasets, sc.match_threshold, sc.match_limit, sc.is_manual, sc.requested_by, sc.is_partial, sc.is_archived, sc.initial_has_matches, sc.whitelisted_entities, sc.error_codes, sc.number_of_matches, sc.created_at, sc.updated_at, scc.id AS config_id, stable_id, name FROM screenings AS sc INNER JOIN screening_configs AS scc ON sc.screening_config_id=scc.id WHERE sc.id = \$1`).
		WithArgs("screening_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningAndConfigColumn).
			AddRow(mockScRow...),
		)
	exec.Mock.ExpectQuery(`SELECT id, screening_id, opensanction_entity_id, status, query_ids, counterparty_id, payload, enriched, reviewed_by, created_at, updated_at FROM screening_matches WHERE screening_id = \$1`).
		WithArgs("screening_id").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...).
			AddRows(mockOtherScmRows...),
		)
	exec.Mock.ExpectQuery(`UPDATE screening_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,screening_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,enriched,reviewed_by,created_at,updated_at`).
		WithArgs(&userId, models.ScreeningMatchStatusConfirmedHit, "NOW()", "matchid").
		WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
			AddRow(mockScmRow...),
		)

	for i := range 3 {
		exec.Mock.ExpectQuery(`UPDATE screening_matches SET reviewed_by = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 RETURNING id,screening_id,opensanction_entity_id,status,query_ids,counterparty_id,payload,enriched,reviewed_by,created_at,updated_at`).
			WithArgs(&userId, models.ScreeningMatchStatusSkipped, "NOW()", mockOtherScms[i].Id).
			WillReturnRows(pgxmock.NewRows(dbmodels.SelectScreeningMatchesColumn).
				AddRow(mockOtherScmRows[i]...),
			)
	}

	exec.Mock.ExpectExec(`UPDATE screenings SET status = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(models.ScreeningStatusConfirmedHit.String(), "NOW()", "screening_id").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	_, err := uc.UpdateMatchStatus(context.TODO(), models.ScreeningMatchUpdate{
		MatchId:    "matchid",
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &userId,
	})
	assert.NoError(t, err)
	assert.NoError(t, exec.Mock.ExpectationsWereMet())
}

func TestGetDatasetCatalog(t *testing.T) {
	// Create mock ScreeningProvider
	providerMock := &mocks.OpenSanctionsRepository{}

	// Create test catalog with mixed datasets
	testCatalog := models.OpenSanctionsCatalog{
		Sections: []models.OpenSanctionsCatalogSection{
			{
				Name:  "test-section",
				Title: "Test Section",
				Datasets: []models.OpenSanctionsCatalogDataset{
					{
						Name:  "regular-dataset-1",
						Title: "Regular Dataset 1",
						Tags:  []string{"public"},
					},
					{
						Name:  "marble-dataset",
						Title: "Marble Dataset",
						Tags:  []string{continuous_screening.MarbleContinuousScreeningTag},
					},
					{
						Name:  "regular-dataset-2",
						Title: "Regular Dataset 2",
						Tags:  []string{"public", "another-tag"},
					},
					{
						Name:  "another-marble-dataset",
						Title: "Another Marble Dataset",
						Tags:  []string{continuous_screening.MarbleContinuousScreeningTag, "extra-tag"},
					},
				},
			},
		},
	}

	// Setup mock expectations
	providerMock.On("GetCatalog", mock.Anything).Return(testCatalog, nil)

	// Create usecase with mock provider
	uc, _ := buildScreeningUsecaseMock()
	uc.openSanctionsProvider = providerMock

	// Call the method under test
	result, err := uc.GetDatasetCatalog(context.TODO())

	// Assert no error
	assert.NoError(t, err)

	// Assert that only one section exists
	assert.Len(t, result.Sections, 1)

	// Assert that only regular datasets remain (2 out of 4)
	assert.Len(t, result.Sections[0].Datasets, 2)

	// Assert that the remaining datasets are the regular ones
	expectedNames := []string{"regular-dataset-1", "regular-dataset-2"}
	actualNames := make([]string, len(result.Sections[0].Datasets))
	for i, dataset := range result.Sections[0].Datasets {
		actualNames[i] = dataset.Name
	}

	assert.ElementsMatch(t, expectedNames, actualNames)

	// Verify mock expectations
	providerMock.AssertExpectations(t)
}
