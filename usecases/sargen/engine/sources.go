package engine

import (
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// AllowedSources builds core.ApplyOptions.AllowedSources: a fill on target may
// cite the ItemSources of every ancestor item of target (the item itself
// included) and the selected objects linked to them by a data-model link (one
// hop, either direction). Items still to be created (`[+x]` keys) carry no
// ItemSources of their own: when no ancestor item has any, the task inputs
// (fallback) are the allowed objects. The doc is read at call time, so the
// function follows the document being filled. Used by the agent runner (dry
// run) and by the orchestrator (final apply), so both refuse the same sources.
func AllowedSources(doc *core.Document, selection []models.SarSelectedObject, dm models.DataModel,
	fallback []core.ObjectRef,
) func(target core.Path) []core.ObjectRef {
	sc := newScope(selection)
	return func(target core.Path) []core.ObjectRef {
		var base []core.ObjectRef
		for _, item := range ancestorItems(target) {
			base = append(base, doc.ItemSources[item]...)
		}
		if len(base) == 0 {
			base = slices.Clone(fallback)
		}
		out := slices.Clone(base)
		for _, ref := range base {
			for _, o := range linkedObjects(sc, dm, ref) {
				out = append(out, core.ObjectRef{Table: o.TableName, ObjectID: o.ObjectId})
			}
		}
		slices.SortFunc(out, func(a, b core.ObjectRef) int {
			if c := strings.Compare(a.Table, b.Table); c != 0 {
				return c
			}
			return strings.Compare(a.ObjectID, b.ObjectID)
		})
		return slices.Compact(out)
	}
}

// ancestorItems returns the item prefixes of p (segments ending with a key).
func ancestorItems(p core.Path) []core.Path {
	segs := strings.Split(string(p), ".")
	var out []core.Path
	for i, s := range segs {
		if strings.HasSuffix(s, "]") {
			out = append(out, core.Path(strings.Join(segs[:i+1], ".")))
		}
	}
	return out
}

// linkedObjects returns the selected objects linked to ref by a data-model
// link: the parent of a belongs-to link held by ref, the selected children
// pointing at ref.
func linkedObjects(sc scope, dm models.DataModel, ref core.ObjectRef) []models.SarSelectedObject {
	src, ok := sc.get(ref.Table, ref.ObjectID)
	if !ok {
		return nil
	}
	var out []models.SarSelectedObject
	for _, table := range dm.Tables {
		for _, link := range table.LinksToSingle {
			if link.ChildTableName == src.TableName {
				if value, has := valueString(src.Snapshot[link.ChildFieldName]); has {
					if parent, sel := findByField(sc, link.ParentTableName, link.ParentFieldName, value); sel {
						out = append(out, parent)
					}
				}
			}
			if link.ParentTableName == src.TableName {
				value, has := valueString(src.Snapshot[link.ParentFieldName])
				if link.ParentFieldName == "object_id" {
					value, has = src.ObjectId, true
				}
				if !has {
					continue
				}
				for _, child := range sc.byTable[link.ChildTableName] {
					if got, ok := valueString(child.Snapshot[link.ChildFieldName]); ok && got == value {
						out = append(out, child)
					}
				}
			}
		}
	}
	return out
}

// sourcesOutsideSelection lists the ObjectRefs that a structured output added to
// the item sources of the document (after versus before) and that are not in
// the frozen selection.
func sourcesOutsideSelection(before, after *core.Document, sc scope) []core.ObjectRef {
	var bad []core.ObjectRef
	for item, refs := range after.ItemSources {
		old := before.ItemSources[item]
		for _, ref := range refs {
			if slices.Contains(old, ref) {
				continue
			}
			if _, ok := sc.get(ref.Table, ref.ObjectID); !ok && !slices.Contains(bad, ref) {
				bad = append(bad, ref)
			}
		}
	}
	slices.SortFunc(bad, func(a, b core.ObjectRef) int {
		if c := strings.Compare(a.Table, b.Table); c != 0 {
			return c
		}
		return strings.Compare(a.ObjectID, b.ObjectID)
	})
	return bad
}

// outsideSelectionMessage is the message (French) of a structured output that
// cites objects outside the selection.
func outsideSelectionMessage(bad []core.ObjectRef) string {
	names := make([]string, len(bad))
	for i, r := range bad {
		names[i] = r.Table + "/" + r.ObjectID
	}
	return "objet hors sélection : " + strings.Join(names, ", ")
}
