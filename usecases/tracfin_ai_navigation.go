package usecases

import (
	"context"
	"sort"

	"github.com/checkmarble/marble-backend/models"
)

// collectAINavigation reads direct links and configured mapping paths, bounded by
// the shared record budget. The path is preserved independently of object identity.
func (uc TracfinUsecase) collectAINavigation(ctx context.Context, c models.Case, config models.TracfinConfiguration, triggers []models.CaseEntityRef, add func(string, string, string, any), fail func(string, string, string)) {
	dm, err := uc.dataModelUsecase.GetDataModel(ctx, uc.orgID, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		fail("navigation", "DATA_MODEL_UNAVAILABLE", err.Error())
		return
	}
	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, uc.orgID)
	if err != nil {
		fail("navigation", "CLIENT_DB_UNAVAILABLE", err.Error())
		return
	}
	pivots, err := uc.cases.ReadCasePivotObjects(ctx, c.Id)
	if err != nil {
		fail("navigation", "PIVOTS_UNAVAILABLE", err.Error())
		return
	}
	type node struct {
		table, id, path string
		object          models.DataModelObject
	}
	var roots []node
	seenRoot := map[string]bool{}
	addRoot := func(table, id string) {
		key := table + ":" + id
		if id == "" || seenRoot[key] {
			return
		}
		seenRoot[key] = true
		t, ok := dm.Tables[table]
		if !ok {
			fail(key, "ROOT_TABLE_UNAVAILABLE", "root table unavailable")
			return
		}
		rows, e := uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, t, id, "object_id", "valid_from")
		if e != nil || len(rows) != 1 {
			fail(key, "ROOT_UNAVAILABLE", "root object unavailable")
			return
		}
		roots = append(roots, node{table, id, key, rows[0]})
	}
	for _, trigger := range triggers {
		addRoot(trigger.TableName, trigger.ObjectId)
	}
	for _, e := range c.Entities {
		addRoot(e.TableName, e.ObjectId)
	}
	for _, p := range pivots {
		if p.IsIngested && p.PivotType == models.PivotTypeObject {
			addRoot(p.PivotObjectName, p.PivotObjectId)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].path < roots[j].path })
	seen := map[string]bool{}
	pathsSeen := map[string]bool{}
	records := 0
	visits := 0
	save := func(n node) bool {
		if pathsSeen[n.path] {
			return true
		}
		if visits >= aiMaxRecords {
			fail("navigation", "RECORD_LIMIT", "10,000 path limit reached")
			return false
		}
		visits++
		pathsSeen[n.path] = true
		key := n.table + ":" + n.id
		if !seen[key] {
			if records >= aiMaxRecords {
				fail("navigation", "RECORD_LIMIT", "10,000 record limit reached")
				return false
			}
			records++
			seen[key] = true
			add("object:"+key, "ingested_object", key, struct {
				Table  string                 `json:"table"`
				ID     string                 `json:"id"`
				Object models.DataModelObject `json:"object"`
			}{n.table, n.id, n.object})
		}
		add("path:"+n.path, "object_path", n.path, struct{ Table, ID, Path string }{n.table, n.id, n.path})
		return true
	}
	for _, r := range roots {
		save(r)
	}
	walk := func(parent node, step models.TracfinPathStep) []node {
		source, filter, nav, e := linkFields(dm, parent.table, step)
		if e != nil {
			fail(parent.path, "LINK_UNAVAILABLE", e.Error())
			return nil
		}
		value, ok := parent.object.Data[source].(string)
		if !ok || value == "" {
			return nil
		}
		var result []node
		var cursor *string
		for {
			var rows []models.DataModelObject
			if nav == nil {
				rows, e = uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, dm.Tables[step.Table], value, filter, "valid_from")
				if len(rows) > 1 {
					fail(parent.path, "LINK_NOT_UNIQUE", "forward link returned multiple records")
					return nil
				}
			} else {
				rows, e = uc.objects.ListIngestedObjects(ctx, exec, dm.Tables[step.Table], models.ExplorationOptions{SourceTableName: parent.table, FilterFieldName: filter, FilterFieldValue: models.NewStringOrNumberFromString(value), OrderingFieldName: nav.OrderingFieldName}, cursor, 101)
			}
			if e != nil {
				fail(parent.path, "NAVIGATION_FAILED", e.Error())
				return nil
			}
			more := nav != nil && len(rows) > 100
			if more {
				rows = rows[:100]
			}
			for _, row := range rows {
				id, ok := row.Data["object_id"].(string)
				if !ok || id == "" {
					fail(parent.path, "OBJECT_ID_MISSING", "linked record has no object_id")
					continue
				}
				child := node{step.Table, id, parent.path + "/" + step.ID + ":" + step.Table + ":" + id, row}
				if !save(child) {
					return result
				}
				result = append(result, child)
			}
			if !more {
				break
			}
			next, ok := rows[len(rows)-1].Data["object_id"].(string)
			if !ok || next == "" || (cursor != nil && *cursor == next) {
				fail(parent.path, "CURSOR_INVALID", "navigation cursor did not advance")
				break
			}
			cursor = &next
		}
		return result
	}
	for _, r := range roots {
		table := dm.Tables[r.table]
		keys := make([]string, 0, len(table.LinksToSingle))
		for key := range table.LinksToSingle {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			link := table.LinksToSingle[key]
			walk(r, models.TracfinPathStep{ID: "direct-forward-" + key, Link: key, Direction: "forward", Table: link.ParentTableName})
		}
		// Inverse links are matched to configured navigation options, preserving all distinct paths.
		targetNames := make([]string, 0, len(dm.Tables))
		for name := range dm.Tables {
			targetNames = append(targetNames, name)
		}
		sort.Strings(targetNames)
		for _, name := range targetNames {
			target := dm.Tables[name]
			keys = keys[:0]
			for key := range target.LinksToSingle {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				link := target.LinksToSingle[key]
				if link.ParentTableName != r.table {
					continue
				}
				walk(r, models.TracfinPathStep{ID: "direct-inverse-" + name + "-" + key, Link: key, Direction: "inverse", Table: name})
			}
		}
		for _, m := range config.Mappings {
			if m.Source.Kind != "ingested" || m.Source.Table != r.table {
				continue
			}
			frontier := []node{r}
			for _, step := range m.Source.Path {
				var next []node
				for _, parent := range frontier {
					next = append(next, walk(parent, step)...)
				}
				frontier = next
				if len(frontier) == 0 {
					break
				}
			}
		}
	}
}
