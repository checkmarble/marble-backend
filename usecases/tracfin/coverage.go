package tracfin

import (
	"github.com/checkmarble/marble-backend/models"
	"strconv"
)

// Unavailable rules matter only where a represented block uses the affected
// field or requires it. Optional absent sections do not poison a declaration.
func validateRuleCoverage(report *models.TracfinValidation, root models.TracfinBlock, c Catalog) {
	var visit func(models.TracfinBlock)
	visit = func(b models.TracfinBlock) {
		for _, rule := range c.Rules {
			if rule.Status != "unavailable" || rule.Kind == "official_xsd" {
				continue
			}
			for _, id := range rule.Fields {
				f, ok := c.Fields[id]
				if !ok || f.Coverage == "excluded" || !c.CompatibleParent(b.CatalogID, id) || !activeField(b, f, c) {
					continue
				}
				relevant := false
				if f.Kind == "block" {
					for _, child := range b.Children {
						if child.CatalogID == id {
							relevant = true
						}
					}
				} else {
					for _, v := range b.Fields[id] {
						if _, present := valueText(v); present || v.Reference != nil {
							relevant = true
						}
					}
				}
				if !relevant {
					min, _ := strconv.Atoi(f.Min)
					relevant = min > 0
					if f.Attribute && c.Fields[f.Parent].Kind == "field" {
						relevant = false
						for _, v := range b.Fields[f.Parent] {
							if _, present := valueText(v); present {
								relevant = min > 0
							}
						}
					}
				}
				if !relevant {
					continue
				}
				// The user-approved POC policy covers PDF only, while the official
				// MIME authority remains unavailable for missing or other values.
				if rule.ID == "catalog-liste_ref_typemime.csv" && id == "/ds/pieceJointe/piece/type_mime" {
					pdfOnly := len(b.Fields[id]) > 0
					for _, value := range b.Fields[id] {
						text, present := valueText(value)
						pdfOnly = pdfOnly && present && text == "application/pdf"
					}
					if pdfOnly {
						continue
					}
				}
				report.Diagnostics = append(report.Diagnostics, models.TracfinDiagnostic{Code: "RULE_NOT_EVALUATED", Layer: "coverage", Severity: "warning", Message: "Applicable source rule is unavailable: " + rule.ID, RuleID: rule.ID, Field: &models.TracfinFieldRef{BlockID: b.ID, FieldID: id}})
				layer := "business"
				if rule.Kind == "element" || rule.Kind == "attribute" {
					layer = "internal_structure"
				}
				l := report.Layers[layer]
				if l.Status != "failed" {
					l.Status = "incomplete"
					l.Reason = "Applicable source rule unavailable: " + rule.ID
					report.Layers[layer] = l
				}
			}
		}
		for _, child := range b.Children {
			if f, ok := c.Fields[child.CatalogID]; ok && activeField(b, f, c) {
				visit(child)
			}
		}
	}
	visit(root)
}
