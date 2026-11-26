//go:build sargen_playground

package main

import (
	_ "embed"
	"encoding/json"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/cockroachdb/errors"
)

// Role is who writes. It fixes the origin and the overwrite policy, as the
// usecases of the stack do for each writer.
type Role string

const (
	RoleLLM        Role = "llm"
	RoleAnalyst    Role = "analyst"
	RoleComputed   Role = "computed"
	RoleOrgSetting Role = "org_setting"
)

// writer returns the origin and the options a role writes with.
func writer(role Role, allowedPrefixes []string) (core.Origin, core.ApplyOptions, error) {
	switch role {
	case RoleLLM:
		prefixes := make([]core.Path, len(allowedPrefixes))
		for i, p := range allowedPrefixes {
			prefixes[i] = core.Path(p)
		}
		return core.OriginLLM, core.ApplyOptions{AllowedPrefixes: prefixes, Overwrite: core.OverwriteNonManual}, nil
	case RoleAnalyst:
		return core.OriginManual, core.ApplyOptions{Overwrite: core.OverwriteAll}, nil
	case RoleComputed:
		return core.OriginComputed, core.ApplyOptions{Overwrite: core.OverwriteNonManual, SkipMaxItems: true}, nil
	case RoleOrgSetting:
		return core.OriginOrgSetting, core.ApplyOptions{Overwrite: core.OverwriteNonManual}, nil
	}
	return "", core.ApplyOptions{}, errors.Newf("unknown role %q", role)
}

// FillInput is one write as typed in the page or a scenario. Sources,
// justification and confidence are read for the LLM only; the computation for
// the computed role only.
type FillInput struct {
	Path          string            `json:"path"`
	Value         string            `json:"value"`
	Sources       []core.SourceRef  `json:"sources,omitempty"`
	Justification string            `json:"justification,omitempty"`
	Confidence    string            `json:"confidence,omitempty"`
	Computation   *core.Computation `json:"computation,omitempty"`
}

// Step is one action: a batch of fills written by a role in a single
// ApplyFills call, or (Review not empty) a MarkReviewed on paths.
type Step struct {
	Role            Role        `json:"role"`
	Fills           []FillInput `json:"fills,omitempty"`
	AllowedPrefixes []string    `json:"allowed_prefixes,omitempty"`
	Review          []string    `json:"review,omitempty"`
	// Expect is used by scenarios only: one outcome per fill ("applied",
	// "noop", "refreshed", "conflict", "rejected:<code>"), or, for a review
	// step, "reviewed" or "rejected:<code>" per path.
	Expect []string `json:"expect,omitempty"`
	Note   string   `json:"note,omitempty"`
}

// Outcome is what happened to one fill (or one reviewed path).
type Outcome struct {
	Path    string            `json:"path"`
	Value   string            `json:"value,omitempty"`
	Outcome string            `json:"outcome"` // applied, noop, refreshed, conflict, rejected, reviewed
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
	// RealPath is the path written, provisional keys resolved.
	RealPath string      `json:"real_path,omitempty"`
	Before   *core.Value `json:"before,omitempty"`
}

// Label is the short form a scenario expects.
func (o Outcome) Label() string {
	if o.Outcome == "rejected" {
		return "rejected:" + o.Code
	}
	return o.Outcome
}

// StepResult is the document after a step and what happened.
type StepResult struct {
	Doc      *core.Document    `json:"doc"`
	Outcomes []Outcome         `json:"outcomes"`
	Resolved map[string]string `json:"resolved,omitempty"`
	Changed  []string          `json:"changed"`
	Issues   []core.Issue      `json:"issues"`
}

// Playground holds the format; it keeps no document: the page sends the
// current document with each step.
type Playground struct {
	format core.Format
	spec   *core.Spec
}

func NewPlayground() (*Playground, error) {
	f, ok := tracfin.New(tracfin.CurrentVersion)
	if !ok {
		return nil, errors.Newf("TRACFIN %s not available", tracfin.CurrentVersion)
	}
	return &Playground{format: f, spec: f.Spec()}, nil
}

// Run applies a step to a copy of doc.
func (pg *Playground) Run(doc *core.Document, step Step) (StepResult, error) {
	doc = doc.Clone()
	before := snapshot(doc)
	var res StepResult
	if len(step.Review) > 0 {
		for _, p := range step.Review {
			o := Outcome{Path: p, Outcome: "reviewed"}
			if issues := core.MarkReviewed(pg.spec, doc, []core.Path{core.Path(p)}); len(issues) > 0 {
				o.Outcome, o.Code, o.Message, o.Params = "rejected", issues[0].Code, issues[0].Message, issues[0].Params
			}
			res.Outcomes = append(res.Outcomes, o)
		}
	} else {
		origin, opts, err := writer(step.Role, step.AllowedPrefixes)
		if err != nil {
			return StepResult{}, err
		}
		fills := make([]core.Fill, len(step.Fills))
		for i, in := range step.Fills {
			f := core.Fill{Path: core.Path(in.Path), Value: in.Value, Origin: origin}
			switch step.Role {
			case RoleLLM:
				f.Sources, f.Justification, f.Confidence = in.Sources, in.Justification, core.Confidence(in.Confidence)
			case RoleComputed:
				f.Computation = in.Computation
			}
			fills[i] = f
		}
		ar := core.ApplyFills(pg.spec, doc, fills, opts)
		res.Outcomes = outcomes(fills, ar)
		if len(ar.Resolved) > 0 {
			res.Resolved = map[string]string{}
			for k, v := range ar.Resolved {
				res.Resolved[string(k)] = string(v)
			}
		}
	}
	res.Doc = doc
	res.Changed = changed(before, snapshot(doc))
	res.Issues = pg.format.Validate(doc)
	if res.Issues == nil {
		res.Issues = []core.Issue{}
	}
	return res, nil
}

// outcomes matches each fill with its fate. ApplyFills reports every list in
// input order, so one cursor per list is enough.
func outcomes(fills []core.Fill, ar core.ApplyResult) []Outcome {
	var ri, ci, ai, fi int
	out := make([]Outcome, 0, len(fills))
	for _, f := range fills {
		o := Outcome{Path: string(f.Path), Value: f.Value}
		real := string(ar.Rewrite(f.Path))
		switch {
		case ri < len(ar.Rejected) && ar.Rejected[ri].Fill.Path == f.Path:
			is := ar.Rejected[ri].Issue
			o.Outcome, o.Code, o.Message, o.Params = "rejected", is.Code, is.Message, is.Params
			ri++
		case ci < len(ar.Conflicts) && ar.Conflicts[ci].After.Path == f.Path:
			b := ar.Conflicts[ci].Before
			o.Outcome, o.RealPath, o.Before = "conflict", string(ar.Conflicts[ci].Path), &b
			ci++
		case ai < len(ar.Applied) && string(ar.Applied[ai]) == real:
			o.Outcome, o.RealPath = "applied", real
			ai++
		case fi < len(ar.Refreshed) && string(ar.Refreshed[fi]) == real:
			o.Outcome, o.RealPath = "refreshed", real
			fi++
		default:
			o.Outcome, o.RealPath = "noop", real
		}
		out = append(out, o)
	}
	return out
}

// snapshot flattens a document into comparable entries: one per value, one per
// item.
func snapshot(doc *core.Document) map[string]string {
	out := map[string]string{}
	for p, v := range doc.Values {
		b, _ := json.Marshal(v)
		out[string(p)] = string(b)
	}
	for _, c := range doc.ItemContainers() {
		for _, k := range doc.ItemKeys(c) {
			out[string(c)+"["+k+"]"] = "item"
		}
	}
	return out
}

func changed(before, after map[string]string) []string {
	out := []string{}
	for k, v := range after {
		if before[k] != v {
			out = append(out, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// generatedDeclaration is a valid TRACFIN declaration with the provenance a
// generation gives: settings from the organization, computed fields with their
// computation, the LLM's values with sources (and a justification for
// judgments), the analysis typed by the analyst; nothing reviewed yet.
//
//go:embed starts/generated.json
var generatedDeclaration []byte

// Start returns a starting document: "empty", or "generated" (a declaration
// as a generation leaves it).
func (pg *Playground) Start(name string) (*core.Document, error) {
	switch name {
	case "", "empty":
		return core.NewDocument(), nil
	case "generated":
		doc := core.NewDocument()
		if err := json.Unmarshal(generatedDeclaration, doc); err != nil {
			return nil, errors.Wrap(err, "generated.json")
		}
		return doc, nil
	}
	return nil, errors.Newf("unknown start document %q", name)
}

// FieldInfo is what the field picker shows for one node of the spec.
type FieldInfo struct {
	Template    string   `json:"template"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Nature      string   `json:"nature"`
	Type        string   `json:"type,omitempty"`
	Min         int      `json:"min"`
	Max         int      `json:"max"`
	MinLength   int      `json:"min_length,omitempty"`
	MaxLength   int      `json:"max_length,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	CodeList    string   `json:"code_list,omitempty"`
	CodeOrText  bool     `json:"code_or_text,omitempty"`
	RefTargets  []string `json:"ref_targets,omitempty"`
	RefOrText   bool     `json:"ref_or_text,omitempty"`
	ExportID    bool     `json:"export_id,omitempty"`
	Repeatable  bool     `json:"repeatable,omitempty"`
	Working     bool     `json:"working,omitempty"`
}

type SpecInfo struct {
	Version   string                   `json:"version"`
	Fields    []FieldInfo              `json:"fields"`
	CodeLists map[string]core.CodeList `json:"code_lists"`
}

// Describe flattens the spec for the page. A node without label takes the
// nearest labelled ancestor's, followed by its own name.
func (pg *Playground) Describe() SpecInfo {
	info := SpecInfo{Version: tracfin.CurrentVersion, CodeLists: pg.spec.CodeLists}
	var walk func(n *core.Node, parentLabel string)
	walk = func(n *core.Node, parentLabel string) {
		label := n.Label
		if label == "" {
			label = strings.TrimSpace(strings.TrimPrefix(parentLabel+" › "+n.Name, " › "))
		}
		if n.Template != "" {
			info.Fields = append(info.Fields, FieldInfo{
				Template: n.Template, Kind: string(n.Kind), Label: label, Description: n.Description,
				Nature: string(n.EffectiveNature()), Type: string(n.Type), Min: n.Min, Max: n.Max,
				MinLength: n.MinLength, MaxLength: n.MaxLength, Pattern: n.Pattern,
				CodeList: n.CodeList, CodeOrText: n.CodeOrText, RefTargets: n.RefTargets, RefOrText: n.RefOrText,
				ExportID: n.ExportID, Repeatable: n.IsRepeatable(), Working: !n.Export,
			})
		}
		for _, c := range n.Children {
			walk(c, label)
		}
	}
	walk(pg.spec.Root, "")
	return info
}
