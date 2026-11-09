package usecases

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type tracfinDraftRepository interface {
	GetTracfinConfiguration(context.Context, repositories.Executor, string) (models.TracfinConfiguration, error)
	GetTracfinDraft(context.Context, repositories.Executor, string, string) (models.TracfinDraft, error)
	CreateTracfinDraft(context.Context, repositories.Executor, string, string, models.TracfinDraft) error
	SaveTracfinDraft(context.Context, repositories.Executor, string, string, models.TracfinDraft, int64) error
}
type tracfinDraftTransactions interface {
	Transaction(context.Context, func(repositories.Transaction) error) error
}
type TracfinDraftDependencies struct {
	Repository   tracfinDraftRepository
	Transactions tracfinDraftTransactions
	Files        tracfinCaseFileRepository
	Blobs        tracfinBlobRepository
	Exports      tracfinExportRepository
	BucketURL    string
}

func (uc TracfinUsecase) draftActor(ctx context.Context) (models.Identity, error) {
	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok || creds.OrganizationId != uc.orgID || creds.ActorIdentity.UserId == "" {
		return models.Identity{}, models.ForbiddenError
	}
	if uc.draftDependencies == nil || uc.draftDependencies.Repository == nil || uc.draftDependencies.Transactions == nil {
		return models.Identity{}, errors.New("TRACFIN editing dependencies unavailable")
	}
	return creds.ActorIdentity, nil
}
func tracfinDraftDiagnostics(d models.TracfinDraft) models.TracfinDraft {
	if d.Configuration.Revision == 0 {
		d.Diagnostics = append(d.Diagnostics, models.TracfinDiagnostic{Code: "CONFIGURATION_UNAVAILABLE", Layer: "sources", Severity: "warning", Message: "No organization configuration; this draft can be completed manually"})
	}
	return d
}
func compatibleDraftProfile(config models.TracfinConfiguration, catalog tracfin.Catalog) bool {
	return catalog.CompatibleProfile(config.ProfileVersion, config.ProfileHash)
}
func (uc TracfinUsecase) CreateDraft(ctx context.Context, caseID string, declarantIsActor bool) (models.TracfinDraft, error) {
	actor, err := uc.draftActor(ctx)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	if c.Outcome == "" || c.Outcome == models.CaseOutcomeUnset || c.Outcome == models.CaseUnknownOutcome {
		return models.TracfinDraft{}, errors.Wrap(models.BadParameterError, "case outcome required")
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinDraft{}, err
	}
	var result models.TracfinDraft
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		config, err := uc.draftDependencies.Repository.GetTracfinConfiguration(ctx, tx, uc.orgID.String())
		if errors.Is(err, models.NotFoundError) {
			config = models.TracfinConfiguration{OrgID: uc.orgID.String(), ProfileVersion: catalog.Version, ProfileHash: catalog.Hash}
			err = nil
		}
		if err != nil {
			return err
		}
		if config.OrgID != "" && config.OrgID != uc.orgID.String() {
			return models.NotFoundError
		}
		if !compatibleDraftProfile(config, catalog) {
			return errors.Wrap(models.BadParameterError, "incompatible frozen TRACFIN profile")
		}
		// Clone configuration separately from the document so editable values cannot alter it.
		raw, err := json.Marshal(config)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &config); err != nil {
			return err
		}
		d := models.TracfinDraft{CaseID: caseID, OrgID: uc.orgID.String(), Configuration: config, Revision: 1, UpdatedBy: string(actor.UserId), Document: models.TracfinDocument{Root: models.TracfinBlock{ID: pure_utils.NewId().String(), CatalogID: "/ds"}}}
		for _, b := range config.Settings.Root.Children {
			if b.CatalogID == "/ds/organisme" || b.CatalogID == "/ds/contact" {
				d.Document.Root.Children = append(d.Document.Root.Children, cloneTracfinSettings(b))
			}
		}
		if declarantIsActor {
			coord := models.TracfinBlock{ID: pure_utils.NewId().String(), CatalogID: "/ds/declarant/coordonnees", Fields: map[string][]models.TracfinValue{}}
			for field, value := range map[string]string{"prenom": actor.FirstName, "nom": actor.LastName, "courriel": actor.Email} {
				if value != "" {
					s := value
					coord.Fields[coord.CatalogID+"/"+field] = []models.TracfinValue{{ID: pure_utils.NewId().String(), Kind: "text", Text: &s, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}}
				}
			}
			d.Document.Root.Children = append(d.Document.Root.Children, models.TracfinBlock{ID: pure_utils.NewId().String(), CatalogID: "/ds/declarant", Children: []models.TracfinBlock{coord}})
		}
		if err = uc.draftDependencies.Repository.CreateTracfinDraft(ctx, tx, uc.orgID.String(), caseID, d); err != nil {
			return err
		}
		result, err = uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, uc.orgID.String(), caseID)
		return err
	})
	return tracfinDraftDiagnostics(result), err
}
func cloneTracfinSettings(b models.TracfinBlock) models.TracfinBlock {
	raw, _ := json.Marshal(b)
	_ = json.Unmarshal(raw, &b)
	ids := map[string]string{}
	var allocate func(*models.TracfinBlock)
	allocate = func(b *models.TracfinBlock) {
		ids[b.ID] = pure_utils.NewId().String()
		b.ID = ids[b.ID]
		b.Selection = nil
		b.Attachment = nil
		for field, values := range b.Fields {
			for i := range values {
				ids[values[i].ID] = pure_utils.NewId().String()
				values[i].ID = ids[values[i].ID]
				values[i].Origin = &models.TracfinOrigin{Source: "settings", Resolution: "resolved"}
			}
			b.Fields[field] = values
		}
		for i := range b.Children {
			allocate(&b.Children[i])
		}
	}
	allocate(&b)
	var remap func(*models.TracfinBlock)
	remap = func(b *models.TracfinBlock) {
		for field, values := range b.Fields {
			for i := range values {
				if values[i].ParentValueID != "" {
					values[i].ParentValueID = ids[values[i].ParentValueID]
				}
				if r := values[i].Reference; r != nil {
					r.BlockID = ids[r.BlockID]
					r.PersonBlockID = ids[r.PersonBlockID]
				}
			}
			b.Fields[field] = values
		}
		for i := range b.Children {
			remap(&b.Children[i])
		}
	}
	remap(&b)
	return b
}
func (uc TracfinUsecase) UpdateDraft(ctx context.Context, caseID string, input models.TracfinDraftInput) (models.TracfinDraft, error) {
	actor, err := uc.draftActor(ctx)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	// One authorization and source loader serve every change of the batch.
	sources, err := uc.sourceLoader(ctx, caseID)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	if input.ExpectedRevision < 1 {
		return models.TracfinDraft{}, models.BadParameterError
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinDraft{}, err
	}
	var result models.TracfinDraft
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, err := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, uc.orgID.String(), caseID)
		if err != nil {
			return err
		}
		if d.OrgID != uc.orgID.String() || d.CaseID != caseID {
			return models.NotFoundError
		}
		if d.Revision != input.ExpectedRevision {
			return models.ConflictError
		}
		if !compatibleDraftProfile(d.Configuration, catalog) {
			return errors.Wrap(models.BadParameterError, "incompatible frozen TRACFIN profile")
		}
		aliases := map[string]string{}
		for _, raw := range input.Changes {
			c := raw
			remap := func(id string) string {
				if mapped, ok := aliases[id]; ok {
					return mapped
				}
				return id
			}
			if c.Action == "add_block" {
				if c.BlockID != "" {
					if _, exists := aliases[c.BlockID]; exists {
						return models.BadParameterError
					}
					exists := false
					tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
						if b.ID == c.BlockID {
							exists = true
						}
					})
					if exists {
						return models.BadParameterError
					}
				}
				id := pure_utils.NewId().String()
				if c.BlockID != "" {
					aliases[c.BlockID] = id
				}
				c.BlockID = id
			} else {
				c.BlockID = remap(c.BlockID)
			}
			c.ParentBlockID = remap(c.ParentBlockID)
			c.Field.BlockID = remap(c.Field.BlockID)
			if c.Value != nil {
				v := *c.Value
				c.Value = &v
				if v.Reference != nil {
					r := *v.Reference
					r.BlockID = remap(r.BlockID)
					r.PersonBlockID = remap(r.PersonBlockID)
					v.Reference = &r
				}
			}
			if c.Relation != nil {
				r := *c.Relation
				r.BlockID = remap(r.BlockID)
				r.PersonBlockID = remap(r.PersonBlockID)
				c.Relation = &r
			}
			if c.Action == "refresh_configuration" {
				config, err := uc.draftDependencies.Repository.GetTracfinConfiguration(ctx, tx, uc.orgID.String())
				if err != nil {
					return err
				}
				if config.OrgID != "" && config.OrgID != uc.orgID.String() {
					return models.NotFoundError
				}
				if !compatibleDraftProfile(config, catalog) {
					return errors.Wrap(models.BadParameterError, "incompatible TRACFIN profile")
				}
				copied, err := tracfin.ApplyChanges(d, nil, catalog)
				if err != nil {
					return err
				}
				d = copied
				d.Configuration = config
				reviewTracfinDraft(&d.Document.Root)
				continue
			}
			if c.Action == "apply_prefill" {
				proposal, err := sources.prefillDraft(ctx, d)
				if err != nil {
					return err
				}
				if c.MappingVersion == "" || c.MappingVersion != proposal.MappingVersion {
					return models.ConflictError
				}
				var field models.TracfinFieldRef
				var selected *models.TracfinValue
				for _, p := range proposal.Proposals {
					if p.ID == c.ProposalID {
						field = p.Field
						for _, candidate := range p.Candidates {
							if candidate.ID == c.CandidateID {
								v := candidate
								selected = &v
							}
						}
					}
				}
				if selected == nil {
					return errors.Wrap(models.BadParameterError, "prefill candidate is stale or unknown")
				}
				input := &models.TracfinFieldInput{Kind: selected.Kind, Text: selected.Text, Boolean: selected.Boolean, Decimal: selected.Decimal, Reference: selected.Reference, ParentValueID: selected.ParentValueID}
				next, err := tracfin.ApplyChanges(d, []tracfin.Change{{Action: "set_field", Field: field, Value: input}}, catalog)
				if err != nil {
					return err
				}
				attachTracfinOrigin(&next.Document.Root, field, selected.Origin, selected.NeedsReview)
				d = next
				continue
			}
			var attachment *models.TracfinAttachment
			var attachmentFile tracfinAttachmentFile
			if c.SourceFileID != "" {
				if c.Action != "add_block" || c.CatalogID != "/ds/pieceJointe/piece" {
					return models.BadParameterError
				}
				attachment, attachmentFile, err = uc.selectTracfinAttachmentFile(ctx, tx, caseID, c.SourceFileID)
				if err != nil {
					return err
				}
				c.SourceFileID = ""
			}
			next, err := tracfin.ApplyChanges(d, []tracfin.Change{c}, catalog)
			if err != nil {
				return err
			}
			if attachment != nil {
				setTracfinAttachment(&next.Document.Root, c.BlockID, attachment)
				// Name and MIME type come from the file itself; a type outside the
				// allowed list is left empty for the analyst.
				source := models.TracfinAISource{ID: "file:" + attachment.SourceFileID}
				source.Content, _ = json.Marshal(map[string]string{"name": attachmentFile.Name, "mime": attachmentFile.Mime})
				next, _ = aiSetAttachmentFileFields(next, catalog, c.BlockID, source, "")
			}
			if c.Action == "set_selection" {
				var selected *models.TracfinSelection
				tracfin.WalkBlocks(next.Document.Root, func(b models.TracfinBlock) {
					if b.ID == c.BlockID {
						selected = b.Selection
					}
				})
				if selected == nil {
					return models.BadParameterError
				}
				if _, err = sources.load(ctx, next.Configuration, c.BlockID, *selected, false); err != nil {
					return err
				}
			}
			d = next
		}
		d.UpdatedBy = string(actor.UserId)
		d.Diagnostics = nil

		if d.AI != nil {
			d.AI.ReviewedRevision = 0
			values := d.Document.Root.Fields["/ds/analyse"]
			if len(values) != 1 || values[0].Text == nil {
				return errors.Wrap(models.BadParameterError, "AI draft requires mandatory human analysis")
			}
			text := *values[0].Text
			if e := validateAIAnalysis(text); e != nil {
				return e
			}
			old := ""
			for _, a := range d.AI.Analyses {
				if a.ID == d.AI.CurrentAnalysisID {
					old = a.Text
				}
			}
			if text != old {
				putAIAnalysis(&d, text, string(actor.UserId), "")
			}
		}
		if uc.ai != nil && d.AI != nil {
			if err = uc.ai.Repository.InvalidateTracfinAIActive(ctx, tx, uc.orgID.String(), caseID); err != nil {
				return err
			}
		}
		if err = uc.draftDependencies.Repository.SaveTracfinDraft(ctx, tx, uc.orgID.String(), caseID, d, input.ExpectedRevision); err != nil {
			return err
		}
		result, err = uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, uc.orgID.String(), caseID)
		return err
	})
	return tracfinDraftDiagnostics(result), err
}

func reviewTracfinDraft(b *models.TracfinBlock) {
	for field, values := range b.Fields {
		for i := range values {
			values[i].NeedsReview = true
			if values[i].Origin != nil && values[i].Origin.Source != "manual" {
				values[i].Origin.Resolution = "needs_review"
			}
		}
		b.Fields[field] = values
	}
	for i := range b.Children {
		reviewTracfinDraft(&b.Children[i])
	}
}
func attachTracfinOrigin(b *models.TracfinBlock, field models.TracfinFieldRef, origin *models.TracfinOrigin, needsReview bool) {
	if b.ID == field.BlockID {
		values := b.Fields[field.FieldID]
		for i := range values {
			if field.ValueID == "" && i == len(values)-1 || values[i].ID == field.ValueID {
				copy := *origin
				copy.Resolution = "resolved"
				values[i].Origin = &copy
				values[i].NeedsReview = needsReview
			}
		}
		b.Fields[field.FieldID] = values
	}
	for i := range b.Children {
		attachTracfinOrigin(&b.Children[i], field, origin, needsReview)
	}
}
