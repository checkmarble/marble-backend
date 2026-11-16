package core

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
)

type Origin string

const (
	OriginDataModel  Origin = "data_model"
	OriginLLM        Origin = "llm"
	OriginMemory     Origin = "memory"
	OriginOrgSetting Origin = "org_setting"
	OriginComputed   Origin = "computed"
	OriginManual     Origin = "manual"
	OriginAnalyst    Origin = "analyst"
)

type Confidence string

const (
	ConfidenceNone   Confidence = ""
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type ObjectRef struct {
	Table    string `json:"table"`
	ObjectID string `json:"object_id"`
}

type SourceRef struct {
	Object   ObjectRef `json:"object"`
	Field    string    `json:"field"`
	RawValue string    `json:"raw_value"`
}

type Computation struct {
	Name   string `json:"name"`
	Inputs []Path `json:"inputs"`
}

type Value struct {
	Value         string       `json:"value"`
	Origin        Origin       `json:"origin"`
	Sources       []SourceRef  `json:"sources,omitempty"`
	RuleIDs       []string     `json:"rule_ids,omitempty"` //nolint:tagliatelle
	Justification string       `json:"justification,omitempty"`
	Confidence    Confidence   `json:"confidence,omitempty"`
	Reviewed      bool         `json:"reviewed"`
	Computation   *Computation `json:"computation,omitempty"`
	TaskID        *uuid.UUID   `json:"task_id,omitempty"`
}

// Document is the pivot instance of a declaration. Items maps a repeatable
// container path (without key segment, e.g. `personnes.pp`) to its ordered
// item keys; nested containers are keyed by their full concrete parent path.
type Document struct {
	Values      map[Path]Value       `json:"values"`
	Items       map[Path][]string    `json:"items"`
	ItemSources map[Path][]ObjectRef `json:"item_sources"`
	NextKey     map[Path]int         `json:"next_key"`
}

func NewDocument() *Document {
	return &Document{
		Values:      map[Path]Value{},
		Items:       map[Path][]string{},
		ItemSources: map[Path][]ObjectRef{},
		NextKey:     map[Path]int{},
	}
}

// AddItem appends a new item `prefix-N` to the container; N never repeats.
func (d *Document) AddItem(container Path, prefix string) Path {
	d.NextKey[container]++
	key := fmt.Sprintf("%s-%d", prefix, d.NextKey[container])
	d.Items[container] = append(d.Items[container], key)
	parent := container.Parent()
	name := strings.TrimPrefix(string(container), string(parent))
	name = strings.TrimPrefix(name, ".")
	return parent.Item(name, key)
}

// RemoveItem removes the item, everything below it, and its sources.
func (d *Document) RemoveItem(item Path) {
	s := string(item)
	if i := strings.LastIndex(s, "["); i >= 0 && strings.HasSuffix(s, "]") {
		key := s[i+1 : len(s)-1]
		name := s[:i]
		if j := strings.LastIndex(name, "."); j >= 0 {
			name = name[j+1:]
		}
		container := item.Parent().Child(name)
		d.Items[container] = slices.DeleteFunc(d.Items[container], func(k string) bool { return k == key })
		if len(d.Items[container]) == 0 {
			delete(d.Items, container)
		}
	}
	for p := range d.Values {
		if p.HasPrefix(item) {
			delete(d.Values, p)
		}
	}
	for p := range d.ItemSources {
		if p.HasPrefix(item) {
			delete(d.ItemSources, p)
		}
	}
	// Nested containers of the removed item (the container itself is keyed
	// by the parent path, so only strictly deeper paths match).
	for p := range d.Items {
		if p != item && p.HasPrefix(item) {
			delete(d.Items, p)
		}
	}
	for p := range d.NextKey {
		if p != item && p.HasPrefix(item) {
			delete(d.NextKey, p)
		}
	}
}

// Clone returns a deep copy.
func (d *Document) Clone() *Document {
	c := NewDocument()
	for p, v := range d.Values {
		v.Sources = slices.Clone(v.Sources)
		v.RuleIDs = slices.Clone(v.RuleIDs)
		if v.Computation != nil {
			comp := *v.Computation
			comp.Inputs = slices.Clone(comp.Inputs)
			v.Computation = &comp
		}
		if v.TaskID != nil {
			id := *v.TaskID
			v.TaskID = &id
		}
		c.Values[p] = v
	}
	for p, k := range d.Items {
		c.Items[p] = slices.Clone(k)
	}
	for p, o := range d.ItemSources {
		c.ItemSources[p] = slices.Clone(o)
	}
	maps.Copy(c.NextKey, d.NextKey)
	return c
}
