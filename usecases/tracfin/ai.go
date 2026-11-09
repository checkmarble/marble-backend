package tracfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type AIEvidence = models.TracfinAIEvidence
type AIBlock struct {
	Key       string `json:"key" jsonschema_description:"New local key, unique in this proposal; values and children refer to it."`
	ParentKey string `json:"parent_key" jsonschema_description:"Key of a block declared earlier in this proposal, or ID of an existing document block, whose catalog type is the parent of catalog_id."`
	CatalogID string `json:"catalog_id" jsonschema_description:"Catalog ID of an entry with kind block."`
}
type AIValue struct {
	Key                string                 `json:"key" jsonschema_description:"New local key of this value, unique in this proposal."`
	BlockKey           string                 `json:"block_key" jsonschema_description:"Key or existing ID of the block holding the value; its catalog type must be the field owner_block."`
	FieldID            string                 `json:"field_id" jsonschema_description:"Catalog ID of an entry with kind field and coverage editable; follow its format and allowed values."`
	Kind               string                 `json:"kind" jsonschema:"enum=text,enum=boolean,enum=decimal,enum=reference" jsonschema_description:"Matches the field value_kind; set exactly the member of the same name (reference uses reference_key)."`
	Text               *string                `json:"text,omitempty"`
	Boolean            *bool                  `json:"boolean,omitempty"`
	Decimal            *models.TracfinDecimal `json:"decimal,omitempty" jsonschema:"-"` // computed by the backend from calculation, never by the model
	Calculation        *AICalculation         `json:"calculation,omitempty"`
	ReferenceKey       string                 `json:"reference_key,omitempty" jsonschema_description:"For kind reference: key or existing ID of the referenced block."`
	PersonReferenceKey string                 `json:"person_reference_key,omitempty" jsonschema_description:"For a reference that also needs its owning person: key or ID of that person block."`
	ParentValueKey     string                 `json:"parent_value_key,omitempty" jsonschema_description:"For an attribute with parent_scalar: key of that scalar value in the same block, emitted earlier in values."`
	Explanation        string                 `json:"explanation" jsonschema_description:"Short French justification; for a code, how it was matched."`
	Sources            []AIEvidence           `json:"sources" jsonschema_description:"At least one precise source for the value."`
}
type AIAttachment struct {
	BlockKey    string       `json:"block_key" jsonschema_description:"Key or ID of a /ds/pieceJointe/piece block."`
	SourceID    string       `json:"source_id" jsonschema_description:"file:<id> source of the case file attached to that piece."`
	Explanation string       `json:"explanation"`
	Sources     []AIEvidence `json:"sources" jsonschema_description:"Must include the selected file source."`
}
type AISectionProposal struct {
	Blocks      []AIBlock      `json:"blocks"`
	Values      []AIValue      `json:"values"`
	Attachments []AIAttachment `json:"attachments"`
	Missing     []string       `json:"missing" jsonschema_description:"French list of required information absent from the sources; [] when nothing required is missing."`
	Conflicts   []string       `json:"conflicts" jsonschema_description:"French list of contradictions between sources, left unresolved."`
}

// AISectionBatch is a complete JSON proposal with an explicit continuation marker.
// An absent marker must not silently certify a partially generated section.
type AISectionBatch struct {
	AISectionProposal
	HasMore *bool `json:"has_more"`
}

type AIEntityTask struct {
	Key         string       `json:"key"`
	Section     string       `json:"section"`
	CatalogID   string       `json:"catalog_id"`
	InventoryID string       `json:"inventory_id"`
	Explanation string       `json:"explanation"`
	Sources     []AIEvidence `json:"sources"`
}
type AIEntityExclusion struct {
	InventoryID string `json:"inventory_id"`
	Reason      string `json:"reason"`
}
type AIEntityPlan struct {
	Tasks      []AIEntityTask      `json:"tasks"`
	Exclusions []AIEntityExclusion `json:"exclusions"`
}

type AIReviewFinding struct {
	ID                string       `json:"id"`
	Section           string       `json:"section" jsonschema:"enum=attachments,enum=persons,enum=operations,enum=declaration,enum=analysis"`
	FieldID           string       `json:"field_id" jsonschema_description:"Catalog field ID owned by the section, /ds/analyse for analysis, or empty for a section-wide finding."`
	Message           string       `json:"message" jsonschema_description:"French, localized finding."`
	Blocking          bool         `json:"blocking" jsonschema_description:"True only when the section must be regenerated."`
	Sources           []AIEvidence `json:"sources"`
	SuggestedAnalysis string       `json:"suggested_analysis,omitempty" jsonschema_description:"Only for section analysis: a proposed analysis text for the human to accept."`
}
type AIReview struct {
	Findings []AIReviewFinding `json:"findings"`
}

// AIRemarkScope is the model's verdict on an analyst correction request.
type AIRemarkScope struct {
	InScope  bool     `json:"in_scope" jsonschema_description:"True only when the remark asks to correct, add or remove content of the declaration sections."`
	Sections []string `json:"sections" jsonschema_description:"Sections the remark concerns: attachments, persons, operations, declaration. Empty when refused."`
	Reason   string   `json:"reason" jsonschema_description:"One short French sentence for the analyst; when refusing, what cannot be done here and, if relevant, where to do it."`
}

var AISections = []string{"attachments", "persons", "operations", "declaration"}

func DecodeAIJSON(raw []byte, out any) error {
	if len(raw) > 1<<20 {
		return fmt.Errorf("response exceeds 1 MiB")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return fmt.Errorf("expected a JSON object")
	}
	switch out.(type) {
	case *AIEntityPlan:
		for _, key := range []string{"tasks", "exclusions"} {
			if value, ok := object[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s array is required", key)
			}
		}
	case *AIRemarkScope:
		for _, key := range []string{"in_scope", "sections", "reason"} {
			if value, ok := object[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s is required", key)
			}
		}
	case *AIReview:
		if value, ok := object["findings"]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("findings array is required")
		}
	case *AIAmendProposal:
		for _, key := range []string{"blocks", "values", "attachments", "missing", "conflicts", "remove_blocks", "clear_values"} {
			if value, ok := object[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s array is required", key)
			}
		}
	case *AISectionProposal, *AISectionBatch:
		for _, key := range []string{"blocks", "values", "attachments", "missing", "conflicts"} {
			if value, ok := object[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s array is required", key)
			}
		}
	}
	if _, batch := out.(*AISectionBatch); batch {
		var more bool
		value, ok := object["has_more"]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &more) != nil {
			return fmt.Errorf("has_more boolean is required")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON object")
	}
	return nil
}
func AISectionOwns(section, id string) bool {
	prefix := map[string]string{"persons": "/ds/personnes", "attachments": "/ds/pieceJointe", "operations": "/ds/operations"}[section]
	if prefix != "" {
		return id == prefix || strings.HasPrefix(id, prefix+"/")
	}
	if section != "declaration" {
		return false
	}
	return id != "/ds/analyse" && id != "/ds" && !strings.HasPrefix(id, "/ds/organisme") && !strings.HasPrefix(id, "/ds/declarant") && !strings.HasPrefix(id, "/ds/contact") && !AISectionOwns("persons", id) && !AISectionOwns("attachments", id) && !AISectionOwns("operations", id)
}

// aiIdenticalValue returns the ID of a value already applied by this run with the same payload.
func aiIdenticalValue(values []models.TracfinValue, input models.TracfinFieldInput, runID string) string {
	same := func(a, b any) bool {
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		return bytes.Equal(x, y)
	}
	for _, v := range values {
		if v.Origin == nil || v.Origin.Source != "ai" || v.Origin.RunID != runID || v.Kind != input.Kind || v.ParentValueID != input.ParentValueID {
			continue
		}
		if same(v.Text, input.Text) && same(v.Boolean, input.Boolean) && same(v.Decimal, input.Decimal) && same(v.Reference, input.Reference) {
			return v.ID
		}
	}
	return ""
}

// aiSectionOf names the generation section owning a catalog ID, for actionable repair messages.
func aiSectionOf(id string) string {
	for _, section := range AISections {
		if AISectionOwns(section, id) {
			return "section " + section
		}
	}
	return "the protected declarant/organisme/contact/analysis part"
}

// aiEditableFields lists the editable fields and attributes held directly by a block.
func aiEditableFields(c Catalog, block string) []string {
	var ids []string
	for id, f := range c.Fields {
		if f.Kind == "field" && f.Coverage == "editable" && c.OwnerBlock(id) == block {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func aiDiagnostic(code, message string, ref *models.TracfinFieldRef) models.TracfinDiagnostic {
	return models.TracfinDiagnostic{Code: code, Message: message, Layer: "ai", Severity: "error", Field: ref}
}
func CheckAIEvidence(explanation string, evidence []AIEvidence, sources map[string]bool) error {
	if strings.TrimSpace(explanation) == "" || len(evidence) == 0 {
		return fmt.Errorf("each value requires an explanation and precise source")
	}
	for _, s := range evidence {
		if !sources[s.SourceID] || strings.TrimSpace(s.Locator) == "" {
			return fmt.Errorf("unknown source or missing locator: %s", s.SourceID)
		}
	}
	return nil
}

// ValidateAIProposalEvidence verifies that each cited locator exists in the immutable source.
func ValidateAIProposalEvidence(p AISectionProposal, sources []models.TracfinAISource, analysisID, analysisText string) []models.TracfinDiagnostic {
	byID := make(map[string]models.TracfinAISource, len(sources))
	for _, source := range sources {
		byID[source.ID] = source
	}
	var diags []models.TracfinDiagnostic
	check := func(evidence []AIEvidence) {
		for _, e := range evidence {
			locator := strings.TrimSpace(e.Locator)
			valid := false
			if analysisID != "" && e.SourceID == "analysis:"+analysisID {
				valid = locator != "" && (locator == "text" || locator == "/text" || locator == "$.text" || strings.Contains(analysisText, locator))
			} else if source, ok := byID[e.SourceID]; ok && locator != "" {
				var content any
				if json.Unmarshal(source.Content, &content) == nil {
					valid = locator == source.Locator && source.Locator != "" || aiLocatorExists(content, locator)
				}
			}
			if !valid {
				diags = append(diags, aiDiagnostic("EVIDENCE_LOCATOR_INVALID", "Source locator does not resolve: "+e.SourceID+" "+locator, nil))
			}
		}
	}
	for _, v := range p.Values {
		check(v.Sources)
	}
	for _, a := range p.Attachments {
		check(a.Sources)
	}
	return diags
}

func aiLocatorExists(content any, locator string) bool {
	if strings.HasPrefix(locator, "$.") {
		locator = "/" + strings.ReplaceAll(strings.TrimPrefix(locator, "$."), ".", "/")
	}
	if strings.HasPrefix(locator, "/") {
		current := content
		for _, part := range strings.Split(strings.TrimPrefix(locator, "/"), "/") {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			switch v := current.(type) {
			case map[string]any:
				var ok bool
				current, ok = v[part]
				if !ok {
					return false
				}
			case []any:
				i, err := strconv.Atoi(part)
				if err != nil || i < 0 || i >= len(v) {
					return false
				}
				current = v[i]
			default:
				return false
			}
		}
		return true
	}
	var contains func(any) bool
	contains = func(v any) bool {
		switch v := v.(type) {
		case string:
			return strings.Contains(v, locator)
		case []any:
			for _, item := range v {
				if contains(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range v {
				if contains(item) {
					return true
				}
			}
		}
		return false
	}
	return contains(content)
}

// ApplyAISection validates and edits a private document. It never mutates input.
// Technical identifiers are allocated server-side, temporary keys exist only within this proposal.
func ApplyAISection(d models.TracfinDraft, section string, p AISectionProposal, sources map[string]bool, c Catalog, runID string) (models.TracfinDraft, []models.TracfinDiagnostic) {
	fail := func(code string, err error) (models.TracfinDraft, []models.TracfinDiagnostic) {
		return d, []models.TracfinDiagnostic{aiDiagnostic(code, err.Error(), nil)}
	}
	raw, _ := json.Marshal(d)
	var out models.TracfinDraft
	_ = json.Unmarshal(raw, &out)
	if len(p.Blocks) > 2000 || len(p.Values) > 10000 || len(p.Attachments) > 2000 {
		return fail("PROPOSAL_LIMIT", fmt.Errorf("too many blocks or values"))
	}
	aliases := map[string]string{}
	keys := map[string]bool{}
	WalkBlocks(out.Document.Root, func(b models.TracfinBlock) { aliases[b.ID] = b.ID })
	var blockDiags []models.TracfinDiagnostic
	for _, b := range p.Blocks {
		problem := ""
		f, known := c.Fields[b.CatalogID]
		switch {
		case b.Key == "" || keys[b.Key] || aliases[b.Key] != "":
			problem = "key is empty, duplicated or equal to an existing block id"
		case !known || f.Coverage == "excluded":
			problem = "catalog_id " + b.CatalogID + " is not in the catalog"
		case f.Kind != "block":
			problem = "catalog_id " + b.CatalogID + " is a field, not a block; put it in values on its block " + c.OwnerBlock(b.CatalogID)
		case !AISectionOwns(section, b.CatalogID):
			problem = "catalog_id " + b.CatalogID + " belongs to " + aiSectionOf(b.CatalogID) + ", not " + section
		}
		if problem != "" {
			blockDiags = append(blockDiags, aiDiagnostic("SECTION_SCOPE", "block "+b.Key+": "+problem, nil))
			continue
		}
		keys[b.Key] = true
		aliases[b.Key] = uuid.NewString()
	}
	if len(blockDiags) > 0 {
		return d, blockDiags
	}
	// Add blocks in dependency order; forward references between scalar references remain possible.
	pending := append([]AIBlock(nil), p.Blocks...)
	for len(pending) > 0 {
		next := pending[:0]
		progress := false
		for _, b := range pending {
			parent := findBlock(&out.Document.Root, aliases[b.ParentKey])
			if parent == nil {
				next = append(next, b)
				continue
			}
			var err error
			out, err = ApplyChanges(out, []Change{{Action: "add_block", BlockID: aliases[b.Key], ParentBlockID: parent.ID, CatalogID: b.CatalogID}}, c)
			if err != nil {
				return fail("BLOCK_CONTRACT", err)
			}
			added := findBlock(&out.Document.Root, aliases[b.Key])
			added.Origin = &models.TracfinOrigin{Source: "ai", Resolution: "resolved", SourceFingerprint: b.Key, RunID: runID, Explanation: "Server-created block for proposal " + b.Key}
			progress = true
		}
		if !progress {
			return fail("BLOCK_DEPENDENCY", fmt.Errorf("unknown or cyclic parent key"))
		}
		pending = next
	}
	attachmentBlocks := map[string]bool{}
	for _, a := range p.Attachments {
		if attachmentBlocks[a.BlockKey] {
			return fail("ATTACHMENT_TARGET", fmt.Errorf("duplicate attachment target"))
		}
		attachmentBlocks[a.BlockKey] = true
		if section != "attachments" {
			return fail("SECTION_SCOPE", fmt.Errorf("attachments belong to the attachments section"))
		}
		block := findBlock(&out.Document.Root, aliases[a.BlockKey])
		if block == nil || block.CatalogID != "/ds/pieceJointe/piece" {
			return fail("ATTACHMENT_TARGET", fmt.Errorf("attachment must target a piece block"))
		}
		if !strings.HasPrefix(a.SourceID, "file:") || !sources[a.SourceID] {
			return fail("ATTACHMENT_SOURCE", fmt.Errorf("attachment must select a known case file"))
		}
		if err := CheckAIEvidence(a.Explanation, a.Sources, sources); err != nil {
			return fail("EVIDENCE_REQUIRED", err)
		}
		cited := false
		for _, e := range a.Sources {
			if e.SourceID == a.SourceID {
				cited = true
			}
		}
		if !cited {
			return fail("EVIDENCE_REQUIRED", fmt.Errorf("attachment evidence must cite its selected file"))
		}
		if block.Origin == nil {
			block.Origin = &models.TracfinOrigin{Source: "ai", Resolution: "resolved", SourceFingerprint: a.BlockKey, RunID: runID}
		}
		if block.Origin.Source != "ai" {
			return fail("MANUAL_PROTECTED", fmt.Errorf("manual attachment block is protected"))
		}
		block.Origin.SourceFingerprint = a.BlockKey
		block.Origin.RunID = runID
		block.Origin.Explanation = a.Explanation
		block.Origin.Evidence = a.Sources
	}
	valueIDs := map[string]string{}
	for _, v := range p.Values {
		if v.Key == "" || valueIDs[v.Key] != "" {
			return fail("VALUE_KEY", fmt.Errorf("duplicate or empty value key"))
		}
		valueIDs[v.Key] = uuid.NewString()
	}
	// Value errors are independent: report all of them so one repair can fix every value.
	var valueDiags []models.TracfinDiagnostic
	reject := func(v AIValue, code, message string) {
		valueDiags = append(valueDiags, aiDiagnostic(code, "value "+v.Key+": "+message, nil))
	}
	for _, v := range p.Values {
		f, ok := c.Fields[v.FieldID]
		if ok && AIBackendComputed(c, f.ID) && AISectionOwns(section, v.FieldID) {
			// Identifiers, derived attributes and the operations period/total are
			// assigned by the backend; a model copy is redundant.
			continue
		}
		b := findBlock(&out.Document.Root, aliases[v.BlockKey])
		switch {
		case !ok || f.Coverage == "excluded":
			reject(v, "FIELD_UNKNOWN", "field_id "+v.FieldID+" is not in the catalog")
			continue
		case f.Kind == "block":
			reject(v, "FIELD_IS_BLOCK", v.FieldID+" is a block, not a field; declare it in blocks and set values on its editable fields: "+strings.Join(aiEditableFields(c, v.FieldID), ", "))
			continue
		case !AISectionOwns(section, v.FieldID):
			reject(v, "SECTION_SCOPE", "field "+v.FieldID+" belongs to "+aiSectionOf(v.FieldID)+", not "+section+"; omit it here")
			continue
		case f.Coverage != "editable":
			reject(v, "FIELD_NOT_EDITABLE", "field "+v.FieldID+" has coverage "+f.Coverage+" and cannot receive a value")
			continue
		case b == nil:
			reject(v, "VALUE_BLOCK_UNKNOWN", "block_key "+v.BlockKey+" is neither a key in blocks nor an existing block id")
			continue
		case !c.CompatibleParent(b.CatalogID, v.FieldID):
			reject(v, "FIELD_BLOCK_MISMATCH", "field "+v.FieldID+" belongs to a "+c.OwnerBlock(v.FieldID)+" block, but block_key "+v.BlockKey+" is a "+b.CatalogID+" block")
			continue
		}
		if err := CheckAIEvidence(v.Explanation, v.Sources, sources); err != nil {
			reject(v, "EVIDENCE_REQUIRED", err.Error())
			continue
		}
		protected := false
		for _, existing := range b.Fields[v.FieldID] {
			if existing.Origin == nil || existing.Origin.Source != "ai" {
				protected = true
			}
		}
		if protected {
			reject(v, "MANUAL_PROTECTED", "manual or deterministic field protected: "+v.FieldID+"; omit it")
			continue
		}
		// Only the member named by kind is meaningful; a stray text next to a reference is ignored.
		input := models.TracfinFieldInput{Kind: v.Kind}
		switch v.Kind {
		case "text":
			input.Text = v.Text
		case "boolean":
			input.Boolean = v.Boolean
		case "decimal":
			input.Decimal = v.Decimal
		}
		if v.ParentValueKey != "" {
			input.ParentValueID = valueIDs[v.ParentValueKey]
			if input.ParentValueID == "" {
				for _, parent := range b.Fields[f.Parent] {
					if parent.ID == v.ParentValueKey && parent.Kind != "cleared" {
						input.ParentValueID = parent.ID
					}
				}
			}
			if input.ParentValueID == "" {
				reject(v, "VALUE_OWNER", "parent_value_key "+v.ParentValueKey+" does not name a scalar "+f.Parent+" value in the same block")
				continue
			}
		}
		if v.Kind == "reference" {
			if v.ReferenceKey == "" {
				reject(v, "UNKNOWN_REFERENCE", "reference_key is required for kind reference: set it to the key or ID of the referenced block (person_reference_key only names the person owning that block)")
				continue
			}
			target := aliases[v.ReferenceKey]
			if target == "" || findBlock(&out.Document.Root, target) == nil {
				reject(v, "UNKNOWN_REFERENCE", "reference_key "+v.ReferenceKey+" must name an accepted or proposed block")
				continue
			}
			person := ""
			if v.PersonReferenceKey != "" {
				person = aliases[v.PersonReferenceKey]
				if person == "" {
					reject(v, "UNKNOWN_REFERENCE", "person_reference_key "+v.PersonReferenceKey+" is unknown")
					continue
				}
			}
			input.Reference = &models.TracfinReference{BlockID: target, PersonBlockID: person}
		}
		// Assembled batches and finalizers may restate a value: an identical one is applied once.
		if existing := aiIdenticalValue(b.Fields[v.FieldID], input, runID); existing != "" {
			valueIDs[v.Key] = existing
			continue
		}
		// Append through the catalog editor, then replace its allocated ID with the server allocation.
		before := len(b.Fields[v.FieldID])
		next, err := ApplyChanges(out, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: b.ID, FieldID: v.FieldID}, Value: &input}}, c)
		if err != nil {
			reject(v, "FIELD_CONTRACT", v.FieldID+": "+err.Error())
			continue
		}
		out = next
		b = findBlock(&out.Document.Root, aliases[v.BlockKey])
		values := b.Fields[v.FieldID]
		if len(values) <= before {
			reject(v, "VALUE_APPLICATION", "value was not appended")
			continue
		}
		values[len(values)-1].ID = valueIDs[v.Key]
		values[len(values)-1].Origin = &models.TracfinOrigin{Source: "ai", Resolution: "resolved", Explanation: v.Explanation, Evidence: v.Sources, RunID: runID}
		b.Fields[v.FieldID] = values
	}
	if len(valueDiags) > 0 {
		return d, valueDiags
	}
	// Local validation excludes missing required values in later sections. Such omissions stay deferred.
	report := ValidateDraft(out, c)
	// The model never sees server-allocated IDs: name its own keys so it can locate each problem.
	keyOf := map[string]string{}
	for key, id := range aliases {
		if keys[key] {
			keyOf[id] = "block_key " + key
		}
	}
	for key, id := range valueIDs {
		keyOf[id] = "value key " + key
	}
	var diags []models.TracfinDiagnostic
	for _, diag := range report.Diagnostics {
		if diag.Severity != "error" || diag.Field == nil || !AISectionOwns(section, diag.Field.FieldID) {
			continue
		}
		if diag.Code == "MISSING_REQUIRED_VALUE" && aiBackendComputed[diag.Field.FieldID] {
			// Computed by the backend once the section is accepted: not the model's gap.
			continue
		}
		var names []string
		for _, id := range []string{diag.Field.BlockID, diag.Field.ValueID} {
			if name := keyOf[id]; name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			diag.Message += " (" + diag.Field.FieldID + ", " + strings.Join(names, ", ") + ")"
		}
		if diag.Code == "MISSING_REQUIRED_VALUE" && diag.Layer == "internal_structure" {
			diag.Code = "AI_MISSING_REQUIRED"
			diag.Severity = "warning"
		}
		diags = append(diags, diag)
	}
	for _, diag := range diags {
		if diag.Severity == "error" {
			return d, diags
		}
	}
	return out, diags
}
