package tracfin

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
)

// AIValueRef names a field of an existing block.
type AIValueRef struct {
	BlockKey string `json:"block_key" jsonschema_description:"ID of an existing block of this section."`
	FieldID  string `json:"field_id" jsonschema_description:"Catalog field ID whose generated values are deleted."`
}

// AIAmendProposal is a minimal patch on an accepted section: values set on an
// existing block replace its generated value (same block ID, so references from
// other sections stay valid); new blocks are added as in a section proposal;
// generated blocks or values can be removed. Unchanged content is not restated.
type AIAmendProposal struct {
	AISectionProposal
	RemoveBlocks []string     `json:"remove_blocks" jsonschema_description:"IDs of generated blocks of this section to delete, with their content. Manual blocks cannot be deleted."`
	ClearValues  []AIValueRef `json:"clear_values" jsonschema_description:"Generated values to delete without replacement. Manual values cannot be deleted."`
}

// ApplyAIAmend applies a patch to the accepted section, then validates it like
// a section proposal. Manual values and blocks stay protected.
func ApplyAIAmend(d models.TracfinDraft, section string, p AIAmendProposal, sources map[string]bool, c Catalog, runID string) (models.TracfinDraft, []models.TracfinDiagnostic) {
	raw, _ := json.Marshal(d)
	var out models.TracfinDraft
	_ = json.Unmarshal(raw, &out)
	var diags []models.TracfinDiagnostic
	reject := func(code, message string) {
		diags = append(diags, aiDiagnostic(code, message, nil))
	}
	generated := func(o *models.TracfinOrigin) bool { return o != nil && o.Source == "ai" }

	for _, id := range p.RemoveBlocks {
		b := findBlock(&out.Document.Root, id)
		switch {
		case b == nil:
			reject("AMEND_TARGET_UNKNOWN", "remove_blocks "+id+": no such block in the document")
			continue
		case !AISectionOwns(section, b.CatalogID):
			reject("AMEND_SCOPE", "remove_blocks "+id+": a "+b.CatalogID+" block belongs to "+aiSectionOf(b.CatalogID)+", not "+section)
			continue
		}
		manual := false
		WalkBlocks(*b, func(x models.TracfinBlock) {
			if !generated(x.Origin) {
				manual = true
			}
			for _, values := range x.Fields {
				for _, v := range values {
					if !generated(v.Origin) {
						manual = true
					}
				}
			}
		})
		if manual {
			reject("MANUAL_PROTECTED", "remove_blocks "+id+": the block holds manual content and cannot be deleted")
			continue
		}
		next, err := ApplyChanges(out, []Change{{Action: "remove_block", BlockID: id}}, c)
		if err != nil {
			reject("AMEND_APPLICATION", "remove_blocks "+id+": "+err.Error())
			continue
		}
		out = next
	}

	// clearGenerated removes a field's generated values and the generated
	// attributes owned by them; manual values make the field protected.
	clearGenerated := func(b *models.TracfinBlock, field string) bool {
		for _, v := range b.Fields[field] {
			if !generated(v.Origin) {
				return false
			}
		}
		removed := map[string]bool{}
		for _, v := range b.Fields[field] {
			removed[v.ID] = true
		}
		delete(b.Fields, field)
		for id, values := range b.Fields {
			kept := values[:0]
			for _, v := range values {
				if !(removed[v.ParentValueID] && generated(v.Origin)) {
					kept = append(kept, v)
				}
			}
			b.Fields[id] = kept
		}
		return true
	}
	for _, ref := range p.ClearValues {
		b := findBlock(&out.Document.Root, ref.BlockKey)
		switch {
		case b == nil:
			reject("AMEND_TARGET_UNKNOWN", "clear_values "+ref.BlockKey+": no such block in the document")
		case !AISectionOwns(section, ref.FieldID):
			reject("AMEND_SCOPE", "clear_values "+ref.FieldID+": the field belongs to "+aiSectionOf(ref.FieldID)+", not "+section)
		case !clearGenerated(b, ref.FieldID):
			reject("MANUAL_PROTECTED", "clear_values "+ref.FieldID+": a manual value is protected")
		}
	}
	// A new value for a single-valued field of an existing block replaces the
	// generated one. A manual value is left in place: ApplyAISection reports it.
	for _, v := range p.Values {
		b := findBlock(&out.Document.Root, v.BlockKey)
		if b == nil || c.Fields[v.FieldID].Max != "1" || len(b.Fields[v.FieldID]) == 0 {
			continue
		}
		clearGenerated(b, v.FieldID)
	}
	if len(diags) > 0 {
		return d, diags
	}
	return ApplyAISection(out, section, p.AISectionProposal, sources, c, runID)
}
