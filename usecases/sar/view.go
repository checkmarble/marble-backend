package sar

import (
	"sort"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

// ExtView is one extension field instance with its editing contract.
type ExtView struct {
	Field      ExtField              `json:"field"`
	Key        string                `json:"key"`                 // field or field#object
	ObjectID   string                `json:"object_id,omitempty"` // pivot object of a per-object field
	Edit       string                `json:"edit"`                // extension/<format>/<key>, or settings/<format>/<key>
	Values     []string              `json:"values"`
	Display    []string              `json:"display,omitempty"`
	Provenance *models.SarProvenance `json:"provenance,omitempty"`
	Editable   bool                  `json:"editable"`
	Issues     []models.SarIssue     `json:"issues,omitempty"`
}

type IssueSummary struct {
	Errors   int            `json:"errors"`
	Warnings int            `json:"warnings"`
	ByLayer  map[string]int `json:"by_layer"`
	Missing  int            `json:"missing"` // required elements absent from the tree
}

// ReportView is the JSON language served to the frontend for one report.
type ReportView struct {
	Report     models.SarReport            `json:"report"`
	Format     Descriptor                  `json:"format"`
	Documents  []format.ViewNode           `json:"documents"`
	Extension  []ExtView                   `json:"extension"`
	Settings   []ExtView                   `json:"settings"`
	Issues     []models.SarIssue           `json:"issues"`
	Summary    IssueSummary                `json:"summary"`
	CodeLists  map[string]*format.CodeList `json:"code_lists"`
	Exportable bool                        `json:"exportable"`
}

// BuildView evaluates and represents a report.
func BuildView(a Adapter, in Input, rep models.SarReport) ReportView {
	ev := Evaluate(a, in)
	in.Ext = ev.Ext
	rep.Extension = ev.Ext
	rep.ReportType = ev.ReportType
	rep.Issues = ev.Issues
	key := a.Descriptor().Key
	view := ReportView{Report: rep, Format: a.Descriptor(), Issues: ev.Issues, Summary: IssueSummary{ByLayer: map[string]int{}}}
	byDoc := map[int][]models.SarIssue{}
	for _, i := range ev.Issues {
		if i.Severity == "error" {
			view.Summary.Errors++
		} else {
			view.Summary.Warnings++
		}
		view.Summary.ByLayer[i.Layer]++
		d := i.Document
		if d == 0 {
			d = 1
		}
		byDoc[d] = append(byDoc[d], i)
	}
	for i, d := range ev.Docs {
		v := format.Represent(a.Schema(), d, byDoc[i+1], a.Hint)
		countMissing(v, &view.Summary.Missing)
		view.Documents = append(view.Documents, v)
	}
	view.CodeLists = format.UsedCodeLists(a.Schema(), view.Documents...)
	issuesByEdit := map[string][]models.SarIssue{}
	for _, i := range ev.Issues {
		if strings.HasPrefix(i.Edit, "extension/") || strings.HasPrefix(i.Edit, "settings/") {
			issuesByEdit[i.Edit] = append(issuesByEdit[i.Edit], i)
		}
	}
	s := a.Schema()
	objectIDs := func(scope string) []string {
		var ids []string
		switch scope {
		case "operation":
			for _, op := range SelectedOperations(in.Pivot) {
				ids = append(ids, op.ID)
			}
		case "party":
			for _, p := range in.Pivot.Parties {
				ids = append(ids, p.ID)
			}
		case "instrument":
			for _, x := range in.Pivot.Instruments {
				ids = append(ids, x.ID)
			}
		case "attachment":
			for _, x := range SelectedAttachments(in.Pivot) {
				ids = append(ids, x.ID)
			}
		}
		return ids
	}
	for _, f := range a.Extension() {
		if f.CodeList != "" {
			if l := s.CodeLists[f.CodeList]; l != nil {
				view.CodeLists[f.CodeList] = l
			}
		}
		instances := []string{""}
		if f.Scope != "report" && f.Scope != "settings" {
			instances = objectIDs(f.Scope)
		}
		for _, obj := range instances {
			k := ExtKey(f.Key, obj)
			ev := ExtView{Field: f, Key: k, ObjectID: obj, Editable: !f.Computed}
			if f.Scope == "settings" {
				ev.Edit = "settings/" + key + "/" + f.Key
				ev.Values = in.Settings[f.Key]
				ev.Provenance = &models.SarProvenance{Kind: models.SarProvSettings}
			} else {
				ev.Edit = "extension/" + key + "/" + k
				ev.Values = in.Ext[k].Values
				ev.Provenance = in.Ext[k].P
			}
			for _, val := range ev.Values {
				if f.CodeList != "" {
					ev.Display = append(ev.Display, s.CodeLabel(f.CodeList, val))
				}
			}
			ev.Issues = issuesByEdit[ev.Edit]
			if f.Scope == "settings" {
				view.Settings = append(view.Settings, ev)
			} else {
				view.Extension = append(view.Extension, ev)
			}
		}
	}
	sort.SliceStable(view.Extension, func(i, j int) bool {
		return view.Extension[i].Field.Scope == "report" && view.Extension[j].Field.Scope != "report"
	})
	view.Exportable = view.Summary.Errors == 0
	return view
}

func countMissing(v format.ViewNode, n *int) {
	if v.Missing {
		*n++
	}
	for _, c := range v.Children {
		countMissing(c, n)
	}
}
