package tracfin

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

// SourceSet is server-built and request-local. Binding identity, not table name,
// addresses records: the same table can occur several times in a selection.
type SourceKey struct{ BlockID, BindingID string }
type SourceRecord struct {
	Table, RecordID string
	Data            map[string]any
}
type SourceSet struct {
	Records     map[SourceKey]SourceRecord
	Decisions   map[string]models.TracfinDecisionSource
	Diagnostics []models.TracfinDiagnostic
}
type PrefillProposal struct {
	ID         string                 `json:"id"`
	Field      models.TracfinFieldRef `json:"field"`
	Candidates []models.TracfinValue  `json:"candidates"`
	Resolution string                 `json:"resolution"`
}
type PrefillResult struct {
	DraftRevision  int64                      `json:"draft_revision"`
	MappingVersion string                     `json:"mapping_version"`
	Proposals      []PrefillProposal          `json:"proposals"`
	Diagnostics    []models.TracfinDiagnostic `json:"diagnostics"`
}

func MappingVersion(c models.TracfinConfiguration) string {
	b, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func DecodeSnapshot(raw json.RawMessage) (map[string]any, error) {
	var data map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&data); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("snapshot must be an object")
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return nil, errors.New("snapshot contains trailing JSON")
	}
	return data, nil
}
func WalkBlocks(b models.TracfinBlock, fn func(models.TracfinBlock)) {
	fn(b)
	for _, child := range b.Children {
		WalkBlocks(child, fn)
	}
}
func MappingForSelection(config models.TracfinConfiguration, s models.TracfinSelection) (models.TracfinMapping, bool) {
	for _, m := range config.Mappings {
		if m.ID == s.MappingID {
			return m, true
		}
	}
	return models.TracfinMapping{}, false
}
func CompatibleMapping(m models.TracfinMapping, s models.TracfinSelection) bool {
	if s.DecisionID != "" {
		return m.Source.Kind == "decision_snapshot" && m.Source.HistoricalType == s.HistoricalType
	}
	return m.Source.Kind == "ingested" && m.Source.Table == s.Root.Table
}
func BoundPath(m models.TracfinMapping, s models.TracfinSelection) (models.TracfinPathStep, bool) {
	parent := s.Root
	for _, step := range m.Source.Path {
		found := false
		for _, binding := range s.Bindings {
			if binding.ID == step.ID && binding.ParentID == parent.ID && binding.Table == step.Table && binding.Link == step.Link && binding.Direction == step.Direction && binding.RecordID != "" {
				parent = binding
				found = true
				break
			}
		}
		if !found {
			return models.TracfinPathStep{}, false
		}
	}
	return parent, parent.RecordID != ""
}
func diagnostic(code string, ref models.TracfinFieldRef) models.TracfinDiagnostic {
	return models.TracfinDiagnostic{Code: code, Layer: "sources", Severity: "warning", Message: code, Field: &ref}
}
func ResolvePrefill(config models.TracfinConfiguration, draft models.TracfinDraft, sources SourceSet) PrefillResult {
	version := MappingVersion(config)
	out := PrefillResult{DraftRevision: draft.Revision, MappingVersion: version, Diagnostics: append([]models.TracfinDiagnostic{}, sources.Diagnostics...)}
	catalog, err := LoadCatalog()
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, diagnostic("CATALOG_UNAVAILABLE", models.TracfinFieldRef{}))
		return out
	}
	if config.ProfileVersion != "" && config.ProfileVersion != catalog.Version || config.ProfileHash != "" && config.ProfileHash != catalog.Hash {
		out.Diagnostics = append(out.Diagnostics, diagnostic("INCOMPATIBLE_PROFILE", models.TracfinFieldRef{}))
		return out
	}
	positions := map[models.TracfinFieldRef]int{}
	WalkBlocks(draft.Document.Root, func(block models.TracfinBlock) {
		s := models.TracfinSelection{}
		if block.Selection != nil {
			s = *block.Selection
		}
		if _, ok := MappingForSelection(config, s); block.Selection != nil && !ok {
			out.Diagnostics = append(out.Diagnostics, diagnostic("MAPPING_UNAVAILABLE", models.TracfinFieldRef{BlockID: block.ID}))
			return
		}
		for _, m := range config.Mappings {
			if m.Source.Kind != "settings" && (block.Selection == nil || !CompatibleMapping(m, s)) {
				continue
			}
			ref := models.TracfinFieldRef{BlockID: block.ID, FieldID: m.TargetField}
			field, ok := catalog.Fields[m.TargetField]
			if !ok || field.Kind != "field" || field.Coverage != "editable" {
				out.Diagnostics = append(out.Diagnostics, diagnostic("INVALID_MAPPING_TARGET", ref))
				continue
			}
			owner := field.Parent
			for catalog.Fields[owner].Kind == "field" {
				owner = catalog.Fields[owner].Parent
			}
			if owner != block.CatalogID {
				continue
			}
			if len(block.Fields[m.TargetField]) > 1 {
				out.Diagnostics = append(out.Diagnostics, diagnostic("TARGET_OCCURRENCE_REQUIRED", ref))
				continue
			}
			if len(block.Fields[m.TargetField]) == 1 {
				ref.ValueID = block.Fields[m.TargetField][0].ID
			}
			parentValueID := ""
			if catalog.Fields[field.Parent].Kind == "field" {
				parents := block.Fields[field.Parent]
				if len(parents) != 1 || parents[0].ID == "" {
					out.Diagnostics = append(out.Diagnostics, diagnostic("PARENT_OCCURRENCE_REQUIRED", ref))
					continue
				}
				parentValueID = parents[0].ID
			}
			protected := false
			for _, old := range block.Fields[m.TargetField] {
				if old.Kind == "cleared" || old.Origin == nil || old.Origin.Source == "manual" {
					protected = true
				}
			}
			if protected {
				continue
			}
			var data map[string]any
			origin := models.TracfinSourceRef{Kind: m.Source.Kind, Field: m.Source.Field}
			if m.Source.Kind == "settings" {
				var settings []models.TracfinValue
				WalkBlocks(config.Settings.Root, func(b models.TracfinBlock) { settings = append(settings, b.Fields[m.Source.Field]...) })
				if len(settings) == 0 {
					continue
				}
				if len(settings) > 1 {
					out.Diagnostics = append(out.Diagnostics, diagnostic("SETTINGS_OCCURRENCE_REQUIRED", ref))
					continue
				}
				setting := settings[0]
				var raw any
				switch {
				case setting.Text != nil:
					raw = *setting.Text
				case setting.Decimal != nil:
					raw = json.Number(*setting.Decimal)
				case setting.Boolean != nil:
					raw = *setting.Boolean
				default:
					continue
				}
				data = map[string]any{m.Source.Field: raw}
			} else if m.Source.Kind == "decision_snapshot" {
				decision, ok := sources.Decisions[s.DecisionID]
				if !ok || decision.HistoricalType != m.Source.HistoricalType {
					out.Diagnostics = append(out.Diagnostics, diagnostic("SOURCE_UNAVAILABLE", ref))
					continue
				}
				var err error
				data, err = DecodeSnapshot(decision.Snapshot)
				if err != nil {
					out.Diagnostics = append(out.Diagnostics, diagnostic("SNAPSHOT_INVALID", ref))
					continue
				}
				origin.DecisionID = decision.DecisionID
				origin.HistoricalType = decision.HistoricalType
			} else {
				binding, ok := BoundPath(m, s)
				if !ok {
					continue
				}
				record, ok := sources.Records[SourceKey{block.ID, binding.ID}]
				if !ok || record.Table != binding.Table || record.RecordID != binding.RecordID {
					out.Diagnostics = append(out.Diagnostics, diagnostic("SOURCE_UNAVAILABLE", ref))
					continue
				}
				data = record.Data
				origin.Table = record.Table
				origin.RecordID = record.RecordID
				origin.Path = append([]models.TracfinPathStep{s.Root}, m.Source.Path...)
				for i, p := range origin.Path {
					if i == 0 {
						continue
					}
					for _, b := range s.Bindings {
						if b.ID == p.ID {
							origin.Path[i] = b
						}
					}
				}
			}
			if m.Discriminator != nil {
				v, ok := scalar(data[m.Discriminator.Field])
				known := false
				for _, allowed := range m.Discriminator.Values {
					if ok && allowed == v {
						known = true
					}
				}
				if !known {
					out.Diagnostics = append(out.Diagnostics, sourceDiagnostic("UNKNOWN_DISCRIMINATOR", ref, origin))
					continue
				}
			}
			raw, exists := data[m.Source.Field]
			if m.Source.UnitOperation && m.Source.Kind == "decision_snapshot" && m.TargetField == "/ds/operations/operation/nombreOperations" {
				raw = json.Number("1")
				exists = true
			}
			if !exists || raw == nil || raw == "" {
				continue
			}
			// Source identifiers are not stable IDs of document blocks. A genuine
			// internal reference needs an explicit declared-object selection.
			if field.ValueKind == "reference" {
				out.Diagnostics = append(out.Diagnostics, sourceDiagnostic("REFERENCE_SELECTION_REQUIRED", ref, origin))
				continue
			}
			if (len(field.Catalogs) > 0 || hasEnumeration(field)) && len(m.Codes) == 0 {
				out.Diagnostics = append(out.Diagnostics, sourceDiagnostic("UNKNOWN_CODE", ref, origin))
				continue
			}
			if field.ValueKind == "decimal" && (m.Unit == "" || m.TargetCurrency == "") {
				out.Diagnostics = append(out.Diagnostics, sourceDiagnostic("CURRENCY_OR_UNIT_UNKNOWN", ref, origin))
				continue
			}
			value, code := convertMapping(m, field, raw, data)
			if code != "" {
				out.Diagnostics = append(out.Diagnostics, sourceDiagnostic(code, ref, origin))
				continue
			}
			if field.ValueKind == "boolean" && value.Kind != "boolean" {
				out.Diagnostics = append(out.Diagnostics, diagnostic("SOURCE_TYPE_UNSUPPORTED", ref))
				continue
			}
			value.ParentValueID = parentValueID
			value.Origin = &models.TracfinOrigin{Source: m.Source.Kind, MappingVersion: version, Sources: []models.TracfinSourceRef{origin}, Resolution: "resolved"}
			pos, ok := positions[ref]
			if !ok {
				pos = len(out.Proposals)
				positions[ref] = pos
				out.Proposals = append(out.Proposals, PrefillProposal{ID: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s:%s", version, draft.Revision, block.ID, m.TargetField)))), Field: ref, Resolution: "resolved"})
			}
			proposal := &out.Proposals[pos]
			matched := false
			for i, c := range proposal.Candidates {
				if equalValue(c, value) {
					proposal.Candidates[i].Origin.Sources = appendSource(proposal.Candidates[i].Origin.Sources, origin)
					matched = true
					break
				}
			}
			if !matched {
				proposal.Candidates = append(proposal.Candidates, value)
			}
			if len(proposal.Candidates) > 1 {
				proposal.Resolution = "conflict"
				for i := range proposal.Candidates {
					proposal.Candidates[i].Origin.Resolution = "conflict"
				}
			}
		}
	})
	retained := out.Proposals[:0]
	for pi := range out.Proposals {
		proposal := &out.Proposals[pi]
		// Bind an explicit resolution to the entire source candidate set, including
		// provenance, so changing any competing source requires another decision.
		encoded, _ := json.Marshal(proposal.Candidates)
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(encoded))
		for ci := range proposal.Candidates {
			candidate := &proposal.Candidates[ci]
			candidate.Origin.SourceFingerprint = fingerprint
			value := *candidate
			value.Origin = nil
			encoded, _ := json.Marshal(value)
			candidate.ID = fmt.Sprintf("%x", sha256.Sum256(append([]byte(proposal.ID+":"+fingerprint+":"), encoded...)))
		}
		resolved := false
		WalkBlocks(draft.Document.Root, func(b models.TracfinBlock) {
			if b.ID != proposal.Field.BlockID {
				return
			}
			for _, v := range b.Fields[proposal.Field.FieldID] {
				if v.Origin != nil && v.Origin.Resolution == "resolved" && !v.NeedsReview && v.Origin.SourceFingerprint == fingerprint {
					resolved = true
				}
			}
		})
		if !resolved {
			retained = append(retained, *proposal)
		}
	}
	out.Proposals = retained
	out.Diagnostics = append(out.Diagnostics, duplicateSnapshotDiagnostics(draft, sources)...)
	return out
}

// Distinct decisions may record the same historical object. They stay separate
// lines: no merge and no sum. Identity is only the snapshot's exact object_id
// lexeme within one historical type, never an arbitrary id or name field.
func duplicateSnapshotDiagnostics(draft models.TracfinDraft, sources SourceSet) []models.TracfinDiagnostic {
	type selected struct {
		block  string
		source models.TracfinSourceRef
	}
	groups := map[[2]string][]selected{}
	var keys [][2]string
	WalkBlocks(draft.Document.Root, func(b models.TracfinBlock) {
		if b.Selection == nil || b.Selection.DecisionID == "" {
			return
		}
		decision, ok := sources.Decisions[b.Selection.DecisionID]
		if !ok || decision.HistoricalType != b.Selection.HistoricalType {
			return
		}
		data, err := DecodeSnapshot(decision.Snapshot)
		if err != nil {
			return
		}
		objectID, ok := scalar(data["object_id"])
		if !ok || objectID == "" {
			return
		}
		key := [2]string{decision.HistoricalType, objectID}
		if _, seen := groups[key]; !seen {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], selected{block: b.ID, source: models.TracfinSourceRef{Kind: "decision_snapshot", DecisionID: decision.DecisionID, HistoricalType: decision.HistoricalType, Field: "object_id"}})
	})
	var out []models.TracfinDiagnostic
	for _, key := range keys {
		group := groups[key]
		if len(group) < 2 {
			continue
		}
		sources := make([]models.TracfinSourceRef, 0, len(group))
		for _, s := range group {
			sources = append(sources, s.source)
		}
		for _, s := range group {
			out = append(out, models.TracfinDiagnostic{Code: "POTENTIAL_DUPLICATE_OPERATION", Layer: "sources", Severity: "warning", Message: "Several selected decisions record the same historical object; they are not merged or summed", Field: &models.TracfinFieldRef{BlockID: s.block}, Sources: append([]models.TracfinSourceRef(nil), sources...)})
		}
	}
	return out
}
func appendSource(sources []models.TracfinSourceRef, source models.TracfinSourceRef) []models.TracfinSourceRef {
	encoded, _ := json.Marshal(source)
	for _, old := range sources {
		other, _ := json.Marshal(old)
		if bytes.Equal(encoded, other) {
			return sources
		}
	}
	return append(sources, source)
}
func scalar(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	case bool:
		return strconv.FormatBool(x), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int16:
		return strconv.FormatInt(int64(x), 10), true
	case float64:
		// FLOAT columns hold binary values; the shortest exact round-trip lexeme
		// keeps the stored precision without adding digits or rounding.
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", false
		}
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return "", false
		}
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano), true
	}
	return "", false
}

const profileDateLayout = "02/01/2006"

// profileDate converts a source date to the profile's DD/MM/YYYY lexeme.
// Timestamps use their UTC calendar date, as Marble stores them in UTC.
// Other text is refused rather than proposed as an invalid date.
func profileDate(raw any) (string, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v.UTC().Format(profileDateLayout), true
	case string:
		for _, layout := range []string{profileDateLayout, time.DateOnly} {
			if d, err := time.Parse(layout, v); err == nil {
				return d.Format(profileDateLayout), true
			}
		}
		if d, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return d.UTC().Format(profileDateLayout), true
		}
	}
	return "", false
}
func equalValue(a, b models.TracfinValue) bool {
	if a.Kind == "decimal" && b.Kind == "decimal" && a.Decimal != nil && b.Decimal != nil {
		left, leftOK := new(big.Rat).SetString(string(*a.Decimal))
		right, rightOK := new(big.Rat).SetString(string(*b.Decimal))
		if !leftOK || !rightOK || left.Cmp(right) != 0 {
			return false
		}
		// Compare the remaining value metadata while preserving both original
		// decimal lexemes for display and provenance. No rounding is involved.
		a.Decimal, b.Decimal = nil, nil
	}
	a.Origin = nil
	b.Origin = nil
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

var decimalPattern = regexp.MustCompile(`^(-?)([0-9]+)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$`)

func exactDecimal(s string, minor bool) (models.TracfinDecimal, bool) {
	parts := decimalPattern.FindStringSubmatch(s)
	if parts == nil {
		return "", false
	}
	exponent := 0
	var err error
	if parts[4] != "" {
		exponent, err = strconv.Atoi(parts[4])
		if err != nil || exponent > 1000 || exponent < -1000 {
			return "", false
		}
	}
	scale := len(parts[3]) - exponent
	if minor {
		scale += 2
	}
	digits := strings.TrimLeft(parts[2]+parts[3], "0")
	if digits == "" {
		digits = "0"
	}
	if scale < 0 {
		digits += strings.Repeat("0", -scale)
	} else if scale > 0 {
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	}
	return models.TracfinDecimal(parts[1] + digits), true
}
func convertMapping(m models.TracfinMapping, field Field, raw any, data map[string]any) (models.TracfinValue, string) {
	if field.XSDType == "t_date" {
		date, ok := profileDate(raw)
		if !ok {
			return models.TracfinValue{}, "INVALID_SOURCE_DATE"
		}
		return models.TracfinValue{Kind: "text", Text: &date}, ""
	}
	text, ok := scalar(raw)
	if !ok {
		return models.TracfinValue{}, "SOURCE_TYPE_UNSUPPORTED"
	}
	if len(m.Codes) > 0 {
		found := false
		for _, code := range m.Codes {
			if code.Source == text {
				text = code.Target
				found = true
				break
			}
		}
		if !found {
			return models.TracfinValue{}, "UNKNOWN_CODE"
		}
	}
	if m.TargetCurrency != "" {
		currency := m.Currency.Constant
		if m.Currency.Field != "" {
			currency, _ = scalar(data[m.Currency.Field])
		}
		if currency == "" {
			return models.TracfinValue{}, "CURRENCY_UNKNOWN"
		}
		if currency != m.TargetCurrency {
			return models.TracfinValue{}, "CURRENCY_MISMATCH"
		}
	}
	if m.Unit != "" {
		if m.Unit != "major" && m.Unit != "minor" {
			return models.TracfinValue{}, "UNKNOWN_UNIT"
		}
		decimal, ok := exactDecimal(text, m.Unit == "minor")
		if !ok {
			return models.TracfinValue{}, "INVALID_DECIMAL"
		}
		return models.TracfinValue{Kind: "decimal", Decimal: &decimal}, ""
	}
	if b, ok := raw.(bool); ok {
		return models.TracfinValue{Kind: "boolean", Boolean: &b}, ""
	}
	return models.TracfinValue{Kind: "text", Text: &text}, ""
}

func hasEnumeration(f Field) bool {
	for _, c := range f.Constraints {
		if c.Kind == "enumeration" {
			return true
		}
	}
	return false
}

func sourceDiagnostic(code string, ref models.TracfinFieldRef, source models.TracfinSourceRef) models.TracfinDiagnostic {
	d := diagnostic(code, ref)
	d.Sources = []models.TracfinSourceRef{source}
	return d
}
