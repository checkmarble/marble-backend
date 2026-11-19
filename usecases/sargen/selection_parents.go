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

// parentNode is an object of the traversal: its data is what links are
// followed from.
type parentNode struct {
	ref  core.ObjectRef
	data map[string]any
}

type parentRead struct {
	ref   core.ObjectRef
	obj   models.ClientObjectDetail
	found bool
}

// autoParents returns the parent objects to add automatically for the newly
// selected seeds: objects reached through to-single links of the data model
// (never to-many), transitively up to maxParentDepth hops, cycle-safe and
// deduplicated, at most maxAutoAddedPerDeclaration auto-added objects in the
// declaration, in a deterministic order (breadth-first, then table and object
// id). Only useful parents (isUsefulParentTable) are added; the traversal may
// pass through a table that is not useful (it counts as a hop). Objects the
// analyst excluded are neither added nor traversed, objects already selected
// are traversed but not added again. Every read goes through the organization
// of the declaration; a parent that does not exist is skipped.
//
// selected holds the selection after the current change, seeds included; the
// caller owns it and does not see the returned objects in it.
func (uc *Usecase) autoParents(ctx context.Context, orgId uuid.UUID, dm models.DataModel,
	declarationId uuid.UUID, seeds []models.SarSelectedObject,
	selected map[core.ObjectRef]models.SarSelectedObject, excluded map[core.ObjectRef]bool,
) ([]models.SarSelectedObject, error) {
	autoCount := 0
	for _, o := range selected {
		if o.AutoAdded {
			autoCount++
		}
	}

	frontier := make([]parentNode, 0, len(seeds))
	for _, s := range seeds {
		frontier = append(frontier, parentNode{ref: rowRef(s), data: s.Snapshot})
	}
	sortNodes(frontier)

	visited := make(map[core.ObjectRef]bool, len(frontier))
	for _, n := range frontier {
		visited[n.ref] = true
	}
	cache := map[string]parentRead{}
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
				read, err := uc.readParent(ctx, orgId, dm, cache, n.data, l)
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
					node: parentNode{ref: read.ref, data: read.obj.Data},
					obj:  read.obj,
					from: models.SarReachedFrom{TableName: n.ref.Table, ObjectId: n.ref.ObjectID, Link: l.Name},
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
				next = append(next, f.node)
				continue
			}
			if !isUsefulParentTable(dm.Tables[r.Table]) {
				next = append(next, f.node) // traversed, not added
				continue
			}
			if autoCount >= maxAutoAddedPerDeclaration {
				return out, nil
			}
			from := f.from
			out = append(out, models.SarSelectedObject{
				Id:            pure_utils.NewId(),
				DeclarationId: declarationId,
				TableName:     r.Table,
				ObjectId:      r.ObjectID,
				Snapshot:      utcTimes(f.obj.Data),
				Origin:        ingestedOrigin(f.obj),
				ReachedFrom:   &from,
				AutoAdded:     true,
			})
			autoCount++
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
	cache map[string]parentRead, data map[string]any, l linkInfo,
) (parentRead, error) {
	value, ok := valueString(data[l.single.ChildFieldName])
	if !ok {
		return parentRead{}, nil
	}
	key := l.single.ParentTableName + "\x00" + l.single.ParentFieldName + "\x00" + value
	if r, hit := cache[key]; hit {
		return r, nil
	}
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

// orphanedAutoAdded returns the auto-added objects of selected that no other
// selected object leads to through a to-single link, repeating until nothing
// more is orphaned (removing a child may orphan its own parents). The order is
// deterministic.
func orphanedAutoAdded(dm models.DataModel, selected map[core.ObjectRef]models.SarSelectedObject) []models.SarSelectedObject {
	alive := make(map[core.ObjectRef]models.SarSelectedObject, len(selected))
	for r, o := range selected {
		alive[r] = o
	}
	var orphans []models.SarSelectedObject
	for {
		changed := false
		refs := make([]core.ObjectRef, 0, len(alive))
		for r := range alive {
			refs = append(refs, r)
		}
		slices.SortFunc(refs, compareRefs)
		for _, r := range refs {
			p := alive[r]
			if !p.AutoAdded || ledTo(dm, alive, p) {
				continue
			}
			orphans = append(orphans, p)
			delete(alive, r)
			changed = true
		}
		if !changed {
			return orphans
		}
	}
}

// ledTo tells whether an object of selected other than p leads to p through a
// to-single link (compared on the frozen copies).
func ledTo(dm models.DataModel, selected map[core.ObjectRef]models.SarSelectedObject, p models.SarSelectedObject) bool {
	for r, from := range selected {
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
