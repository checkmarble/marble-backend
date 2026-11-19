package sargen

import (
	"context"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	// maxParentDepth is how many to-single links are followed from a selected
	// object to find its parents (transaction -> account -> person -> company).
	maxParentDepth = 3
	// maxAutoAddedPerDeclaration caps the objects added automatically to one
	// declaration, whatever the number of explicit additions.
	maxAutoAddedPerDeclaration = 50
)

// identifyingFieldSemantics are the field semantics that make a table worth
// putting in front of the model even when its table semantic is not an entity.
var identifyingFieldSemantics = map[models.FieldSemanticType]bool{
	models.FieldSemanticTypeIban:               true,
	models.FieldSemanticTypeAccountNumber:      true,
	models.FieldSemanticTypeBic:                true,
	models.FieldSemanticTypeRegistrationNumber: true,
	models.FieldSemanticTypeTaxId:              true,
	models.FieldSemanticTypeAddress:            true,
	models.FieldSemanticTypeName:               true,
	models.FieldSemanticTypeFirstName:          true,
	models.FieldSemanticTypeLastName:           true,
	models.FieldSemanticTypeDateOfBirth:        true,
	models.FieldSemanticTypeEmail:              true,
	models.FieldSemanticTypePhoneNumber:        true,
	models.FieldSemanticTypeCountry:            true,
}

// isUsefulParentTable is the single rule deciding whether an object reached as
// a parent is added to the selection automatically. It only looks at the data
// model semantics (nothing format-specific): the table is a person, a company,
// an account or a partner, or it has at least one non-archived field with an
// identifying semantic (iban, account number, bic, registration number, tax
// id, address, name, first/last name, date of birth, email, phone number,
// country). Tables with none of these, or with unset semantics, are not
// useful: the rule is deliberately conservative.
func isUsefulParentTable(t models.Table) bool {
	switch t.SemanticType {
	case models.SemanticTypePerson, models.SemanticTypeCompany,
		models.SemanticTypeAccount, models.SemanticTypePartner:
		return true
	}
	for _, f := range t.Fields {
		if !f.Archived && identifyingFieldSemantics[f.SemanticType] {
			return true
		}
	}
	return false
}

// maxParentReads bounds the object reads of one selection change (the cache
// absorbs repeated parents; this bounds a wide selection through a deep model).
const maxParentReads = 500

// parentNode is an object of the traversal: its data is what links are
// followed from, root the nearest selected object it was reached from (itself
// when it is selected).
type parentNode struct {
	ref  core.ObjectRef
	root core.ObjectRef
	data map[string]any
}

type parentRead struct {
	ref   core.ObjectRef
	obj   models.ClientObjectDetail
	found bool
}

// autoParents returns the parent objects to add automatically for the seeds
// (the objects the analyst just chose, or turned from auto-added into a choice):
// objects reached through to-single links of the data model (never to-many),
// transitively up to maxParentDepth hops, cycle-safe and deduplicated, at most
// maxAutoAddedPerDeclaration auto-added objects in the declaration, in a
// deterministic order (breadth-first, then table and object id). Only useful
// parents (isUsefulParentTable) are added; the traversal may pass through a
// table that is not useful (it counts as a hop). Objects the analyst excluded
// are neither added nor traversed, objects already selected are traversed but
// not added again. Every read goes through the organization of the declaration
// and at most maxParentReads reads are made; a parent that does not exist is
// skipped.
//
// An added parent records, as ReachedFrom, the nearest selected object it was
// reached from (the seed, or the selected parent before a non-useful table) and
// the last link followed. A parent that is also a direct candidate of the
// declaration (decision trigger, pivot, case entity) is added with the same
// snapshot as an analyst choice, still marked auto-added for display but not
// context-only: it can be declared. Any other parent is context only.
//
// selected holds the selection after the current change, seeds included; the
// caller owns it and does not see the returned objects in it.
func (uc *Usecase) autoParents(ctx context.Context, orgId uuid.UUID, c selectionContext,
	declarationId uuid.UUID, seeds []models.SarSelectedObject,
	selected map[core.ObjectRef]models.SarSelectedObject, excluded map[core.ObjectRef]bool,
) ([]models.SarSelectedObject, error) {
	dm := c.dataModel
	autoCount := 0
	for _, o := range selected {
		if o.AutoAdded {
			autoCount++
		}
	}

	frontier := make([]parentNode, 0, len(seeds))
	for _, s := range seeds {
		frontier = append(frontier, parentNode{ref: rowRef(s), root: rowRef(s), data: s.Snapshot})
	}
	sortNodes(frontier)

	visited := make(map[core.ObjectRef]bool, len(frontier))
	for _, n := range frontier {
		visited[n.ref] = true
	}
	cache := map[string]parentRead{}
	reads := 0
	var out []models.SarSelectedObject

	for depth := 1; depth <= maxParentDepth && len(frontier) > 0; depth++ {
		type found struct {
			node parentNode
			obj  models.ClientObjectDetail
			from models.SarReachedFrom
		}
		level := map[core.ObjectRef]found{}
		for _, n := range frontier {
			for _, l := range linksOf(dm, n.ref.Table) {
				if l.single == nil {
					continue // never to-many links
				}
				if _, known := dm.Tables[l.single.ParentTableName]; !known {
					continue
				}
				read, err := uc.readParent(ctx, orgId, dm, cache, &reads, n.data, l)
				if err != nil {
					return nil, err
				}
				if !read.found || visited[read.ref] || excluded[read.ref] {
					continue
				}
				if _, dup := level[read.ref]; dup {
					continue
				}
				level[read.ref] = found{
					node: parentNode{ref: read.ref, root: n.root, data: read.obj.Data},
					obj:  read.obj,
					from: models.SarReachedFrom{TableName: n.root.Table, ObjectId: n.root.ObjectID, Link: l.Name},
				}
			}
		}
		refs := make([]core.ObjectRef, 0, len(level))
		for r := range level {
			refs = append(refs, r)
		}
		slices.SortFunc(refs, compareRefs)

		next := make([]parentNode, 0, len(refs))
		for _, r := range refs {
			f := level[r]
			visited[r] = true
			if sel, isSelected := selected[r]; isSelected {
				if sel.Snapshot != nil { // the frozen copy wins
					f.node.data = sel.Snapshot
				}
				f.node.root = r
				next = append(next, f.node)
				continue
			}
			if !isUsefulParentTable(dm.Tables[r.Table]) {
				next = append(next, f.node) // traversed, not added, keeps its root
				continue
			}
			if autoCount >= maxAutoAddedPerDeclaration {
				return out, nil
			}
			from := f.from
			row := models.SarSelectedObject{
				Id:            pure_utils.NewId(),
				DeclarationId: declarationId,
				TableName:     r.Table,
				ObjectId:      r.ObjectID,
				Snapshot:      utcTimes(f.obj.Data),
				Origin:        ingestedOrigin(f.obj),
				ReachedFrom:   &from,
				AutoAdded:     true,
				ContextOnly:   true,
			}
			if _, direct := c.candidates[r]; direct {
				// Same snapshot as an analyst choice (a trigger object comes from
				// its decision).
				snap, err := uc.snapshotFor(ctx, orgId, c, SelectionAdd{Ref: r})
				if err != nil {
					return nil, err
				}
				row.Snapshot, row.Origin, row.ContextOnly = snap.Snapshot, snap.Origin, false
				f.node.data = row.Snapshot
			}
			out = append(out, row)
			autoCount++
			f.node.root = r
			next = append(next, f.node)
		}
		frontier = next
	}
	return out, nil
}

// readParent reads the parent reached from data through the to-single link l.
// Reads are cached for the duration of one selection change. A missing parent
// is not an error.
func (uc *Usecase) readParent(ctx context.Context, orgId uuid.UUID, dm models.DataModel,
	cache map[string]parentRead, reads *int, data map[string]any, l linkInfo,
) (parentRead, error) {
	value, ok := valueString(data[l.single.ChildFieldName])
	if !ok {
		return parentRead{}, nil
	}
	key := l.single.ParentTableName + "\x00" + l.single.ParentFieldName + "\x00" + value
	if r, hit := cache[key]; hit {
		return r, nil
	}
	if *reads >= maxParentReads {
		return parentRead{}, nil // read budget spent: nothing more is followed
	}
	*reads++
	objs, err := uc.ingestedReader.GetIngestedObject(ctx, orgId, &dm,
		l.single.ParentTableName, value, l.single.ParentFieldName)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return parentRead{}, err
	}
	var r parentRead
	if err == nil && len(objs) > 0 {
		obj := objs[len(objs)-1]
		if id, ok := objectIdOf(obj.Data); ok {
			r = parentRead{ref: core.ObjectRef{Table: l.single.ParentTableName, ObjectID: id}, obj: obj, found: true}
		}
	}
	cache[key] = r
	return r, nil
}

// orphanedAutoAdded returns the auto-added objects of selected that are no
// longer reachable from an analyst's choice (a row that is not auto-added): an
// auto-added object stays while it was reached from a live object (its
// ReachedFrom) or a live object leads to it through a to-single link (compared
// on the frozen copies). Liveness is computed from the analyst's rows, so a
// cycle of parents never keeps itself alive and a parent reached through a
// table that was not added is not lost. The order is deterministic.
func orphanedAutoAdded(dm models.DataModel, selected map[core.ObjectRef]models.SarSelectedObject) []models.SarSelectedObject {
	alive := map[core.ObjectRef]bool{}
	var pending []models.SarSelectedObject
	for r, o := range selected {
		if o.AutoAdded {
			pending = append(pending, o)
		} else {
			alive[r] = true
		}
	}
	for changed := true; changed; {
		changed = false
		rest := pending[:0:0]
		for _, p := range pending {
			if reachedFromAlive(dm, selected, alive, p) {
				alive[rowRef(p)] = true
				changed = true
			} else {
				rest = append(rest, p)
			}
		}
		pending = rest
	}
	slices.SortFunc(pending, func(a, b models.SarSelectedObject) int { return compareRefs(rowRef(a), rowRef(b)) })
	return pending
}

// reachedFromAlive tells whether a live object is the recorded origin of p or
// leads to p through a to-single link.
func reachedFromAlive(dm models.DataModel, selected map[core.ObjectRef]models.SarSelectedObject,
	alive map[core.ObjectRef]bool, p models.SarSelectedObject,
) bool {
	if rf := p.ReachedFrom; rf != nil && alive[core.ObjectRef{Table: rf.TableName, ObjectID: rf.ObjectId}] {
		return true
	}
	for r := range alive {
		from := selected[r]
		if r == rowRef(p) {
			continue
		}
		for _, l := range linksOf(dm, from.TableName) {
			if l.single != nil && l.single.ParentTableName == p.TableName && l.reaches(from.Snapshot, p.Snapshot) {
				return true
			}
		}
	}
	return false
}

func compareRefs(a, b core.ObjectRef) int {
	if c := strings.Compare(a.Table, b.Table); c != 0 {
		return c
	}
	return strings.Compare(a.ObjectID, b.ObjectID)
}

func sortNodes(nodes []parentNode) {
	slices.SortFunc(nodes, func(a, b parentNode) int { return compareRefs(a.ref, b.ref) })
}
