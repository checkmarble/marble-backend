package tracfin

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
)

type Change = models.TracfinChangeInput

// ApplyChanges edits a private copy. IDs in this domain function are trusted;
// the HTTP-facing usecase allocates server IDs and verifies source selections.
// Proposals and configuration refresh require server reads and are not accepted here.
func ApplyChanges(draft models.TracfinDraft, changes []Change, catalog Catalog) (models.TracfinDraft, error) {
	data, err := json.Marshal(draft)
	if err != nil {
		return draft, err
	}
	var out models.TracfinDraft
	if err = json.Unmarshal(data, &out); err != nil {
		return draft, err
	}
	for _, change := range changes {
		if err = applyChange(&out.Document.Root, change, catalog); err != nil {
			return draft, err
		}
		if change.Action == "set_field" || change.Action == "clear_field" || change.Action == "set_relation" {
			activateAncestors(&out.Document.Root, change.Field.BlockID, catalog)
		}
	}
	return out, nil
}

// Editing an existing retained branch is also an explicit selection of that
// branch. Siblings remain stored so switching back restores their values.
func activateAncestors(b *models.TracfinBlock, id string, catalog Catalog) bool {
	if b.ID == id {
		return true
	}
	for i := range b.Children {
		if activateAncestors(&b.Children[i], id, catalog) {
			activateBranch(b, catalog.Fields[b.Children[i].CatalogID])
			return true
		}
	}
	return false
}

func findBlock(root *models.TracfinBlock, id string) *models.TracfinBlock {
	if root.ID == id {
		return root
	}
	for i := range root.Children {
		if b := findBlock(&root.Children[i], id); b != nil {
			return b
		}
	}
	return nil
}

func applyChange(root *models.TracfinBlock, c Change, catalog Catalog) error {
	bad := func(message string) error { return errors.Wrap(models.BadParameterError, message) }
	if c.SourceFileID != "" {
		return bad("file selection requires verified attachment content")
	}
	switch c.Action {
	case "add_block":
		parent := findBlock(root, c.ParentBlockID)
		f, ok := catalog.Fields[c.CatalogID]
		if parent == nil || !ok || f.Kind != "block" || !catalog.CompatibleParent(parent.CatalogID, c.CatalogID) {
			return bad("invalid block parent or catalogue")
		}
		if c.BlockID == "" {
			c.BlockID = pure_utils.NewId().String()
		}
		if findBlock(root, c.BlockID) != nil {
			return bad("duplicate block ID")
		}
		count := 0
		for _, b := range parent.Children {
			if b.CatalogID == c.CatalogID {
				count++
			}
		}
		if max, err := strconv.Atoi(f.Max); err == nil && count >= max {
			return bad("block cardinality exceeded")
		}
		if c.CatalogID == "/ds/operations/operation" && count >= 30 {
			return bad("maximum 30 principal operations")
		}
		parent.Children = append(parent.Children, models.TracfinBlock{ID: c.BlockID, CatalogID: c.CatalogID})
		activateBranch(parent, f)
		return nil
	case "remove_block":
		if c.BlockID == root.ID || findBlock(root, c.BlockID) == nil {
			return bad("invalid removed block")
		}
		removed := map[string]bool{}
		WalkBlocks(*findBlock(root, c.BlockID), func(b models.TracfinBlock) { removed[b.ID] = true })
		var remove func(*models.TracfinBlock)
		remove = func(b *models.TracfinBlock) {
			b.Children = slices.DeleteFunc(b.Children, func(child models.TracfinBlock) bool { return child.ID == c.BlockID })
			for i := range b.Children {
				remove(&b.Children[i])
			}
		}
		remove(root)
		markDetached(root, removed)
		return nil
	case "set_selection":
		b := findBlock(root, c.BlockID)
		if b == nil || c.Selection == nil {
			return bad("selection requires a block and selection")
		}
		s := *c.Selection
		s.Bindings = append([]models.TracfinPathStep(nil), s.Bindings...)
		if b.Selection != nil && !reflect.DeepEqual(*b.Selection, s) {
			changed := map[string]bool{}
			if b.Selection.Root != s.Root || b.Selection.DecisionID != s.DecisionID || b.Selection.MappingID != s.MappingID {
				changed["root"] = true
			}
			old := map[string]models.TracfinPathStep{}
			for _, step := range b.Selection.Bindings {
				old[step.ID] = step
			}
			kept := s.Bindings[:0]
			for _, step := range s.Bindings {
				if previous, existed := old[step.ID]; changed[step.ParentID] && existed && previous == step {
					changed[step.ID] = true
					continue
				}
				if prev, ok := old[step.ID]; !ok || prev != step {
					changed[step.ID] = true
				}
				kept = append(kept, step)
			}
			s.Bindings = kept
			markBlockReview(b)
		}
		if b.CatalogID == "/ds/operations/operation" && s.DecisionID != "" {
			duplicate := false
			WalkBlocks(*root, func(other models.TracfinBlock) {
				if other.ID != b.ID && other.CatalogID == b.CatalogID && other.Selection != nil && other.Selection.DecisionID == s.DecisionID {
					duplicate = true
				}
			})
			if duplicate {
				return bad("decision already selected")
			}
		}
		b.Selection = &s
		return nil
	case "set_field", "clear_field", "set_relation":
		b := findBlock(root, c.Field.BlockID)
		f, ok := catalog.Fields[c.Field.FieldID]
		if b == nil || !ok || f.Kind != "field" || f.Coverage != "editable" || !catalog.CompatibleParent(b.CatalogID, f.ID) {
			return bad("field is not editable in this block")
		}
		values := b.Fields[f.ID]
		index := -1
		if c.Field.ValueID != "" {
			for i := range values {
				if values[i].ID == c.Field.ValueID {
					index = i
				}
			}
			if index < 0 {
				return bad("unknown value occurrence")
			}
		}
		input := c.Value
		if c.Action == "set_relation" {
			input = &models.TracfinFieldInput{Kind: "reference", Reference: c.Relation}
		}
		value := models.TracfinValue{ID: c.Field.ValueID, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}
		if value.ID == "" {
			value.ID = pure_utils.NewId().String()
		}
		if c.Action == "clear_field" {
			value.Kind = "cleared"
			value.Origin.Resolution = "cleared"
			if index >= 0 {
				value.ParentValueID = values[index].ParentValueID
			} else if input != nil {
				value.ParentValueID = input.ParentValueID
			}
		} else {
			if input == nil {
				return bad("missing field value")
			}
			value.Kind, value.Text, value.Boolean, value.Decimal, value.Reference, value.ParentValueID = input.Kind, input.Text, input.Boolean, input.Decimal, input.Reference, input.ParentValueID
			n := 0
			if input.Text != nil {
				n++
			}
			if input.Boolean != nil {
				n++
			}
			if input.Decimal != nil {
				n++
			}
			if input.Reference != nil {
				n++
			}
			if n != 1 {
				return bad("field requires exactly one typed value")
			}
			if value.Kind != "text" && value.Kind != "boolean" && value.Kind != "decimal" && value.Kind != "reference" {
				return bad("invalid value kind")
			}
			if value.Kind == "text" && value.Text == nil || value.Kind == "boolean" && value.Boolean == nil || value.Kind == "decimal" && value.Decimal == nil || value.Kind == "reference" && value.Reference == nil {
				return bad("value kind mismatch")
			}
			if value.Kind != f.ValueKind && !(f.ValueKind == "text_or_reference" && (value.Kind == "text" || value.Kind == "reference")) {
				return bad("invalid field value kind")
			}
			if value.Reference != nil && value.Reference.BlockID == "" {
				return bad("empty reference")
			}
		}
		if catalog.Fields[f.Parent].Kind == "field" {
			valid := false
			for _, parent := range b.Fields[f.Parent] {
				if parent.ID == value.ParentValueID && parent.Kind != "cleared" {
					valid = true
				}
			}
			if !valid {
				return bad("attribute requires its scalar owner")
			}
		} else if value.ParentValueID != "" {
			return bad("unexpected scalar owner")
		}
		if b.Fields == nil {
			b.Fields = map[string][]models.TracfinValue{}
		}
		if index >= 0 {
			values[index] = value
		} else {
			count := 0
			for _, v := range values {
				if v.ParentValueID == value.ParentValueID {
					count++
				}
			}
			if max, err := strconv.Atoi(f.Max); err == nil && count >= max {
				return bad("field cardinality exceeded")
			}
			values = append(values, value)
		}
		b.Fields[f.ID] = values
		if c.Action == "clear_field" {
			for field, attrs := range b.Fields {
				b.Fields[field] = slices.DeleteFunc(attrs, func(attr models.TracfinValue) bool { return attr.ParentValueID == value.ID })
			}
		}
		activateBranch(b, f)
		return nil
	default:
		return bad("unsupported draft change")
	}
}

func activateBranch(b *models.TracfinBlock, f Field) {
	if f.Choice != "" {
		if b.ActiveBranches == nil {
			b.ActiveBranches = map[string]string{}
		}
		b.ActiveBranches[f.Choice] = f.ID
	}
}
func markBlockReview(b *models.TracfinBlock) {
	for field, values := range b.Fields {
		for i := range values {
			values[i].NeedsReview = true
			if values[i].Origin != nil && values[i].Origin.Source != "manual" {
				values[i].Origin.Resolution = "needs_review"
			}
		}
		b.Fields[field] = values
	}
	for i := range b.Children {
		markBlockReview(&b.Children[i])
	}
}
func markDetached(b *models.TracfinBlock, removed map[string]bool) {
	for field, values := range b.Fields {
		for i := range values {
			r := values[i].Reference
			if r != nil && (removed[r.BlockID] || removed[r.PersonBlockID]) {
				values[i].NeedsReview = true
				if values[i].Origin != nil && values[i].Origin.Source != "manual" {
					values[i].Origin.Resolution = "needs_review"
				}
			}
		}
		b.Fields[field] = values
	}
	for i := range b.Children {
		markDetached(&b.Children[i], removed)
	}
}
