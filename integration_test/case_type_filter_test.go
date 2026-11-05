package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func TestListCasesFiltersByTypeAcrossPages(t *testing.T) {
	bg := context.Background()
	tenantId, orgId, inboxId := uuid.New(), uuid.New(), uuid.New()

	_, err := pgPool.Exec(bg, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantId, tenantId.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(bg, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
		orgId, orgId.String(), tenantId)
	require.NoError(t, err)
	_, err = pgPool.Exec(bg, "INSERT INTO inboxes (id, name, organization_id) VALUES ($1, 'inbox', $2)", inboxId, orgId)
	require.NoError(t, err)

	// Interleave both types, with a boosted case of each so that pages cross the boost boundary of the sort.
	start := time.Now().Add(-time.Hour)
	for i := range 15 {
		caseType := "decision"
		if i%3 == 0 {
			caseType = "continuous_screening"
		}
		var boost *string
		if i == 6 || i == 7 {
			boost = utils.Ptr(string(models.BoostEscalated))
		}
		_, err = pgPool.Exec(bg, `
			INSERT INTO cases (org_id, inbox_id, name, type, boost, created_at)
			VALUES ($1, $2, 'case', $3, $4, $5)`,
			orgId, inboxId, caseType, boost, start.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	creds := models.Credentials{
		ActorIdentity:  models.Identity{UserId: "case-type-test-user"},
		OrganizationId: orgId,
		Roles:          []models.Role{models.ADMIN},
	}
	ctx := context.WithValue(bg, utils.ContextKeyCredentials, creds)
	uc := generateUsecaseWithCreds(testUsecases, creds)
	usecase := uc.NewCaseUseCase()

	listAll := func(t *testing.T, limit int, types ...models.CaseType) []models.Case {
		t.Helper()
		var all []models.Case
		offsetId := ""
		for {
			page, err := usecase.ListCases(ctx, orgId, models.PaginationAndSorting{
				OffsetId: offsetId,
				Limit:    limit,
				Order:    models.SortingOrderDesc,
				Sorting:  models.CasesSortingCreatedAt,
			}, models.CaseFilters{Types: types})
			require.NoError(t, err)
			require.LessOrEqual(t, len(page.Cases), limit)
			all = append(all, page.Cases...)
			if !page.HasNextPage {
				return all
			}
			require.Len(t, page.Cases, limit, "a page with a next page must be full")
			offsetId = page.Cases[len(page.Cases)-1].Id
		}
	}
	idsOfType := func(cases []models.Case, caseType models.CaseType) []string {
		var ids []string
		for _, c := range cases {
			if c.Type == caseType {
				ids = append(ids, c.Id)
			}
		}
		return ids
	}

	unfiltered := listAll(t, 100)
	require.Len(t, unfiltered, 15)

	for _, caseType := range []models.CaseType{models.CaseTypeContinuousScreening, models.CaseTypeDecision} {
		t.Run(caseType.String(), func(t *testing.T) {
			expected := idsOfType(unfiltered, caseType)
			for _, limit := range []int{1, 2, 100} {
				cases := listAll(t, limit, caseType)
				assert.Equal(t, expected, idsOfType(cases, caseType), "limit %d", limit)
				assert.Len(t, cases, len(expected), "limit %d: only cases of this type", limit)
			}
		})
	}

	t.Run("both types", func(t *testing.T) {
		both := listAll(t, 4, models.CaseTypeDecision, models.CaseTypeContinuousScreening)
		assert.Equal(t, unfiltered, both)
	})
}
