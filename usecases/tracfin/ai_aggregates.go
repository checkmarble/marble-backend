package tracfin

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

const (
	aggregateTotalField     = "/ds/operations/montantTotal"
	aggregateStartField     = "/ds/operations/dateDebut"
	aggregateEndField       = "/ds/operations/dateFin"
	aggregateOperationField = "/ds/operations/operation/montantTotal"
	aggregateCountField     = "/ds/operations/operation/nombreOperations"
	AttachmentNameField     = "/ds/pieceJointe/piece/nom"
	AttachmentMimeField     = "/ds/pieceJointe/piece/type_mime"
)

// aiBackendComputed lists fields the backend derives itself, so the model never
// proposes them: the operations period and total and each line's operation
// count (from the accepted operations), and an attachment's name and MIME type
// (from the attached file).
var aiBackendComputed = map[string]bool{
	aggregateTotalField: true, aggregateStartField: true, aggregateEndField: true, aggregateCountField: true,
	AttachmentNameField: true, AttachmentMimeField: true,
}

// AIBackendComputed reports a field the backend fills itself during generation:
// catalog-calculated fields and the operations period and total.
func AIBackendComputed(c Catalog, id string) bool {
	return c.Fields[id].Coverage == "calculated" || aiBackendComputed[id]
}

// DropAIOperationAggregates removes model values for the operations period and
// total: the backend computes them, so a model copy is ignored rather than
// rejected (rejecting would only cost a repair).
func DropAIOperationAggregates(p *AISectionProposal) {
	kept := p.Values[:0]
	for _, v := range p.Values {
		if !aiBackendComputed[v.FieldID] {
			kept = append(kept, v)
		}
	}
	p.Values = kept
}

// TargetCurrency returns the currency of an amount field as established by the
// catalog or by configured mappings, or "" when unknown or contradictory.
func TargetCurrency(d models.TracfinDraft, c Catalog, field string) string {
	result := c.Fields[field].Currency
	for _, m := range d.Configuration.Mappings {
		if m.TargetField == field && m.TargetCurrency != "" {
			if result != "" && result != m.TargetCurrency {
				return ""
			}
			result = m.TargetCurrency
		}
	}
	return result
}

// ComputeAIOperationAggregates fills the operations period and total from the
// accepted principal operations:
//   - montantTotal is the exact sum of the lines' amounts, only when every line
//     has one and the aggregate currency is established and equal to theirs
//     (line currencies are already checked against their target currency);
//   - dateDebut/dateFin are the earliest/latest Paris civil dates, read through
//     the configured date mappings from the sources cited by each line amount.
//
// Manual values and manual clears are kept. Gaps are warnings, never guesses.
func ComputeAIOperationAggregates(d models.TracfinDraft, c Catalog, sources []models.TracfinAISource, runID string) (models.TracfinDraft, []models.TracfinDiagnostic) {
	byID := make(map[string]models.TracfinAISource, len(sources))
	for _, s := range sources {
		byID[s.ID] = s
	}
	var containers []models.TracfinBlock
	WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/operations" {
			containers = append(containers, b)
		}
	})
	var diagnostics []models.TracfinDiagnostic
	warn := func(code, message, block, field string) {
		diag := aiDiagnostic(code, message, &models.TracfinFieldRef{BlockID: block, FieldID: field})
		diag.Severity = "warning"
		diagnostics = append(diagnostics, diag)
	}
	for _, container := range containers {
		var lines []models.TracfinBlock
		for _, child := range container.Children {
			if child.CatalogID == "/ds/operations/operation" {
				lines = append(lines, child)
			}
		}
		if len(lines) == 0 {
			continue
		}

		// Operation count of each line: distinct source transactions cited by its amount.
		for _, line := range lines {
			identities := map[string]bool{}
			var evidence []models.TracfinAIEvidence
			for _, v := range line.Fields[aggregateOperationField] {
				if v.Origin == nil {
					continue
				}
				for _, e := range v.Origin.Evidence {
					if identity, locator, ok := aiSourceOperationIdentity(d, byID[e.SourceID]); ok {
						identities[identity] = true
						evidence = append(evidence, models.TracfinAIEvidence{SourceID: e.SourceID, Locator: locator})
					}
				}
			}
			if len(identities) == 0 {
				warn("OPERATION_COUNT_UNAVAILABLE", "No configured operation-count mapping applies to the sources of this operation amount: the count is not computed", line.ID, aggregateCountField)
				continue
			}
			text := fmt.Sprint(len(identities))
			explanation := fmt.Sprintf("Nombre de transactions sources distinctes citées par le montant de cette opération : %s (calculé par le backend).", text)
			d = SetComputedValue(d, c, line.ID, aggregateCountField, models.TracfinFieldInput{Kind: "text", Text: &text}, explanation, dedupeEvidence(evidence), runID, false)
		}

		// Total.
		total := new(big.Rat)
		scale, missing := 0, 0
		var totalEvidence []models.TracfinAIEvidence
		for _, line := range lines {
			amounts := line.Fields[aggregateOperationField]
			if len(amounts) != 1 || amounts[0].Decimal == nil {
				missing++
				continue
			}
			raw := string(*amounts[0].Decimal)
			n, ok := new(big.Rat).SetString(raw)
			if !ok {
				missing++
				continue
			}
			total.Add(total, n)
			if dot := strings.IndexByte(raw, '.'); dot >= 0 {
				scale = max(scale, len(raw)-dot-1)
			}
			if amounts[0].Origin != nil {
				totalEvidence = append(totalEvidence, amounts[0].Origin.Evidence...)
			}
		}
		currency := TargetCurrency(d, c, aggregateTotalField)
		switch {
		case missing > 0:
			warn("AMOUNT_TOTAL_INCOMPLETE", fmt.Sprintf("%d of %d principal operations have no amount: the total is not computed", missing, len(lines)), container.ID, aggregateTotalField)
		case currency == "" || currency != TargetCurrency(d, c, aggregateOperationField):
			warn("AMOUNT_TOTAL_CURRENCY_UNPROVEN", "The aggregate currency is not established by the catalog or configuration, or differs from the operations currency: the total is not computed", container.ID, aggregateTotalField)
		default:
			decimal := models.TracfinDecimal(total.FloatString(scale))
			explanation := fmt.Sprintf("Somme calculée par le backend des montants des %d opérations retenues : %s %s.", len(lines), decimal, currency)
			d = SetComputedValue(d, c, container.ID, aggregateTotalField, models.TracfinFieldInput{Kind: "decimal", Decimal: &decimal}, explanation, dedupeEvidence(totalEvidence), runID, false)
		}

		// Period.
		var dates []time.Time
		var dateEvidence []models.TracfinAIEvidence
		undated := 0
		for _, line := range lines {
			found := false
			for _, v := range line.Fields[aggregateOperationField] {
				if v.Origin == nil {
					continue
				}
				for _, e := range v.Origin.Evidence {
					date, locator, ok := aiSourceOperationDate(d, byID[e.SourceID])
					if ok {
						dates = append(dates, date)
						dateEvidence = append(dateEvidence, models.TracfinAIEvidence{SourceID: e.SourceID, Locator: locator})
						found = true
					}
				}
			}
			if !found {
				undated++
			}
		}
		if len(dates) == 0 {
			warn("PERIOD_UNAVAILABLE", "No configured date mapping applies to the sources of the principal operations: the period is not computed", container.ID, aggregateStartField)
			continue
		}
		sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
		partial := undated > 0
		note := ""
		if partial {
			note = fmt.Sprintf(" Période partielle : %d des %d opérations n'ont pas de date source ; à vérifier.", undated, len(lines))
			warn("PARTIAL_PERIOD", fmt.Sprintf("%d of %d principal operations have no source date: the period is to review", undated, len(lines)), container.ID, aggregateStartField)
		}
		evidence := dedupeEvidence(dateEvidence)
		for _, bound := range []struct {
			field, label string
			date         time.Time
		}{{aggregateStartField, "la plus ancienne", dates[0]}, {aggregateEndField, "la plus récente", dates[len(dates)-1]}} {
			text := bound.date.Format(profileDateLayout)
			explanation := fmt.Sprintf("Date %s des opérations retenues (date civile à Paris), calculée par le backend.%s", bound.label, note)
			d = SetComputedValue(d, c, container.ID, bound.field, models.TracfinFieldInput{Kind: "text", Text: &text}, explanation, evidence, runID, partial)
		}
	}
	return d, diagnostics
}

// aiSourceOperationDate reads an operation date from an ingested object or a
// decision snapshot, through a configured dateDebut/dateFin mapping of its type.
// A decision snapshot's historical type is the table of its trigger object.
func aiSourceOperationDate(d models.TracfinDraft, source models.TracfinAISource) (time.Time, string, bool) {
	var typ, prefix string
	var data map[string]any
	switch source.Kind {
	case "ingested_object":
		var content struct {
			Table  string `json:"table"`
			Object struct {
				Data map[string]any `json:"Data"`
			} `json:"object"`
		}
		if json.Unmarshal(source.Content, &content) != nil {
			return time.Time{}, "", false
		}
		typ, prefix, data = content.Table, "/object/Data/", content.Object.Data
	case "decision_snapshot":
		var content struct {
			HistoricalType string         `json:"historical_type"`
			Snapshot       map[string]any `json:"snapshot"`
		}
		if json.Unmarshal(source.Content, &content) != nil {
			return time.Time{}, "", false
		}
		typ, prefix, data = content.HistoricalType, "/snapshot/", content.Snapshot
	default:
		return time.Time{}, "", false
	}
	for _, m := range d.Configuration.Mappings {
		if m.TargetField != aggregateStartField && m.TargetField != aggregateEndField || m.Source.Field == "" {
			continue
		}
		if !(m.Source.Kind == "decision_snapshot" && m.Source.HistoricalType == typ) && !(m.Source.Kind == "ingested" && m.Source.Table == typ) {
			continue
		}
		text, ok := profileDate(data[m.Source.Field])
		if !ok {
			continue
		}
		date, err := time.Parse(profileDateLayout, text)
		if err == nil {
			return date, prefix + m.Source.Field, true
		}
	}
	return time.Time{}, "", false
}

// aiSourceOperationIdentity identifies the transaction behind an ingested object
// or a decision snapshot, through a configured unit-operation count mapping of
// its type, so the object and the snapshots of one transaction count once.
func aiSourceOperationIdentity(d models.TracfinDraft, source models.TracfinAISource) (string, string, bool) {
	var typ, objectID string
	var data map[string]any
	locator := "/id"
	switch source.Kind {
	case "ingested_object":
		var content struct {
			ID    string `json:"id"`
			Table string `json:"table"`
		}
		if json.Unmarshal(source.Content, &content) != nil {
			return "", "", false
		}
		typ, objectID = content.Table, content.ID
	case "decision_snapshot":
		var content struct {
			HistoricalType string         `json:"historical_type"`
			Snapshot       map[string]any `json:"snapshot"`
		}
		if json.Unmarshal(source.Content, &content) != nil {
			return "", "", false
		}
		typ, data = content.HistoricalType, content.Snapshot
	default:
		return "", "", false
	}
	for _, m := range d.Configuration.Mappings {
		if m.TargetField != aggregateCountField || !m.Source.UnitOperation {
			continue
		}
		if !(m.Source.Kind == "decision_snapshot" && m.Source.HistoricalType == typ) && !(m.Source.Kind == "ingested" && m.Source.Table == typ) {
			continue
		}
		id := objectID
		if data != nil {
			text, ok := data[m.Source.Field].(string)
			if !ok {
				continue
			}
			id, locator = text, "/snapshot/"+m.Source.Field
		}
		if id != "" {
			return typ + ":" + id, locator, true
		}
	}
	return "", "", false
}

func dedupeEvidence(in []models.TracfinAIEvidence) []models.TracfinAIEvidence {
	seen := map[models.TracfinAIEvidence]bool{}
	var out []models.TracfinAIEvidence
	for _, e := range in {
		if e.SourceID != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}

// SetComputedValue replaces a non-manual value with a backend-computed one.
func SetComputedValue(d models.TracfinDraft, c Catalog, blockID, field string, input models.TracfinFieldInput, explanation string, evidence []models.TracfinAIEvidence, runID string, needsReview bool) models.TracfinDraft {
	ref := models.TracfinFieldRef{BlockID: blockID, FieldID: field}
	protected := false
	WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.ID != blockID {
			return
		}
		existing := b.Fields[field]
		if len(existing) > 1 {
			protected = true
		}
		if len(existing) == 1 {
			if existing[0].Kind == "cleared" || existing[0].Origin == nil || existing[0].Origin.Source == "manual" {
				protected = true
			}
			ref.ValueID = existing[0].ID
		}
	})
	if protected {
		return d
	}
	next, err := ApplyChanges(d, []Change{{Action: "set_field", Field: ref, Value: &input}}, c)
	if err != nil {
		return d
	}
	origin := models.TracfinOrigin{Source: "calculated", Resolution: "resolved", RunID: runID, Explanation: explanation, Evidence: evidence}
	var mark func(b *models.TracfinBlock)
	mark = func(b *models.TracfinBlock) {
		if b.ID == blockID {
			values := b.Fields[field]
			// At most one value existed and it was replaced: mark the field's value.
			for i := range values {
				o := origin
				values[i].Origin = &o
				values[i].NeedsReview = needsReview
			}
			b.Fields[field] = values
			return
		}
		for i := range b.Children {
			mark(&b.Children[i])
		}
	}
	mark(&next.Document.Root)
	return next
}
