package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type configOrgFake struct{ calls int }

func (f *configOrgFake) GetOrganizationById(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.Organization, error) {
	f.calls++
	return models.Organization{Id: id}, nil
}

type configSecurityFake struct{ err error }

func (f configSecurityFake) EditOrganization(models.Organization) error { return f.err }

type configRepoFake struct{ calls int }

func (f *configRepoFake) GetTracfinConfiguration(context.Context, repositories.Executor, string) (models.TracfinConfiguration, error) {
	f.calls++
	return models.TracfinConfiguration{}, models.NotFoundError
}
func (f *configRepoFake) SaveTracfinConfiguration(context.Context, repositories.Executor, string, models.TracfinConfiguration, int64) error {
	f.calls++
	return nil
}
func TestTracfinConfigurationPermissionAndScope(t *testing.T) {
	org := uuid.New()
	uc := draftTestUsecase(org, &draftTestRepo{}, models.CaseConfirmedRisk)
	repo := &configRepoFake{}
	orgs := &configOrgFake{}
	uc.configurationDependencies = &TracfinConfigurationDependencies{Repository: repo, Organizations: orgs, Security: configSecurityFake{err: models.ForbiddenError}}
	_, err := uc.GetConfiguration(draftTestContext(org), org)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Zero(t, orgs.calls)
	require.Zero(t, repo.calls)
	_, err = uc.SaveConfiguration(draftTestContext(org), org, models.TracfinConfigurationInput{})
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Zero(t, repo.calls)
	_, err = uc.GetConfiguration(draftTestContext(org), uuid.New())
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Zero(t, orgs.calls)
	require.Zero(t, repo.calls)
}
func TestTracfinConfigurationSettingsWhitelist(t *testing.T) {
	catalog, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	text := "Example"
	setting := func(id string) models.TracfinSettingInput {
		return models.TracfinSettingInput{Field: models.TracfinFieldRef{FieldID: id}, Value: models.TracfinFieldInput{Kind: "text", Text: &text}}
	}
	_, err = tracfinSettings([]models.TracfinSettingInput{setting("/ds/organisme/libelle")}, catalog)
	require.NoError(t, err)
	for _, id := range []string{"/ds/declarant/coordonnees/nom", "/ds/pieceJointe/piece", "/ds/personne"} {
		t.Run(id, func(t *testing.T) {
			_, err := tracfinSettings([]models.TracfinSettingInput{setting(id)}, catalog)
			require.ErrorIs(t, err, models.BadParameterError)
		})
	}
	ref := setting("/ds/organisme/libelle")
	ref.Value = models.TracfinFieldInput{Kind: "reference", Reference: &models.TracfinReference{BlockID: "x"}}
	_, err = tracfinSettings([]models.TracfinSettingInput{ref}, catalog)
	require.ErrorIs(t, err, models.BadParameterError)
	_, err = tracfinSettings([]models.TracfinSettingInput{setting("/ds/organisme/libelle"), setting("/ds/organisme/libelle")}, catalog)
	require.ErrorIs(t, err, models.BadParameterError)
}
