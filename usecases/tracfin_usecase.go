package usecases

import (
	"context"
	"fmt"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"slices"
)

var (
	errTracfinSourceUnavailable = errors.Wrap(models.NotFoundError, "TRACFIN source unavailable")
	errTracfinLinkUnavailable   = errors.Wrap(models.BadParameterError, "TRACFIN configured link unavailable")
	errTracfinIndexUnavailable  = errors.Wrap(models.UnprocessableEntityError, "NAVIGATION_INDEX_UNAVAILABLE")
)

type tracfinCaseReader interface {
	GetCase(context.Context, string) (models.Case, error)
	ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error)
}
type tracfinExecutorFactory interface {
	NewExecutor() repositories.Executor
	NewClientDbExecutor(context.Context, uuid.UUID) (repositories.Executor, error)
}
type tracfinSourceRepository interface {
	GetTracfinDraft(context.Context, repositories.Executor, string, string) (models.TracfinDraft, error)
	GetTracfinDecisionSources(context.Context, repositories.Executor, string, string, []string) ([]models.TracfinDecisionSource, error)
}
type tracfinObjectReader interface {
	QueryIngestedObjectByUniqueField(context.Context, repositories.Executor, models.Table, string, string, ...string) ([]models.DataModelObject, error)
	ListIngestedObjects(context.Context, repositories.Executor, models.Table, models.ExplorationOptions, *string, int, ...string) ([]models.DataModelObject, error)
}
type TracfinUsecase struct {
	orgID                     uuid.UUID
	repository                tracfinSourceRepository
	cases                     tracfinCaseReader
	dataModelUsecase          ingestedDataReaderDataModelUsecase
	objects                   tracfinObjectReader
	executorFactory           tracfinExecutorFactory
	draftDependencies         *TracfinDraftDependencies
	configurationDependencies *TracfinConfigurationDependencies
}

func NewTracfinUsecase(orgID uuid.UUID, repository tracfinSourceRepository, cases tracfinCaseReader, dataModel ingestedDataReaderDataModelUsecase, objects tracfinObjectReader, executors tracfinExecutorFactory, drafts ...TracfinDraftDependencies) TracfinUsecase {
	uc := TracfinUsecase{orgID: orgID, repository: repository, cases: cases, dataModelUsecase: dataModel, objects: objects, executorFactory: executors}
	if len(drafts) > 0 {
		uc.draftDependencies = &drafts[0]
	}
	return uc
}

type TracfinRoot struct {
	Table    string   `json:"table"`
	RecordID string   `json:"record_id"`
	Origins  []string `json:"origins"` // case_manual_entity and decision_pivot remain distinct
}
type TracfinContext struct {
	Roots       []TracfinRoot              `json:"roots"`
	Diagnostics []models.TracfinDiagnostic `json:"diagnostics"`
}
type TracfinNavigationPage struct {
	Objects     []models.DataModelObject   `json:"objects"`
	HasNextPage bool                       `json:"has_next_page"`
	NextCursor  *string                    `json:"next_cursor,omitempty"`
	Diagnostics []models.TracfinDiagnostic `json:"diagnostics"`
}

func (uc TracfinUsecase) authorize(ctx context.Context, caseID string) (models.Case, error) {
	c, err := uc.cases.GetCase(ctx, caseID)
	if err != nil {
		return c, err
	}
	if c.Id != caseID || c.OrganizationId != uc.orgID {
		return models.Case{}, models.NotFoundError
	}
	return c, nil
}
func (uc TracfinUsecase) GetContext(ctx context.Context, caseID string) (TracfinContext, error) {
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return TracfinContext{}, err
	}
	dm, err := uc.dataModelUsecase.GetDataModel(ctx, uc.orgID, models.DataModelReadOptions{}, false)
	if err != nil {
		return TracfinContext{}, err
	}
	pivots, err := uc.cases.ReadCasePivotObjects(ctx, caseID)
	if err != nil {
		return TracfinContext{}, err
	}
	out := TracfinContext{}
	positions := map[models.CaseEntityRef]int{}
	add := func(ref models.CaseEntityRef, origin string) {
		table, ok := dm.Tables[ref.TableName]
		if !ok || (table.SemanticType != models.SemanticTypePerson && table.SemanticType != models.SemanticTypeCompany) || ref.ObjectId == "" {
			return
		}
		i, ok := positions[ref]
		if !ok {
			i = len(out.Roots)
			positions[ref] = i
			out.Roots = append(out.Roots, TracfinRoot{Table: ref.TableName, RecordID: ref.ObjectId})
		}
		if !slices.Contains(out.Roots[i].Origins, origin) {
			out.Roots[i].Origins = append(out.Roots[i].Origins, origin)
		}
	}
	for _, entity := range c.Entities {
		add(entity.CaseEntityRef, "case_manual_entity")
	}
	for _, pivot := range pivots {
		if pivot.PivotType == models.PivotTypeObject && pivot.IsIngested {
			add(models.CaseEntityRef{TableName: pivot.PivotObjectName, ObjectId: pivot.PivotObjectId}, "decision_pivot")
		}
	}
	return out, nil
}
func (uc TracfinUsecase) draft(ctx context.Context, caseID string) (models.TracfinDraft, error) {
	if _, err := uc.authorize(ctx, caseID); err != nil {
		return models.TracfinDraft{}, err
	}
	d, err := uc.repository.GetTracfinDraft(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), caseID)
	if err == nil && (d.OrgID != uc.orgID.String() || d.CaseID != caseID) {
		return models.TracfinDraft{}, models.NotFoundError
	}
	return d, err
}

// VerifySelection also serves editing: it is independent from applying proposals.
func (uc TracfinUsecase) VerifySelection(ctx context.Context, caseID string, selection models.TracfinSelection) (models.TracfinSelection, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return models.TracfinSelection{}, err
	}
	_, err = uc.verifySelection(ctx, caseID, d.Configuration, "selection", selection)
	return selection, err
}
func selectionSteps(config models.TracfinConfiguration, s models.TracfinSelection) (map[string]models.TracfinPathStep, error) {
	anchor, ok := tracfin.MappingForSelection(config, s)
	if !ok || !tracfin.CompatibleMapping(anchor, s) {
		return nil, models.BadParameterError
	}
	out := map[string]models.TracfinPathStep{}
	for _, m := range config.Mappings {
		if !tracfin.CompatibleMapping(m, s) {
			continue
		}
		parent := "root"
		for _, step := range m.Source.Path {
			if step.ID == "" || step.ID == "root" || step.ParentID != parent || step.Table == "" || step.Link == "" || (step.Direction != "forward" && step.Direction != "inverse") {
				return nil, models.BadParameterError
			}
			if old, ok := out[step.ID]; ok && (old.ParentID != step.ParentID || old.Table != step.Table || old.Link != step.Link || old.Direction != step.Direction) {
				return nil, models.BadParameterError
			}
			out[step.ID] = step
			parent = step.ID
		}
	}
	return out, nil
}
func (uc TracfinUsecase) readObject(ctx context.Context, dm models.DataModel, table, id string) (tracfin.SourceRecord, error) {
	t, ok := dm.Tables[table]
	if !ok {
		return tracfin.SourceRecord{}, errTracfinSourceUnavailable
	}
	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, uc.orgID)
	if err != nil {
		return tracfin.SourceRecord{}, err
	}
	rows, err := uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, t, id, "object_id")
	if err != nil {
		return tracfin.SourceRecord{}, err
	}
	if len(rows) != 1 || rows[0].Data["object_id"] != id {
		return tracfin.SourceRecord{}, errTracfinSourceUnavailable
	}
	return tracfin.SourceRecord{Table: table, RecordID: id, Data: rows[0].Data}, nil
}

// linkFields validates the configured named edge in the current org datamodel.
// Inverse collections require the exact current navigation option and valid index.
func linkFields(dm models.DataModel, parentTable string, step models.TracfinPathStep) (string, string, *models.NavigationOption, error) {
	parent, ok := dm.Tables[parentTable]
	if !ok {
		return "", "", nil, errTracfinSourceUnavailable
	}
	target, ok := dm.Tables[step.Table]
	if !ok {
		return "", "", nil, errTracfinSourceUnavailable
	}
	switch step.Direction {
	case "forward":
		link, ok := parent.LinksToSingle[step.Link]
		if !ok || link.ChildTableName != parentTable || link.ParentTableName != step.Table {
			return "", "", nil, errTracfinLinkUnavailable
		}
		return link.ChildFieldName, link.ParentFieldName, nil, nil
	case "inverse":
		link, ok := target.LinksToSingle[step.Link]
		if !ok || link.ParentTableName != parentTable || link.ChildTableName != step.Table {
			return "", "", nil, errTracfinLinkUnavailable
		}
		for _, nav := range parent.NavigationOptions {
			if nav.SourceFieldName == link.ParentFieldName && nav.TargetTableName == step.Table && nav.FilterFieldName == link.ChildFieldName && nav.Status == models.IndexStatusValid {
				return link.ParentFieldName, link.ChildFieldName, &nav, nil
			}
		}
		return "", "", nil, errTracfinIndexUnavailable
	}
	return "", "", nil, models.BadParameterError
}
func (uc TracfinUsecase) verifySelection(ctx context.Context, caseID string, config models.TracfinConfiguration, blockID string, s models.TracfinSelection) (tracfin.SourceSet, error) {
	return uc.loadSelection(ctx, caseID, config, blockID, s, false)
}

// Prefill reads persisted selections that may have lost individual sources.
// Selection edits and navigation retain strict verification of every binding.
func (uc TracfinUsecase) loadSelection(ctx context.Context, caseID string, config models.TracfinConfiguration, blockID string, s models.TracfinSelection, allowUnavailable bool) (tracfin.SourceSet, error) {
	if _, err := uc.authorize(ctx, caseID); err != nil {
		return tracfin.SourceSet{}, err
	}
	out := tracfin.SourceSet{Records: map[tracfin.SourceKey]tracfin.SourceRecord{}, Decisions: map[string]models.TracfinDecisionSource{}}
	if s.DecisionID != "" {
		anchor, ok := tracfin.MappingForSelection(config, s)
		if !ok || !tracfin.CompatibleMapping(anchor, s) || len(s.Bindings) > 0 || s.Root.RecordID != "" {
			return out, models.BadParameterError
		}
		rows, err := uc.repository.GetTracfinDecisionSources(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), caseID, []string{s.DecisionID})
		if err != nil {
			return out, err
		}
		if len(rows) != 1 || rows[0].DecisionID != s.DecisionID || rows[0].HistoricalType != s.HistoricalType {
			return out, errTracfinSourceUnavailable
		}
		out.Decisions[s.DecisionID] = rows[0]
		return out, nil
	}
	allowed, err := selectionSteps(config, s)
	if err != nil {
		return out, err
	}
	if s.Root.ID != "root" || s.Root.ParentID != "" || s.Root.Link != "" || s.Root.Direction != "" || len(s.Bindings) > 20 {
		return out, models.BadParameterError
	}
	roots, err := uc.GetContext(ctx, caseID)
	if err != nil {
		return out, err
	}
	found := false
	for _, r := range roots.Roots {
		if r.Table == s.Root.Table && r.RecordID == s.Root.RecordID {
			found = true
		}
	}
	if !found {
		return out, errTracfinSourceUnavailable
	}
	dm, err := uc.dataModelUsecase.GetDataModel(ctx, uc.orgID, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return out, err
	}
	root, err := uc.readObject(ctx, dm, s.Root.Table, s.Root.RecordID)
	if err != nil {
		return out, err
	}
	out.Records[tracfin.SourceKey{blockID, "root"}] = root
	seen := map[string]bool{"root": true}
	unavailable := map[string]bool{}
	skipUnavailable := func(bindingID string, err error) bool {
		code := tracfinAvailabilityCode(err)
		if !allowUnavailable || code == "" {
			return false
		}
		unavailable[bindingID] = true
		out.Diagnostics = append(out.Diagnostics, models.TracfinDiagnostic{Code: code, Layer: "sources", Severity: "warning", Message: code, Field: &models.TracfinFieldRef{BlockID: blockID}})
		return true
	}
	for _, binding := range s.Bindings {
		step, ok := allowed[binding.ID]
		if !ok || seen[binding.ID] || binding.RecordID == "" || binding.ParentID != step.ParentID || binding.Table != step.Table || binding.Link != step.Link || binding.Direction != step.Direction {
			return out, models.BadParameterError
		}
		seen[binding.ID] = true
		parent, ok := out.Records[tracfin.SourceKey{blockID, binding.ParentID}]
		if !ok {
			if allowUnavailable && unavailable[binding.ParentID] {
				unavailable[binding.ID] = true
				continue
			}
			return out, models.BadParameterError
		}
		parentField, childField, _, err := linkFields(dm, parent.Table, step)
		if err != nil {
			if skipUnavailable(binding.ID, err) {
				continue
			}
			return out, err
		}
		child, err := uc.readObject(ctx, dm, binding.Table, binding.RecordID)
		if err != nil {
			if skipUnavailable(binding.ID, err) {
				continue
			}
			return out, err
		}
		// Exact strings are the supported relation keys. No float conversion or null equality.
		p, pok := parent.Data[parentField].(string)
		c, cok := child.Data[childField].(string)
		if !pok || !cok || p == "" || p != c {
			return out, errors.Wrap(models.BadParameterError, "selected object does not belong to selected parent")
		}
		out.Records[tracfin.SourceKey{blockID, binding.ID}] = child
	}
	return out, nil
}
func (uc TracfinUsecase) Navigate(ctx context.Context, caseID string, s models.TracfinSelection, stepID string, cursor *string, limit int) (TracfinNavigationPage, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return TracfinNavigationPage{}, err
	}
	return uc.navigate(ctx, caseID, d.Configuration, s, stepID, cursor, limit)
}
func (uc TracfinUsecase) navigate(ctx context.Context, caseID string, config models.TracfinConfiguration, s models.TracfinSelection, stepID string, cursor *string, limit int) (TracfinNavigationPage, error) {
	page := TracfinNavigationPage{}
	if limit < 1 || limit > 100 || s.DecisionID != "" {
		return page, models.BadParameterError
	}
	sources, err := uc.verifySelection(ctx, caseID, config, "navigation", s)
	if err != nil {
		return page, err
	}
	steps, err := selectionSteps(config, s)
	if err != nil {
		return page, err
	}
	step, ok := steps[stepID]
	if !ok {
		return page, models.BadParameterError
	}
	parent, ok := sources.Records[tracfin.SourceKey{"navigation", step.ParentID}]
	if !ok {
		return page, models.BadParameterError
	}
	dm, err := uc.dataModelUsecase.GetDataModel(ctx, uc.orgID, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return page, err
	}
	sourceField, filterField, nav, err := linkFields(dm, parent.Table, step)
	if errors.Is(err, errTracfinIndexUnavailable) {
		page.Diagnostics = []models.TracfinDiagnostic{{Code: "NAVIGATION_INDEX_UNAVAILABLE", Layer: "sources", Severity: "warning", Message: "The selected collection requires an available navigation index"}}
		return page, nil
	}
	if err != nil {
		return page, err
	}
	value, ok := parent.Data[sourceField].(string)
	if !ok || value == "" {
		return page, models.BadParameterError
	}
	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, uc.orgID)
	if err != nil {
		return page, err
	}
	if nav == nil {
		if cursor != nil {
			return page, models.BadParameterError
		}
		page.Objects, err = uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, dm.Tables[step.Table], value, filterField)
		if len(page.Objects) > 1 {
			return page, errors.Wrap(models.UnprocessableEntityError, "TRACFIN forward link is not unique")
		}
		return page, err
	}
	rows, err := uc.objects.ListIngestedObjects(ctx, exec, dm.Tables[step.Table], models.ExplorationOptions{SourceTableName: parent.Table, FilterFieldName: filterField, FilterFieldValue: models.NewStringOrNumberFromString(value), OrderingFieldName: nav.OrderingFieldName}, cursor, limit+1)
	if err != nil {
		return page, err
	}
	page.HasNextPage = len(rows) > limit
	page.Objects = rows[:min(limit, len(rows))]
	if page.HasNextPage {
		id, ok := page.Objects[len(page.Objects)-1].Data["object_id"].(string)
		if !ok || id == "" {
			return page, fmt.Errorf("navigation result has no object ID")
		}
		page.NextCursor = &id
	}
	return page, nil
}
func (uc TracfinUsecase) Prefill(ctx context.Context, caseID string) (tracfin.PrefillResult, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return tracfin.PrefillResult{}, err
	}
	return uc.prefillDraft(ctx, caseID, d)
}

func (uc TracfinUsecase) prefillDraft(ctx context.Context, caseID string, d models.TracfinDraft) (tracfin.PrefillResult, error) {
	sources := tracfin.SourceSet{Records: map[tracfin.SourceKey]tracfin.SourceRecord{}, Decisions: map[string]models.TracfinDecisionSource{}}
	var loadErr error
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if loadErr != nil || b.Selection == nil {
			return
		}
		loaded, err := uc.loadSelection(ctx, caseID, d.Configuration, b.ID, *b.Selection, true)
		if err != nil {
			code := tracfinAvailabilityCode(err)
			if code == "" {
				loadErr = err
				return
			}
			sources.Diagnostics = append(sources.Diagnostics, models.TracfinDiagnostic{Code: code, Layer: "sources", Severity: "warning", Message: code, Field: &models.TracfinFieldRef{BlockID: b.ID}})
		}
		sources.Diagnostics = append(sources.Diagnostics, loaded.Diagnostics...)
		for k, v := range loaded.Records {
			sources.Records[k] = v
		}
		for k, v := range loaded.Decisions {
			sources.Decisions[k] = v
		}
	})
	if loadErr != nil {
		return tracfin.PrefillResult{}, loadErr
	}
	return tracfin.ResolvePrefill(d.Configuration, d, sources), nil
}

func tracfinAvailabilityCode(err error) string {
	switch {
	case errors.Is(err, errTracfinSourceUnavailable):
		return "SOURCE_UNAVAILABLE"
	case errors.Is(err, errTracfinIndexUnavailable):
		return "NAVIGATION_INDEX_UNAVAILABLE"
	case errors.Is(err, errTracfinLinkUnavailable):
		return "LINK_UNAVAILABLE"
	}
	return ""
}
