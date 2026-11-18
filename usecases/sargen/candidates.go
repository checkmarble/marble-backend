package sargen

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	ReasonTriggerObject = "trigger_object"
	ReasonPivot         = "pivot"
	ReasonCaseEntity    = "case_entity"
	// ReasonLinked marks an object reached by navigating a link.
	ReasonLinked = "linked"

	LinkKindToSingle = "to_single"
	LinkKindToMany   = "to_many"

	// MaxNavigateLimit caps the page size of Navigate.
	MaxNavigateLimit = 100
)

// Candidate is an object the analyst may select.
type Candidate struct {
	Ref      core.ObjectRef
	Reason   string
	Summary  map[string]any
	Links    []LinkOption
	Selected bool
}

// LinkOption is a link that can be followed from a candidate.
type LinkOption struct {
	Name        string
	Kind        string
	TargetTable string
}

// DecisionReader is the subset of the decision repository used to list the
// objects behind a case.
type DecisionReader interface {
	// DecisionsByCaseId lists every decision of the case in one query, without
	// rules or screenings (not needed to list or navigate objects).
	DecisionsByCaseId(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, caseId string) ([]models.Decision, error)
	DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error)
}

// CaseEntityLister lists the entities manually added to a case.
type CaseEntityLister interface {
	ListCaseManualEntities(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error)
}

// CaseEntityObjectReader hydrates case entity references (satisfied by the
// usecases case entity reader).
type CaseEntityObjectReader interface {
	ReadCaseEntityObjects(ctx context.Context, orgId uuid.UUID, refs []models.CaseEntityRef) (map[models.CaseEntityRef]models.DataModelObject, error)
}

// IngestedDataReader is the subset of usecases.IngestedDataReaderUsecase used here.
type IngestedDataReader interface {
	GetIngestedObject(ctx context.Context, organizationId uuid.UUID, dataModel *models.DataModel,
		objectType, uniqueFieldValue, uniqueFieldName string) ([]models.ClientObjectDetail, error)
	ReadIngestedClientObjects(ctx context.Context, orgId uuid.UUID, objectType string,
		input models.ClientDataListRequestBody, fieldsToRead ...string,
	) ([]models.ClientObjectDetail, []models.FieldStatistics, models.ClientDataListPagination, error)
	ReadPivotObjectsFromValues(ctx context.Context, orgId uuid.UUID, values []models.PivotDataWithCount) ([]models.PivotObject, error)
}

// DataModelReader reads the organization data model.
type DataModelReader interface {
	GetDataModel(ctx context.Context, organizationID uuid.UUID, options models.DataModelReadOptions,
		useCache bool) (models.DataModel, error)
}

// triggerInfo is the trigger object of the most recent decision having it.
type triggerInfo struct {
	decisionId uuid.UUID
	createdAt  time.Time
	data       map[string]any
}

type candidateInfo struct {
	reason  string
	data    map[string]any
	trigger *triggerInfo
}

// selectionContext is what ListCandidates, Navigate and UpdateSelection share.
type selectionContext struct {
	dataModel  models.DataModel
	candidates map[core.ObjectRef]*candidateInfo
	order      []core.ObjectRef
	selected   map[core.ObjectRef]models.SarSelectedObject
}

// dataOf returns the data of a candidate or selected object (frozen copy first).
func (c selectionContext) dataOf(ref core.ObjectRef) (map[string]any, bool) {
	// The frozen copy of an already selected object wins over the live data.
	if sel, ok := c.selected[ref]; ok && sel.Snapshot != nil {
		return sel.Snapshot, true
	}
	cand, isCand := c.candidates[ref]
	if isCand && cand.data != nil {
		return cand.data, true
	}
	// A known object without data: reachable, nothing to follow from it.
	if _, isSel := c.selected[ref]; isSel || isCand {
		return nil, true
	}
	return nil, false
}

func objectIdOf(data map[string]any) (string, bool) {
	id, ok := data["object_id"].(string)
	return id, ok && id != ""
}

// loadSelectionContext gathers the candidates (all decisions of the case,
// pivot objects, case entities) and the current selection, always with the
// organization of the declaration (checked against the credentials).
func (uc *Usecase) loadSelectionContext(ctx context.Context, exec repositories.Executor,
	orgId uuid.UUID, d models.SarDeclaration,
) (selectionContext, error) {
	c := selectionContext{
		candidates: map[core.ObjectRef]*candidateInfo{},
		selected:   map[core.ObjectRef]models.SarSelectedObject{},
	}
	dm, err := uc.dataModelReader.GetDataModel(ctx, orgId,
		models.DataModelReadOptions{IncludeNavigationOptions: true}, true)
	if err != nil {
		return c, err
	}
	c.dataModel = dm

	sel, err := uc.selectionRepo.ListSarSelection(ctx, exec, d.Id)
	if err != nil {
		return c, err
	}
	for _, o := range sel {
		c.selected[core.ObjectRef{Table: o.TableName, ObjectID: o.ObjectId}] = o
	}

	caseId := d.CaseId.String()
	// 1. Trigger objects of every decision of the case.
	decisions, err := uc.decisionReader.DecisionsByCaseId(ctx, exec, orgId, caseId)
	if err != nil {
		return c, err
	}
	for _, dec := range decisions {
		id, ok := objectIdOf(dec.ClientObject.Data)
		if !ok {
			continue
		}
		ref := core.ObjectRef{Table: dec.ClientObject.TableName, ObjectID: id}
		cand := c.candidates[ref]
		if cand == nil {
			cand = &candidateInfo{reason: ReasonTriggerObject}
			c.candidates[ref] = cand
			c.order = append(c.order, ref)
		}
		if cand.trigger == nil || dec.CreatedAt.After(cand.trigger.createdAt) {
			cand.trigger = &triggerInfo{decisionId: dec.DecisionId, createdAt: dec.CreatedAt, data: dec.ClientObject.Data}
			cand.data = dec.ClientObject.Data
		}
	}

	add := func(ref core.ObjectRef, reason string, data map[string]any) {
		if _, ok := c.candidates[ref]; ok {
			return
		}
		c.candidates[ref] = &candidateInfo{reason: reason, data: data}
		c.order = append(c.order, ref)
	}

	// 2. Pivot objects.
	pivotValues, err := uc.decisionReader.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return c, err
	}
	{
		pivots, err := uc.ingestedReader.ReadPivotObjectsFromValues(ctx, orgId, pivotValues)
		if err != nil {
			return c, err
		}
		for _, p := range pivots {
			if !p.IsIngested || p.PivotObjectId == "" {
				continue
			}
			add(core.ObjectRef{Table: p.PivotObjectName, ObjectID: p.PivotObjectId}, ReasonPivot, p.PivotObjectData.Data)
		}
	}

	// 3. Entities added to the case.
	entities, err := uc.caseEntityLister.ListCaseManualEntities(ctx, exec, orgId, caseId)
	if err != nil {
		return c, err
	}
	if len(entities) > 0 {
		refs := make([]models.CaseEntityRef, len(entities))
		for i, e := range entities {
			refs[i] = e.CaseEntityRef
		}
		objs, err := uc.caseEntityObjects.ReadCaseEntityObjects(ctx, orgId, refs)
		if err != nil {
			return c, err
		}
		for _, r := range refs {
			var data map[string]any
			if o, ok := objs[r]; ok {
				data = o.Data
			}
			add(core.ObjectRef{Table: r.TableName, ObjectID: r.ObjectId}, ReasonCaseEntity, data)
		}
	}
	return c, nil
}

// linkInfo is a followable link with what is needed to follow it.
type linkInfo struct {
	LinkOption
	single *models.LinkToSingle
	nav    *models.NavigationOption
}

func valueString(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, x != ""
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return fmt.Sprint(x), true
	}
}

// reaches tells whether target is the object the link leads to from fromData.
func (l linkInfo) reaches(fromData, targetData map[string]any) bool {
	var fromField, targetField string
	if l.single != nil {
		fromField, targetField = l.single.ChildFieldName, l.single.ParentFieldName
	} else {
		fromField, targetField = l.nav.SourceFieldName, l.nav.FilterFieldName
	}
	fv, ok := valueString(fromData[fromField])
	if !ok {
		return false
	}
	tv, ok := valueString(targetData[targetField])
	return ok && fv == tv
}

func toManyName(o models.NavigationOption) string { return o.TargetTableName + "." + o.FilterFieldName }

// linksOf lists the links followable from a table: to-single links of the
// data model and navigation options with a valid index.
func linksOf(dm models.DataModel, table string) []linkInfo {
	t, ok := dm.Tables[table]
	if !ok {
		return nil
	}
	var out []linkInfo
	names := make([]string, 0, len(t.LinksToSingle))
	for n := range t.LinksToSingle {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		l := t.LinksToSingle[n]
		out = append(out, linkInfo{
			LinkOption: LinkOption{Name: n, Kind: LinkKindToSingle, TargetTable: l.ParentTableName},
			single:     &l,
		})
	}
	seen := map[string]bool{}
	for _, o := range t.NavigationOptions {
		name := toManyName(o)
		if o.Status != models.IndexStatusValid || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, linkInfo{
			LinkOption: LinkOption{Name: name, Kind: LinkKindToMany, TargetTable: o.TargetTableName},
			nav:        &o,
		})
	}
	return out
}

func findLink(dm models.DataModel, table, name string) (linkInfo, bool) {
	for _, l := range linksOf(dm, table) {
		if l.Name == name {
			return l, true
		}
	}
	return linkInfo{}, false
}

func (uc *Usecase) toCandidate(c selectionContext, ref core.ObjectRef, reason string, data map[string]any) Candidate {
	links := linksOf(c.dataModel, ref.Table)
	opts := make([]LinkOption, len(links))
	for i, l := range links {
		opts[i] = l.LinkOption
	}
	_, selected := c.selected[ref]
	return Candidate{Ref: ref, Reason: reason, Summary: data, Links: opts, Selected: selected}
}

// ListCandidates returns the trigger objects of all the case decisions, the
// pivot objects and the case entities, deduplicated by ObjectRef.
func (uc *Usecase) ListCandidates(ctx context.Context, id uuid.UUID) ([]Candidate, error) {
	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return nil, err
	}
	c, err := uc.loadSelectionContext(ctx, exec, d.OrganizationId, d)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(c.order))
	for _, ref := range c.order {
		cand := c.candidates[ref]
		out = append(out, uc.toCandidate(c, ref, cand.reason, cand.data))
	}
	return out, nil
}

// filterValue types the exploration filter from the DataType of the filtered
// field of the target table (like the real reader expects), not from the Go
// type of the source value.
func filterValue(dataType models.DataType, v any) (models.StringOrNumber, error) {
	switch dataType {
	case models.Float, models.Int:
		switch x := v.(type) {
		case float64:
			return models.StringOrNumber{FloatValue: &x}, nil
		case int:
			f := float64(x)
			return models.StringOrNumber{FloatValue: &f}, nil
		case int64:
			f := float64(x)
			return models.StringOrNumber{FloatValue: &f}, nil
		case string:
			f, err := strconv.ParseFloat(x, 64)
			if err != nil {
				return models.StringOrNumber{}, errors.Wrapf(models.BadParameterError,
					"la valeur %q n'est pas un nombre", x)
			}
			return models.StringOrNumber{FloatValue: &f}, nil
		}
		return models.StringOrNumber{}, errors.Wrapf(models.BadParameterError,
			"la valeur %v n'est pas un nombre", v)
	default:
		s, ok := valueString(v)
		if !ok {
			return models.StringOrNumber{}, errors.Wrap(models.BadParameterError, "valeur de filtre vide")
		}
		return models.StringOrNumber{StringValue: &s}, nil
	}
}

// Navigate follows link from an object that is a candidate or already selected.
func (uc *Usecase) Navigate(ctx context.Context, id uuid.UUID, from core.ObjectRef, link string,
	cursor *string, limit int,
) ([]Candidate, *string, error) {
	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return nil, nil, err
	}
	orgId := d.OrganizationId
	c, err := uc.loadSelectionContext(ctx, exec, orgId, d)
	if err != nil {
		return nil, nil, err
	}
	fromData, ok := c.dataOf(from)
	if !ok {
		return nil, nil, errors.Wrapf(models.ForbiddenError,
			"l'objet %s/%s n'est ni un candidat ni sélectionné", from.Table, from.ObjectID)
	}
	l, ok := findLink(c.dataModel, from.Table, link)
	if !ok {
		return nil, nil, errors.Wrapf(models.BadParameterError, "lien inconnu : %q", link)
	}

	if l.single != nil {
		value, ok := valueString(fromData[l.single.ChildFieldName])
		if !ok {
			return []Candidate{}, nil, nil
		}
		objs, err := uc.ingestedReader.GetIngestedObject(ctx, orgId, &c.dataModel,
			l.single.ParentTableName, value, l.single.ParentFieldName)
		if err != nil && !errors.Is(err, models.NotFoundError) {
			return nil, nil, err
		}
		if err != nil || len(objs) == 0 {
			return []Candidate{}, nil, nil
		}
		obj := objs[len(objs)-1]
		objId, ok := objectIdOf(obj.Data)
		if !ok {
			return []Candidate{}, nil, nil
		}
		ref := core.ObjectRef{Table: l.single.ParentTableName, ObjectID: objId}
		return []Candidate{uc.toCandidate(c, ref, ReasonLinked, obj.Data)}, nil, nil
	}

	if limit <= 0 || limit > MaxNavigateLimit {
		limit = MaxNavigateLimit
	}
	sourceValue, ok := fromData[l.nav.SourceFieldName]
	if !ok || sourceValue == nil {
		return []Candidate{}, nil, nil
	}
	targetField, ok := c.dataModel.Tables[l.nav.TargetTableName].Fields[l.nav.FilterFieldName]
	if !ok {
		return nil, nil, errors.Wrapf(models.NotFoundError, "champ de filtre introuvable : %s.%s",
			l.nav.TargetTableName, l.nav.FilterFieldName)
	}
	filter, err := filterValue(targetField.DataType, sourceValue)
	if err != nil {
		return nil, nil, err
	}
	objs, _, page, err := uc.ingestedReader.ReadIngestedClientObjects(ctx, orgId, l.nav.TargetTableName,
		models.ClientDataListRequestBody{
			ExplorationOptions: models.ExplorationOptions{
				SourceTableName:   l.nav.SourceTableName,
				FilterFieldName:   l.nav.FilterFieldName,
				FilterFieldValue:  filter,
				OrderingFieldName: l.nav.OrderingFieldName,
			},
			CursorId: cursor,
			Limit:    limit,
		})
	if err != nil {
		return nil, nil, err
	}
	out := make([]Candidate, 0, len(objs))
	for _, o := range objs {
		objId, ok := objectIdOf(o.Data)
		if !ok {
			continue
		}
		out = append(out, uc.toCandidate(c, core.ObjectRef{Table: l.nav.TargetTableName, ObjectID: objId}, ReasonLinked, o.Data))
	}
	var next *string
	if page.HasNextPage {
		next = page.NextCursorId
	}
	return out, next, nil
}
