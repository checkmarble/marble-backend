package sargen

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// ---- decisions ----

// fakeDecisions holds the decisions of every organization and case. Add records
// a decision in DefaultCase, AddInCase in the given case. The case readers only
// return the decisions of the requested organization and case, like the real
// queries (d.org_id and d.case_id).
type fakeDecisions struct {
	mu          sync.Mutex
	List        []models.DecisionWithRulesAndScreeningsBaseInfo
	PivotValues []models.PivotDataWithCount
	// DefaultCase is the case of the decisions recorded with Add.
	DefaultCase string
	caseOf      map[uuid.UUID]string
	// LightCalls counts DecisionsByCaseId calls (no rules, no screenings).
	LightCalls int
}

// Add records a decision whose trigger object is (table, data) in DefaultCase
// and returns its id. data should contain "object_id".
func (d *fakeDecisions) Add(orgId uuid.UUID, table string, data map[string]any, createdAt time.Time) uuid.UUID {
	return d.AddInCase(orgId, d.DefaultCase, table, data, createdAt)
}

// AddInCase records a decision of orgId attached to caseId.
func (d *fakeDecisions) AddInCase(orgId uuid.UUID, caseId string, table string,
	data map[string]any, createdAt time.Time,
) uuid.UUID {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.caseOf == nil {
		d.caseOf = map[uuid.UUID]string{}
	}
	id := uuid.New()
	d.List = append(d.List, models.DecisionWithRulesAndScreeningsBaseInfo{Decision: models.Decision{
		DecisionId: id, OrganizationId: orgId, CreatedAt: createdAt,
		ClientObject: models.ClientObject{TableName: table, Data: data},
	}})
	d.caseOf[id] = caseId
	return id
}

// inScope returns the decisions of orgId attached to caseId, newest first.
// Callers hold d.mu.
func (d *fakeDecisions) inScope(orgId uuid.UUID, caseId string) []models.DecisionWithRulesAndScreeningsBaseInfo {
	out := []models.DecisionWithRulesAndScreeningsBaseInfo{}
	for _, x := range d.List {
		if x.OrganizationId == orgId && d.caseOf[x.DecisionId] == caseId {
			out = append(out, x)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// DecisionsByCaseId returns every decision of orgId in caseId, newest first,
// without rules or screenings.
func (d *fakeDecisions) DecisionsByCaseId(_ context.Context, _ repositories.Executor,
	orgId uuid.UUID, caseId string,
) ([]models.Decision, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.LightCalls++
	out := []models.Decision{}
	for _, x := range d.inScope(orgId, caseId) {
		out = append(out, x.Decision)
	}
	return out, nil
}

func (d *fakeDecisions) DecisionPivotValuesByCase(_ context.Context, _ repositories.Executor, _ string) ([]models.PivotDataWithCount, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.PivotValues, nil
}

// ---- data model ----

// fakeDataModel serves Model to the organization OrgId only; any other
// organization gets an empty data model. Orgs records each requested
// organization.
type fakeDataModel struct {
	mu    sync.Mutex
	Model models.DataModel
	OrgId uuid.UUID
	Orgs  []uuid.UUID
}

func (m *fakeDataModel) GetDataModel(_ context.Context, orgId uuid.UUID, _ models.DataModelReadOptions, _ bool) (models.DataModel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Orgs = append(m.Orgs, orgId)
	if orgId != m.OrgId {
		return models.DataModel{}, nil
	}
	return m.Model, nil
}

// ---- ingested data ----

// ingestedEntry is one stored object of one organization.
type ingestedEntry struct {
	orgId uuid.UUID
	obj   models.ClientObjectDetail
}

// fakeIngested is the ingestion reader. Objects are keyed by organization,
// table and object_id: a reader called with an organization only sees the
// objects of that organization. Put stores objects of OrgId.
// Deleted(true) makes every read of an object return NotFound (source
// deletion); the real reader may also return an empty list, handle both.
type fakeIngested struct {
	mu      sync.Mutex
	OrgId   uuid.UUID
	objects map[string]ingestedEntry
	deleted bool
	// Pivots is returned by ReadPivotObjectsFromValues (for OrgId only).
	Pivots []models.PivotObject
	// model, when set, is used to check that explorations match a navigation option.
	model *fakeDataModel
	// Reads counts GetIngestedObject + ReadIngestedClientObjects calls.
	Reads int
	// OrgIds records the organization of every call of the reader.
	OrgIds []uuid.UUID
}

func newFakeIngested() *fakeIngested {
	return &fakeIngested{objects: map[string]ingestedEntry{}}
}

func ingestedKey(orgId uuid.UUID, table, id string) string {
	return orgId.String() + "/" + table + "/" + id
}

// Put stores an ingested object of OrgId (data should contain "object_id").
func (i *fakeIngested) Put(table, objectId string, data map[string]any, validFrom time.Time) {
	i.PutInOrg(i.OrgId, table, objectId, data, validFrom)
}

// PutInOrg stores an ingested object of the given organization.
func (i *fakeIngested) PutInOrg(orgId uuid.UUID, table, objectId string, data map[string]any, validFrom time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	vf := validFrom
	i.objects[ingestedKey(orgId, table, objectId)] = ingestedEntry{orgId: orgId, obj: models.ClientObjectDetail{
		Data: data, Metadata: models.ClientObjectMetadata{ValidFrom: &vf, ObjectType: table},
	}}
}

// Remove deletes one object of OrgId.
func (i *fakeIngested) Remove(table, objectId string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.objects, ingestedKey(i.OrgId, table, objectId))
}

func (i *fakeIngested) SetDeleted(v bool) { i.mu.Lock(); i.deleted = v; i.mu.Unlock() }

func (i *fakeIngested) GetIngestedObject(_ context.Context, orgId uuid.UUID, _ *models.DataModel,
	objectType, value, field string,
) ([]models.ClientObjectDetail, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Reads++
	i.OrgIds = append(i.OrgIds, orgId)
	if i.deleted {
		return nil, errors.Wrap(models.NotFoundError, "ingested object deleted")
	}
	out := []models.ClientObjectDetail{}
	for _, e := range i.objects {
		if e.orgId == orgId && e.obj.Metadata.ObjectType == objectType && e.obj.Data[field] == value {
			out = append(out, e.obj)
		}
	}
	if len(out) == 0 {
		return nil, errors.Wrap(models.NotFoundError, "ingested object")
	}
	return out, nil
}

// ReadIngestedClientObjects returns the objects of objectType whose
// FilterFieldName equals the filter value, ordered by object_id, paginated by
// an integer offset cursor. The exploration must match a navigation option of the fake data model, else UnprocessableEntityError.
func (i *fakeIngested) ReadIngestedClientObjects(_ context.Context, orgId uuid.UUID, objectType string,
	input models.ClientDataListRequestBody, _ ...string,
) ([]models.ClientObjectDetail, []models.FieldStatistics, models.ClientDataListPagination, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Reads++
	i.OrgIds = append(i.OrgIds, orgId)
	if i.deleted {
		return nil, nil, models.ClientDataListPagination{}, errors.Wrap(models.NotFoundError, "ingested objects deleted")
	}
	ex := input.ExplorationOptions
	if i.model != nil {
		ok := false
		for _, t := range i.model.Model.Tables {
			for _, o := range t.NavigationOptions {
				if o.SourceTableName == ex.SourceTableName && o.FilterFieldName == ex.FilterFieldName &&
					o.OrderingFieldName == ex.OrderingFieldName && o.TargetTableName == objectType {
					ok = true
				}
			}
		}
		if !ok {
			return nil, nil, models.ClientDataListPagination{},
				errors.Wrap(models.UnprocessableEntityError, "no matching navigation option")
		}
		// Like the real reader: the filter is typed from the DataType of the
		// filtered field of the target table.
		field, found := i.model.Model.Tables[objectType].Fields[ex.FilterFieldName]
		if !found {
			return nil, nil, models.ClientDataListPagination{},
				errors.Wrap(models.NotFoundError, "filter field not found")
		}
		switch field.DataType {
		case models.String:
			if ex.FilterFieldValue.StringValue == nil {
				return nil, nil, models.ClientDataListPagination{},
					errors.Wrap(models.UnprocessableEntityError, "filter field must be a string")
			}
		case models.Float:
			if ex.FilterFieldValue.FloatValue == nil {
				return nil, nil, models.ClientDataListPagination{},
					errors.Wrap(models.UnprocessableEntityError, "filter field must be a number")
			}
		default:
			return nil, nil, models.ClientDataListPagination{},
				errors.Wrap(models.UnprocessableEntityError, "filter field type not supported")
		}
	}
	var matches []models.ClientObjectDetail
	for _, e := range i.objects {
		o := e.obj
		if e.orgId != orgId || o.Metadata.ObjectType != objectType {
			continue
		}
		v, ok := o.Data[ex.FilterFieldName]
		switch {
		case !ok:
		case ex.FilterFieldValue.StringValue != nil && v == *ex.FilterFieldValue.StringValue:
			matches = append(matches, o)
		case ex.FilterFieldValue.FloatValue != nil && v == *ex.FilterFieldValue.FloatValue:
			matches = append(matches, o)
		}
	}
	sort.Slice(matches, func(a, b int) bool {
		return matches[a].Data["object_id"].(string) < matches[b].Data["object_id"].(string)
	})
	start := 0
	if input.CursorId != nil {
		start, _ = strconv.Atoi(*input.CursorId)
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 25
	}
	end := min(start+limit, len(matches))
	page := models.ClientDataListPagination{}
	if end < len(matches) {
		next := strconv.Itoa(end)
		page = models.ClientDataListPagination{NextCursorId: &next, HasNextPage: true}
	}
	if start > len(matches) {
		start = len(matches)
	}
	return matches[start:end], nil, page, nil
}

// ReadPivotObjectsFromValues serves Pivots to OrgId only.
func (i *fakeIngested) ReadPivotObjectsFromValues(_ context.Context, orgId uuid.UUID, _ []models.PivotDataWithCount) ([]models.PivotObject, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.OrgIds = append(i.OrgIds, orgId)
	if orgId != i.OrgId {
		return nil, nil
	}
	return i.Pivots, nil
}

// ---- case entities ----

// fakeCaseEntities lists Refs as the manual entities of CaseId only, and
// hydrates them from the ingested fake of the requested organization (deleted
// or missing objects are omitted, like the real reader).
type fakeCaseEntities struct {
	mu       sync.Mutex
	CaseId   string
	Refs     []models.CaseEntityRef
	ingested *fakeIngested
}

func (c *fakeCaseEntities) ListCaseManualEntities(_ context.Context, _ repositories.Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]models.CaseManualEntity, 0, len(c.Refs))
	if caseId != c.CaseId {
		return out, nil
	}
	for _, r := range c.Refs {
		out = append(out, models.CaseManualEntity{Id: uuid.NewString(), OrganizationId: orgId, CaseId: caseId, CaseEntityRef: r})
	}
	return out, nil
}

func (c *fakeCaseEntities) ReadCaseEntityObjects(_ context.Context, orgId uuid.UUID, refs []models.CaseEntityRef) (map[models.CaseEntityRef]models.DataModelObject, error) {
	c.ingested.mu.Lock()
	defer c.ingested.mu.Unlock()
	out := map[models.CaseEntityRef]models.DataModelObject{}
	if c.ingested.deleted {
		return out, nil
	}
	for _, r := range refs {
		if e, ok := c.ingested.objects[ingestedKey(orgId, r.TableName, r.ObjectId)]; ok {
			out[r] = models.DataModelObject{Data: e.obj.Data}
		}
	}
	return out, nil
}

// ---- selection repository ----

// fakeSelectionRepo is an in-memory SarSelectionRepository. Add returns
// models.ConflictError on a duplicate (declaration, table, object).
type fakeSelectionRepo struct {
	mu         sync.Mutex
	Rows       []models.SarSelectedObject
	Exclusions []models.CaseEntityRef
}

func (r *fakeSelectionRepo) ClearSarSelectionAutoAdded(_ context.Context, _ repositories.Executor, declarationId uuid.UUID, refs []models.CaseEntityRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, o := range r.Rows {
		for _, ref := range refs {
			if o.DeclarationId == declarationId && ref.TableName == o.TableName && ref.ObjectId == o.ObjectId {
				r.Rows[i].AutoAdded, r.Rows[i].ContextOnly = false, false
			}
		}
	}
	return nil
}

func (r *fakeSelectionRepo) ListSarSelectionExclusions(_ context.Context, _ repositories.Executor, _ uuid.UUID) ([]models.CaseEntityRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]models.CaseEntityRef{}, r.Exclusions...), nil
}

func (r *fakeSelectionRepo) AddSarSelectionExclusions(_ context.Context, _ repositories.Executor, _ uuid.UUID, refs []models.CaseEntityRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ref := range refs {
		if !slices.Contains(r.Exclusions, ref) {
			r.Exclusions = append(r.Exclusions, ref)
		}
	}
	return nil
}

func (r *fakeSelectionRepo) RemoveSarSelectionExclusions(_ context.Context, _ repositories.Executor, _ uuid.UUID, refs []models.CaseEntityRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.Exclusions[:0]
	for _, e := range r.Exclusions {
		if !slices.Contains(refs, e) {
			kept = append(kept, e)
		}
	}
	r.Exclusions = kept
	return nil
}

func (r *fakeSelectionRepo) ListSarSelection(_ context.Context, _ repositories.Executor, declarationId uuid.UUID) ([]models.SarSelectedObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.SarSelectedObject{}
	for _, o := range r.Rows {
		if o.DeclarationId == declarationId {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *fakeSelectionRepo) AddSarSelection(_ context.Context, _ repositories.Executor, objs []models.SarSelectedObject) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range objs {
		for _, o := range r.Rows {
			if o.DeclarationId == n.DeclarationId && o.TableName == n.TableName && o.ObjectId == n.ObjectId {
				return errors.Wrap(models.ConflictError, "objet déjà sélectionné")
			}
		}
	}
	for _, n := range objs {
		if n.CreatedAt.IsZero() {
			n.CreatedAt = time.Now()
		}
		r.Rows = append(r.Rows, jsonRoundTrip(n))
	}
	return nil
}

// jsonRoundTrip stores the row like the real repository does: snapshot, origin
// and reached_from go through JSON (numbers become float64, times strings in
// the snapshot, UUIDs and times in origin are re-parsed).
func jsonRoundTrip(o models.SarSelectedObject) models.SarSelectedObject {
	snap, err := json.Marshal(o.Snapshot)
	if err != nil {
		panic(err)
	}
	var back map[string]any
	if err := json.Unmarshal(snap, &back); err != nil {
		panic(err)
	}
	o.Snapshot = back
	for _, v := range []any{&o.Origin, &o.ReachedFrom} {
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(raw, v); err != nil {
			panic(err)
		}
	}
	return o
}

func (r *fakeSelectionRepo) RemoveSarSelection(_ context.Context, _ repositories.Executor, declarationId uuid.UUID, refs []models.CaseEntityRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.Rows[:0]
	for _, o := range r.Rows {
		drop := false
		if o.DeclarationId == declarationId {
			for _, ref := range refs {
				if ref.TableName == o.TableName && ref.ObjectId == o.ObjectId {
					drop = true
				}
			}
		}
		if !drop {
			kept = append(kept, o)
		}
	}
	r.Rows = kept
	return nil
}
