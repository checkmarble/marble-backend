package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// sarDeclarationTestFixture creates a tenant, an organization, an inbox and a case
// and returns the organization and case identifiers. Everything is removed on cleanup.
func sarDeclarationTestFixture(t *testing.T) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	tenantID := uuid.New()
	organizationID := uuid.New()
	inboxID := uuid.New()
	caseID := uuid.New()

	_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
		organizationID, organizationID.String(), tenantID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO inboxes (id, name, organization_id) VALUES ($1, $2, $3)",
		inboxID, "sar-declaration-inbox-"+inboxID.String(), organizationID)
	require.NoError(t, err)

	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	require.NoError(t, repo.CreateCase(ctx, exec, models.CreateCaseAttributes{
		InboxId:        inboxID,
		Name:           "sar declaration case",
		OrganizationId: organizationID,
		Type:           models.CaseTypeDecision,
	}, caseID.String()))

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pgPool.Exec(bg, "DELETE FROM sar_declaration_exports WHERE declaration_id IN (SELECT id FROM sar_declarations WHERE organization_id = $1)", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM sar_declarations WHERE organization_id = $1", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM sar_org_format_settings WHERE organization_id = $1", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM cases WHERE org_id = $1", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM inboxes WHERE organization_id = $1", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM organizations WHERE id = $1", organizationID)
		_, _ = pgPool.Exec(bg, "DELETE FROM tenants WHERE id = $1", tenantID)
	})

	return organizationID, caseID
}

func newSarTestDeclaration(organizationID, caseID uuid.UUID) models.SarDeclaration {
	return models.SarDeclaration{
		Id:             uuid.New(),
		OrganizationId: organizationID,
		CaseId:         caseID,
		FormatKey:      "tracfin:FR:1.4.0",
		ReportType:     "soupcon",
		Document:       []byte(`{}`),
		Version:        1,
	}
}

func TestSarDeclaration_CreateGetUpdate(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	got, err := repo.GetSarDeclaration(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Equal(t, declaration.Id, got.Id)
	require.Equal(t, organizationID, got.OrganizationId)
	require.Equal(t, caseID, got.CaseId)
	require.Equal(t, "tracfin:FR:1.4.0", got.FormatKey)
	require.Equal(t, "soupcon", got.ReportType)
	require.Equal(t, 1, got.Version)
	require.Equal(t, "", got.Analysis)
	require.Equal(t, "", got.ExtraInfo)
	require.JSONEq(t, `{}`, string(got.Document))

	list, err := repo.ListSarDeclarationsByCase(ctx, exec, caseID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, declaration.Id, list[0].Id)

	newVersion, err := repo.UpdateSarDeclarationDocument(ctx, exec, declaration.Id, 1,
		[]byte(`{"report":{"type":"soupcon"}}`), "analyse rédigée", "info complémentaire")
	require.NoError(t, err)
	require.Equal(t, 2, newVersion)

	updated, err := repo.GetSarDeclaration(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, "analyse rédigée", updated.Analysis)
	require.Equal(t, "info complémentaire", updated.ExtraInfo)
	require.JSONEq(t, `{"report":{"type":"soupcon"}}`, string(updated.Document))
}

func TestUpdateField_StaleVersionConflict(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	_, err := repo.UpdateSarDeclarationDocument(ctx, exec, declaration.Id, 1,
		[]byte(`{"first":true}`), "premier", "")
	require.NoError(t, err)

	_, err = repo.UpdateSarDeclarationDocument(ctx, exec, declaration.Id, 1,
		[]byte(`{"second":true}`), "second", "")
	require.Error(t, err)
	require.True(t, errors.Is(err, models.ErrSarVersionConflict))
	require.True(t, errors.Is(err, models.ConflictError))

	stored, err := repo.GetSarDeclaration(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Equal(t, 2, stored.Version)
	require.Equal(t, "premier", stored.Analysis)
	require.JSONEq(t, `{"first":true}`, string(stored.Document))
}

func TestSarOrgFormatSettings_Upsert(t *testing.T) {
	ctx := t.Context()
	organizationID, _ := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	const family = "tracfin"

	absent, err := repo.GetSarOrgFormatSettings(ctx, exec, organizationID, family)
	require.NoError(t, err)
	require.Nil(t, absent)

	require.NoError(t, repo.UpsertSarOrgFormatSettings(ctx, exec, models.SarOrgFormatSettings{
		OrganizationId: organizationID,
		FormatFamily:   family,
		Values: map[string]string{
			"organisme.nom":    "Banque Exemple",
			"declarant.nom":    "Jean Dupont",
			"contact.courriel": "conformite@example.test",
		},
	}))

	first, err := repo.GetSarOrgFormatSettings(ctx, exec, organizationID, family)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, organizationID, first.OrganizationId)
	require.Equal(t, family, first.FormatFamily)
	require.Equal(t, map[string]string{
		"organisme.nom":    "Banque Exemple",
		"declarant.nom":    "Jean Dupont",
		"contact.courriel": "conformite@example.test",
	}, first.Values)
	require.False(t, first.UpdatedAt.IsZero())

	require.NoError(t, repo.UpsertSarOrgFormatSettings(ctx, exec, models.SarOrgFormatSettings{
		OrganizationId: organizationID,
		FormatFamily:   family,
		Values: map[string]string{
			"organisme.nom": "Banque Exemple Holding",
		},
	}))

	second, err := repo.GetSarOrgFormatSettings(ctx, exec, organizationID, family)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, map[string]string{"organisme.nom": "Banque Exemple Holding"}, second.Values)
	require.False(t, second.UpdatedAt.Before(first.UpdatedAt))
}

func TestSarDeclarationExport_CreateGetList(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	export := models.SarDeclarationExport{
		Id:                 uuid.New(),
		DeclarationId:      declaration.Id,
		DeclarationVersion: 1,
		Document:           []byte(`{"report":{"type":"soupcon"}}`),
		Outstanding:        []byte(`[{"kind":"warning","path":"organisme.nom"}]`),
		BucketName:         "file:///tmp/tempFiles?create_dir=true",
		FileReference:      "sar/export-1.xml",
		Sha256:             "0123456789abcdef",
	}
	require.NoError(t, repo.CreateSarDeclarationExport(ctx, exec, export))

	got, err := repo.GetSarDeclarationExport(ctx, exec, export.Id)
	require.NoError(t, err)
	require.Equal(t, export.Id, got.Id)
	require.Equal(t, declaration.Id, got.DeclarationId)
	require.Equal(t, 1, got.DeclarationVersion)
	require.JSONEq(t, `{"report":{"type":"soupcon"}}`, string(got.Document))
	require.JSONEq(t, `[{"kind":"warning","path":"organisme.nom"}]`, string(got.Outstanding))
	require.Equal(t, export.BucketName, got.BucketName)
	require.Equal(t, export.FileReference, got.FileReference)
	require.Equal(t, export.Sha256, got.Sha256)

	second := export
	second.Id = uuid.New()
	second.DeclarationVersion = 2
	second.Outstanding = []byte(`[]`)
	second.FileReference = "sar/export-2.xml"
	require.NoError(t, repo.CreateSarDeclarationExport(ctx, exec, second))

	count, lastAt, err := repo.GetSarDeclarationExportsSummary(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NotNil(t, lastAt)
	require.False(t, lastAt.Before(got.CreatedAt))

	noExports := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, noExports))
	count, lastAt, err = repo.GetSarDeclarationExportsSummary(ctx, exec, noExports.Id)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	require.Nil(t, lastAt)

	list, err := repo.ListSarDeclarationExports(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Len(t, list, 2)
	ids := []uuid.UUID{list[0].Id, list[1].Id}
	require.ElementsMatch(t, []uuid.UUID{export.Id, second.Id}, ids)
}

func TestSarDeclaration_CascadeOnCaseDelete(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))
	export := models.SarDeclarationExport{
		Id:                 uuid.New(),
		DeclarationId:      declaration.Id,
		DeclarationVersion: 1,
		Document:           []byte(`{}`),
		Outstanding:        []byte(`[]`),
		BucketName:         "file:///tmp/tempFiles?create_dir=true",
		FileReference:      "sar/cascade.xml",
		Sha256:             "fedcba9876543210",
	}
	require.NoError(t, repo.CreateSarDeclarationExport(ctx, exec, export))

	_, err := pgPool.Exec(ctx, "DELETE FROM cases WHERE id = $1", caseID)
	require.NoError(t, err)

	var declarationCount, exportCount int
	require.NoError(t, pgPool.QueryRow(ctx,
		"SELECT count(*) FROM sar_declarations WHERE id = $1", declaration.Id).Scan(&declarationCount))
	require.NoError(t, pgPool.QueryRow(ctx,
		"SELECT count(*) FROM sar_declaration_exports WHERE id = $1", export.Id).Scan(&exportCount))
	require.Equal(t, 0, declarationCount)
	require.Equal(t, 0, exportCount)
}
