package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
)

type Origin string

const (
	OriginLLM        Origin = "llm"
	OriginOrgSetting Origin = "org_setting"
	OriginComputed   Origin = "computed"
	OriginManual     Origin = "manual"
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

// SourceRef is a citation of a value in an ingested object.
type SourceRef struct {
	Object ObjectRef `json:"object"`
	// Field is the field of the cited object that holds the value.
	Field string `json:"field"`
	// RawValue is the value as read in that field. The generation checks it
	// against the frozen snapshot of the object before accepting the citation.
	RawValue string `json:"raw_value"`
}

// Computation is the proof of an OriginComputed value.
type Computation struct {
	// Name is the name of the computation that produced the value.
	Name string `json:"name"`
	// Inputs are the document paths the computation read.
	Inputs []Path `json:"inputs"`
}

// Value is the content of one document path, with how it was obtained.
type Value struct {
	Value  string `json:"value"`
	Origin Origin `json:"origin"`
	// Sources are the object fields the value was read from (LLM values,
	// question options); a manual, setting or computed value has none.
	Sources []SourceRef `json:"sources,omitempty"`
	// RuleIDs are the memory rules (instructions given to the generation, not
	// format rules) the LLM cites as the reason for the value.
	RuleIDs []string `json:"rule_ids,omitempty"` //nolint:tagliatelle
	// Justification explains a value that is not a word-for-word copy of its
	// source (a judgment, a code chosen from a label, a conversion).
	Justification string `json:"justification,omitempty"`
	// Confidence is the LLM's confidence in a justified value; the engine forces
	// it to low when it cannot prove the correspondence. Empty when not given.
	Confidence Confidence `json:"confidence,omitempty"`
	// Reviewed is true once the analyst wrote or confirmed the value; an
	// unreviewed low-confidence or computed value is listed for review.
	Reviewed bool `json:"reviewed"`
	// Computation is the proof of an OriginComputed value; nil for other origins.
	Computation *Computation `json:"computation,omitempty"`
	// TaskID is the generation task that wrote the value; nil when it was not
	// written by a task.
	TaskID *uuid.UUID `json:"task_id,omitempty"`
}

// Item is one entry of a repeatable container: its key and the ingested
// objects it was built from, if any. Sources are those objects (e.g. the
// customer row a person item was created from).
type Item struct {
	Key     string      `json:"key"`
	Sources []ObjectRef `json:"sources,omitempty"`
}

// Document is the pivot instance of a declaration. Items maps a repeatable
// container path (without key segment, e.g. `personnes.pp`) to its ordered
// items; nested containers are keyed by their full concrete parent path.
// Outside this file, items are read and written through the accessors below.
// Repeatable fields (e.g. `nationalite[]`) are items too: their value is stored
// at the item path. An item of a repeatable field without a value is ignored by
// validation (indexAncestors, checkChildren) and by export (writeChild).
type Document struct {
	Values  map[Path]Value  `json:"values"`
	Items   map[Path][]Item `json:"items"`
	NextKey map[Path]int    `json:"next_key"`
}

func NewDocument() *Document {
	return &Document{
		Values:  map[Path]Value{},
		Items:   map[Path][]Item{},
		NextKey: map[Path]int{},
	}
}

// splitItem splits an item path such as `personnes.pp[pp-1]` into its
// container (`personnes.pp`) and key (`pp-1`).
func splitItem(item Path) (container Path, key string, ok bool) {
	s := string(item)
	i := strings.LastIndex(s, "[")
	if i < 0 || !strings.HasSuffix(s, "]") {
		return "", "", false
	}
	return Path(s[:i]), s[i+1 : len(s)-1], true
}

// findItem returns the index of the item in its container, or -1.
func (d *Document) findItem(item Path) (Path, int) {
	container, key, ok := splitItem(item)
	if !ok {
		return "", -1
	}
	return container, slices.IndexFunc(d.Items[container], func(it Item) bool { return it.Key == key })
}

// AddItem appends a new item `prefix-N` to the container, built from the
// given objects; N never repeats.
func (d *Document) AddItem(container Path, prefix string, sources ...ObjectRef) Path {
	d.NextKey[container]++
	key := fmt.Sprintf("%s-%d", prefix, d.NextKey[container])
	var src []ObjectRef
	if len(sources) > 0 {
		src = slices.Clone(sources)
	}
	d.Items[container] = append(d.Items[container], Item{Key: key, Sources: src})
	parent := container.Parent()
	name := strings.TrimPrefix(string(container), string(parent))
	name = strings.TrimPrefix(name, ".")
	return parent.Item(name, key)
}

// ItemKeys returns the ordered keys of the container's items.
func (d *Document) ItemKeys(container Path) []string {
	items := d.Items[container]
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.Key
	}
	return keys
}

// ItemContainers returns, sorted, the containers holding at least one item.
func (d *Document) ItemContainers() []Path {
	out := make([]Path, 0, len(d.Items))
	for c, items := range d.Items {
		if len(items) > 0 {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

// HasItem reports whether the item exists in its container.
func (d *Document) HasItem(item Path) bool {
	_, i := d.findItem(item)
	return i >= 0
}

// ItemSources returns a copy of the objects the item was built from; nil if
// the item does not exist or has no sources.
func (d *Document) ItemSources(item Path) []ObjectRef {
	container, i := d.findItem(item)
	if i < 0 {
		return nil
	}
	return slices.Clone(d.Items[container][i].Sources)
}

// SetItemSources replaces the objects an existing item was built from. It
// returns false, and changes nothing, when the item does not exist.
func (d *Document) SetItemSources(item Path, sources []ObjectRef) bool {
	container, i := d.findItem(item)
	if i < 0 {
		return false
	}
	var src []ObjectRef
	if len(sources) > 0 {
		src = slices.Clone(sources)
	}
	d.Items[container][i].Sources = src
	return true
}

// RemoveItem removes the item, with its sources, and everything below it.
func (d *Document) RemoveItem(item Path) {
	if container, key, ok := splitItem(item); ok {
		d.Items[container] = slices.DeleteFunc(d.Items[container], func(it Item) bool { return it.Key == key })
		if len(d.Items[container]) == 0 {
			delete(d.Items, container)
		}
	}
	for p := range d.Values {
		if p.HasPrefix(item) {
			delete(d.Values, p)
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
	for p, items := range d.Items {
		cp := make([]Item, len(items))
		for i, it := range items {
			cp[i] = Item{Key: it.Key, Sources: slices.Clone(it.Sources)}
		}
		c.Items[p] = cp
	}
	maps.Copy(c.NextKey, d.NextKey)
	return c
}

// UnmarshalJSON decodes a Document and guarantees all maps are non-nil, so
// documents stored with missing or null maps can be written to safely.
func (d *Document) UnmarshalJSON(data []byte) error {
	if string(bytes.TrimSpace(data)) == "null" {
		*d = *NewDocument()
		return nil
	}
	type plain Document
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*d = Document(p)
	if d.Values == nil {
		d.Values = map[Path]Value{}
	}
	if d.Items == nil {
		d.Items = map[Path][]Item{}
	}
	if d.NextKey == nil {
		d.NextKey = map[Path]int{}
	}
	return nil
}
