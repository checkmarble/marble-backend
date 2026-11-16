package sargen

import (
	"context"
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

// fakeDecisions serves f.Decisions.List newest first (CreatedAt desc) with
// cursor pagination on the decision id, like DecisionsByCaseIdFromCursor.
// Use Add to append decisions; PivotValues feeds DecisionPivotValuesByCase.
type fakeDecisions struct {
	mu          sync.Mutex
	List        []models.DecisionWithRulesAndScreeningsBaseInfo
	PivotValues []models.PivotDataWithCount
	// Calls counts DecisionsByCaseIdFromCursor calls (to assert pagination is followed).
	Calls int
}

// Add records a decision whose trigger object is (table, data) and returns its id.
// data should contain "object_id".
func (d *fakeDecisions) Add(orgId uuid.UUID, table string, data map[string]any, createdAt time.Time) uuid.UUID {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := uuid.New()
	d.List = append(d.List, models.DecisionWithRulesAndScreeningsBaseInfo{Decision: models.Decision{
		DecisionId: id, OrganizationId: orgId, CreatedAt: createdAt,
		ClientObject: models.ClientObject{TableName: table, Data: data},
	}})
	return id
}

func (d *fakeDecisions) DecisionsByCaseIdFromCursor(_ context.Context, _ repositories.Executor,
	req models.CaseDecisionsRequest,
) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Calls++
	all := append([]models.DecisionWithRulesAndScreeningsBaseInfo(nil), d.List...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	start := 0
	if req.CursorId != "" {
		for i, x := range all {
			if x.DecisionId.String() == req.CursorId {
				start = i + 1
			}
		}
	}
	all = all[start:]
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	if len(all) > limit {
		return all[:limit], true, nil
	}
	return all, false, nil
}

func (d *fakeDecisions) DecisionPivotValuesByCase(_ context.Context, _ repositories.Executor, _ string) ([]models.PivotDataWithCount, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.PivotValues, nil
}

// ---- data model ----

// fakeDataModel returns Model (set Tables with LinksToSingle and
// NavigationOptions per table; see tableWith).
type fakeDataModel struct{ Model models.DataModel }

func (m *fakeDataModel) GetDataModel(_ context.Context, _ uuid.UUID, _ models.DataModelReadOptions, _ bool) (models.DataModel, error) {
	return m.Model, nil
}

// ---- ingested data ----

// fakeIngested is the ingestion reader. Objects are keyed by table/object_id.
// Deleted(true) makes every read of an object return NotFound (source
// deletion); the real reader may also return an empty list, handle both.
type fakeIngested struct {
	mu      sync.Mutex
	objects map[string]models.ClientObjectDetail
	deleted bool
	// Pivots is returned by ReadPivotObjectsFromValues.
	Pivots []models.PivotObject
	// Reads counts GetIngestedObject + ReadIngestedClientObjects calls.
	Reads int
}

func newFakeIngested() *fakeIngested {
	return &fakeIngested{objects: map[string]models.ClientObjectDetail{}}
}

func ingestedKey(table, id string) string { return table + "/" + id }

// Put stores an ingested object (data should contain "object_id").
func (i *fakeIngested) Put(table, objectId string, data map[string]any, validFrom time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	vf := validFrom
	i.objects[ingestedKey(table, objectId)] = models.ClientObjectDetail{
		Data: data, Metadata: models.ClientObjectMetadata{ValidFrom: &vf, ObjectType: table},
	}
}

// Remove deletes one object; SetDeleted(true) makes all reads fail with NotFound.
func (i *fakeIngested) Remove(table, objectId string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.objects, ingestedKey(table, objectId))
}

func (i *fakeIngested) SetDeleted(v bool) { i.mu.Lock(); i.deleted = v; i.mu.Unlock() }

func (i *fakeIngested) GetIngestedObject(_ context.Context, _ uuid.UUID, _ *models.DataModel,
	objectType, value, field string,
) ([]models.ClientObjectDetail, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Reads++
	if i.deleted {
		return nil, errors.Wrap(models.NotFoundError, "ingested object deleted")
	}
	out := []models.ClientObjectDetail{}
	for _, o := range i.objects {
		if o.Metadata.ObjectType == objectType && o.Data[field] == value {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return nil, errors.Wrap(models.NotFoundError, "ingested object")
	}
	return out, nil
}

// ReadIngestedClientObjects returns the objects of objectType whose
// FilterFieldName equals the filter value, ordered by object_id, paginated by
// an integer offset cursor. Navigation options are not checked by the fake.
func (i *fakeIngested) ReadIngestedClientObjects(_ context.Context, _ uuid.UUID, objectType string,
	input models.ClientDataListRequestBody, _ ...string,
) ([]models.ClientObjectDetail, []models.FieldStatistics, models.ClientDataListPagination, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Reads++
	if i.deleted {
		return nil, nil, models.ClientDataListPagination{}, errors.Wrap(models.NotFoundError, "ingested objects deleted")
	}
	ex := input.ExplorationOptions
	var matches []models.ClientObjectDetail
	for _, o := range i.objects {
		if o.Metadata.ObjectType != objectType {
			continue
		}
		if v, ok := o.Data[ex.FilterFieldName]; ok && ex.FilterFieldValue.StringValue != nil && v == *ex.FilterFieldValue.StringValue {
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

func (i *fakeIngested) ReadPivotObjectsFromValues(_ context.Context, _ uuid.UUID, _ []models.PivotDataWithCount) ([]models.PivotObject, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.Pivots, nil
}

// ---- case entities ----

// fakeCaseEntities lists Refs as case manual entities and hydrates them from
// the ingested fake (deleted or missing objects are omitted, like the real reader).
type fakeCaseEntities struct {
	mu       sync.Mutex
	Refs     []models.CaseEntityRef
	ingested *fakeIngested
}

func (c *fakeCaseEntities) ListCaseManualEntities(_ context.Context, _ repositories.Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]models.CaseManualEntity, 0, len(c.Refs))
	for _, r := range c.Refs {
		out = append(out, models.CaseManualEntity{Id: uuid.NewString(), OrganizationId: orgId, CaseId: caseId, CaseEntityRef: r})
	}
	return out, nil
}

func (c *fakeCaseEntities) ReadCaseEntityObjects(_ context.Context, _ uuid.UUID, refs []models.CaseEntityRef) (map[models.CaseEntityRef]models.DataModelObject, error) {
	c.ingested.mu.Lock()
	defer c.ingested.mu.Unlock()
	out := map[models.CaseEntityRef]models.DataModelObject{}
	if c.ingested.deleted {
		return out, nil
	}
	for _, r := range refs {
		if o, ok := c.ingested.objects[ingestedKey(r.TableName, r.ObjectId)]; ok {
			out[r] = models.DataModelObject{Data: o.Data}
		}
	}
	return out, nil
}

// ---- selection repository ----

// fakeSelectionRepo is an in-memory SarSelectionRepository. Add returns
// models.ConflictError on a duplicate (declaration, table, object).
type fakeSelectionRepo struct {
	mu   sync.Mutex
	Rows []models.SarSelectedObject
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
		r.Rows = append(r.Rows, n)
	}
	return nil
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
