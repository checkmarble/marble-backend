package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type tracfinConfigurationRepository interface {
	GetTracfinConfiguration(context.Context, repositories.Executor, string) (models.TracfinConfiguration, error)
	SaveTracfinConfiguration(context.Context, repositories.Executor, string, models.TracfinConfiguration, int64) error
}
type tracfinOrganizationRepository interface {
	GetOrganizationById(context.Context, repositories.Executor, uuid.UUID) (models.Organization, error)
}
type tracfinOrganizationSecurity interface {
	EditOrganization(models.Organization) error
}
type TracfinConfigurationDependencies struct {
	Repository    tracfinConfigurationRepository
	Organizations tracfinOrganizationRepository
	Security      tracfinOrganizationSecurity
}

func (uc TracfinUsecase) configurationAccess(ctx context.Context, orgID uuid.UUID) error {
	if orgID != uc.orgID {
		return models.ForbiddenError
	}
	deps := uc.configurationDependencies
	if deps == nil || deps.Repository == nil || deps.Organizations == nil || deps.Security == nil {
		return errors.New("TRACFIN configuration dependencies unavailable")
	}
	// Check the existing permission before revealing whether the organization exists.
	if err := deps.Security.EditOrganization(models.Organization{Id: orgID}); err != nil {
		return err
	}
	org, err := deps.Organizations.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), orgID)
	if err != nil {
		return err
	}
	if org.Id != orgID {
		return models.NotFoundError
	}
	return deps.Security.EditOrganization(org)
}
func (uc TracfinUsecase) GetConfiguration(ctx context.Context, orgID uuid.UUID) (models.TracfinConfiguration, error) {
	if err := uc.configurationAccess(ctx, orgID); err != nil {
		return models.TracfinConfiguration{}, err
	}
	config, err := uc.configurationDependencies.Repository.GetTracfinConfiguration(ctx, uc.executorFactory.NewExecutor(), orgID.String())
	if err == nil && config.OrgID != orgID.String() {
		return models.TracfinConfiguration{}, models.NotFoundError
	}
	return config, err
}
func (uc TracfinUsecase) SaveConfiguration(ctx context.Context, orgID uuid.UUID, input models.TracfinConfigurationInput) (models.TracfinConfiguration, error) {
	if err := uc.configurationAccess(ctx, orgID); err != nil {
		return models.TracfinConfiguration{}, err
	}
	actor, err := uc.draftActor(ctx)
	if err != nil {
		return models.TracfinConfiguration{}, err
	}
	if input.ExpectedRevision < 0 {
		return models.TracfinConfiguration{}, models.BadParameterError
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinConfiguration{}, err
	}
	settings, err := tracfinSettings(input.Settings, catalog)
	if err != nil {
		return models.TracfinConfiguration{}, err
	}
	config := models.TracfinConfiguration{OrgID: orgID.String(), Settings: settings, Mappings: input.Mappings, ProfileVersion: catalog.Version, ProfileHash: catalog.Hash, UpdatedBy: string(actor.UserId)}
	var result models.TracfinConfiguration
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.configurationDependencies.Repository.SaveTracfinConfiguration(ctx, tx, orgID.String(), config, input.ExpectedRevision); err != nil {
			return err
		}
		var err error
		result, err = uc.configurationDependencies.Repository.GetTracfinConfiguration(ctx, tx, orgID.String())
		if err == nil && result.OrgID != orgID.String() {
			return models.NotFoundError
		}
		return err
	})
	return result, err
}

// Settings are singleton organization/contact fields. Field IDs address their
// fixed catalog owners; server IDs and origins are allocated on every replacement.
func tracfinSettings(settings []models.TracfinSettingInput, catalog tracfin.Catalog) (models.TracfinDocument, error) {
	draft := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "/ds", CatalogID: "/ds"}}}
	blocks := map[string]bool{"/ds": true}
	var ensureBlock func(string) error
	ensureBlock = func(id string) error {
		if blocks[id] {
			return nil
		}
		f, ok := catalog.Fields[id]
		if !ok || f.Kind != "block" {
			return models.BadParameterError
		}
		if err := ensureBlock(f.Parent); err != nil {
			return err
		}
		var err error
		draft, err = tracfin.ApplyChanges(draft, []tracfin.Change{{Action: "add_block", ParentBlockID: f.Parent, BlockID: id, CatalogID: id}}, catalog)
		if err == nil {
			blocks[id] = true
		}
		return err
	}
	seen := map[string]bool{}
	valueIDs := map[string]string{}
	// Element values precede scalar attributes so their owners already exist.
	for _, attributes := range []bool{false, true} {
		for _, s := range settings {
			f, ok := catalog.Fields[s.Field.FieldID]
			if !ok || f.Kind != "field" || f.Coverage != "editable" || !(strings.HasPrefix(f.ID, "/ds/organisme/") || strings.HasPrefix(f.ID, "/ds/contact/")) || s.Value.Reference != nil || s.Value.Kind == "reference" {
				return models.TracfinDocument{}, models.BadParameterError
			}
			if f.Attribute != attributes {
				continue
			}
			if seen[f.ID] {
				return models.TracfinDocument{}, models.BadParameterError
			}
			seen[f.ID] = true
			parent := f.Parent
			for catalog.Fields[parent].Kind != "block" {
				next := catalog.Fields[parent].Parent
				if next == "" {
					return models.TracfinDocument{}, models.BadParameterError
				}
				parent = next
			}
			if err := ensureBlock(parent); err != nil {
				return models.TracfinDocument{}, err
			}
			value := s.Value
			value.ParentValueID = ""
			if catalog.Fields[f.Parent].Kind == "field" {
				value.ParentValueID = valueIDs[f.Parent]
			}
			field := models.TracfinFieldRef{BlockID: parent, FieldID: f.ID, ParentValueID: value.ParentValueID}
			var err error
			draft, err = tracfin.ApplyChanges(draft, []tracfin.Change{{Action: "set_field", Field: field, Value: &value}}, catalog)
			if err != nil {
				return models.TracfinDocument{}, err
			}
			tracfin.WalkBlocks(draft.Document.Root, func(b models.TracfinBlock) {
				if b.ID == parent && len(b.Fields[f.ID]) > 0 {
					valueIDs[f.ID] = b.Fields[f.ID][0].ID
				}
			})
		}
	}
	// Remove predictable construction aliases before persistence and assign settings origins.
	doc := models.TracfinDocument{Root: cloneTracfinSettings(draft.Document.Root)}
	return doc, nil
}
func (uc TracfinUsecase) GetDraft(ctx context.Context, caseID string) (models.TracfinDraft, error) {
	d, err := uc.draft(ctx, caseID)
	return tracfinDraftDiagnostics(d), err
}
func (uc TracfinUsecase) PrefillAtRevision(ctx context.Context, caseID string, revision int64) (tracfin.PrefillResult, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return tracfin.PrefillResult{}, err
	}
	if revision < 1 {
		return tracfin.PrefillResult{}, models.BadParameterError
	}
	if d.Revision != revision {
		return tracfin.PrefillResult{}, models.ConflictError
	}
	return uc.prefillDraft(ctx, caseID, d)
}
func (uc TracfinUsecase) ValidateDraft(ctx context.Context, caseID string, revision int64) (models.TracfinValidation, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return models.TracfinValidation{}, err
	}
	if revision < 1 {
		return models.TracfinValidation{}, models.BadParameterError
	}
	if d.Revision != revision {
		return models.TracfinValidation{}, models.ConflictError
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinValidation{}, err
	}
	return tracfin.ValidateDraft(d, catalog), nil
}
