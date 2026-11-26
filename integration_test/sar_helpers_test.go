package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
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
