package tracfin

import (
	"fmt"
	"github.com/checkmarble/marble-backend/models"
	"strings"
)

// validateConditionalRules applies the supplied profile's documented conditions to
// active represented blocks. It is shared by draft and independently parsed XML.
func validateConditionalRules(report *models.TracfinValidation, root models.TracfinBlock, c Catalog) {
	bank := false
	var findBank func(models.TracfinBlock)
	findBank = func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/organisme" {
			for _, v := range b.Fields[b.CatalogID+"/profession"] {
				text, present := valueText(v)
				bank = bank || present && text == "TPI1"
			}
		}
		for _, child := range b.Children {
			if activeField(b, c.Fields[child.CatalogID], c) {
				findBank(child)
			}
		}
	}
	findBank(root)
	var visit func(models.TracfinBlock)
	visit = func(b models.TracfinBlock) {
		text := func(name string) string {
			for _, v := range b.Fields[b.CatalogID+"/"+name] {
				if s, ok := valueText(v); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
			return ""
		}
		has := func(name string) bool {
			for _, v := range b.Fields[b.CatalogID+"/"+name] {
				if v.Kind != "cleared" && v.Reference != nil {
					return true
				}
				if s, ok := valueText(v); ok && strings.TrimSpace(s) != "" {
					return true
				}
			}
			for _, child := range b.Children {
				if child.CatalogID == b.CatalogID+"/"+name && activeField(b, c.Fields[child.CatalogID], c) {
					return true
				}
			}
			return false
		}
		contains := func(name, value string) bool {
			for _, v := range b.Fields[b.CatalogID+"/"+name] {
				s, ok := valueText(v)
				if ok && s == value {
					return true
				}
			}
			return false
		}
		add := func(name, code, message string, line int) {
			d := validationDiagnostic(code, "business", message, models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID + "/" + name})
			d.RuleID = fmt.Sprintf("xsd-L%d-documentation", line)
			addValidation(report, d)
		}
		require := func(name string, line int) {
			if !has(name) {
				add(name, "CONDITIONAL_REQUIRED", "Field is required by the current profile conditions", line)
			}
		}
		optionalOnly := func(name string, allowed bool, line int) {
			if has(name) && !allowed {
				add(name, "CONDITIONAL_NOT_APPLICABLE", "Field is not applicable to the selected condition", line)
			}
		}
		iff := func(name string, condition bool, line int) {
			if condition {
				require(name, line)
			} else {
				optionalOnly(name, false, line)
			}
		}
		civil := b.CatalogID == "/ds/personnes/pp/etatCivil" || b.CatalogID == "/ds/personnes/pp/etatCivil/autreIdentite"
		if civil {
			nameLine, aliasLine := 890, 898
			if strings.HasSuffix(b.CatalogID, "/autreIdentite") {
				nameLine, aliasLine = 853, 861
			}
			if !has("nomNaissance") {
				require("nomDusage", nameLine)
			}
			if !has("prenom") {
				require("alias", aliasLine)
			}
		}
		if civil || b.CatalogID == "/ds/personnes/pm/dirigeant/autreBeneficiaireEffectif" {
			if has("dateNaissance") && has("dateNaissanceEstimation") {
				line := 907
				if strings.HasSuffix(b.CatalogID, "/autreIdentite") {
					line = 870
				}
				if strings.HasSuffix(b.CatalogID, "/autreBeneficiaireEffectif") {
					line = 1360
				}
				add("dateNaissanceEstimation", "BIRTH_DATE_ALTERNATIVE", "A precise and estimated birth date cannot both be supplied", line)
			}
		}
		activityPP := b.CatalogID == "/ds/personnes/pp/activite"
		activityPM := b.CatalogID == "/ds/personnes/pm/activiteCoordonnee" || b.CatalogID == "/ds/personnes/pm/dirigeant/autreDirigeantM"
		if activityPP || activityPM {
			register := text("numeroIdentification/@type")
			known := register != "" && register != "Je ne sais pas"
			siren := register == "SIREN"
			foreign := register == "RN6" || register == "RN7"
			denominationLine, sectorLine, nameLine, countryLine := 1273, 1280, 1322, 1329
			if activityPP {
				denominationLine, sectorLine, nameLine, countryLine = 988, 995, 1023, 1030
			}
			if known {
				require("denominationSociale", denominationLine)
			}
			if siren {
				require("secteurActivite", sectorLine)
			}
			if foreign {
				require("nomAutreTypeRegistre", nameLine)
				require("paysAutreTypeRegistre", countryLine)
			}
			professionalID := has("numeroIdentification") && register != "Non"
			if activityPP {
				optionalOnly("formeJuridique", !siren, 1002)
				optionalOnly("activitePrincipale", !siren, 1009)
				optionalOnly("adresse", professionalID, 1016)
				optionalOnly("autreAdresse", siren, 1037)
			} else {
				optionalOnly("enseigneSigle", !siren, 1287)
				optionalOnly("formeJuridique", !siren, 1294)
				optionalOnly("activitePrincipale", !siren, 1301)
				if professionalID {
					require("adresse", 1308)
				}
				optionalOnly("autreAdresse", siren, 1315)
			}
		}
		if b.CatalogID == "/ds/personnes/pm/notorieteDefavorable" || b.CatalogID == "/ds/personnes/pp/notorieteDefavorable" {
			optionalOnly("listesSanctions", text("presenceSanction") == "true" || text("presenceSanction") == "1", 1221)
		}
		if b.CatalogID == "/ds/personnes/pm/dirigeant" {
			if text("fonctionDirigeant") == "MF14" {
				require("autreFonctionDirigeant", 1394)
			}
			declared, other := has("idBeneficiaireEffectif"), has("autreBeneficiaireEffectif")
			if declared && other {
				add("idBeneficiaireEffectif", "BENEFICIARY_ALTERNATIVE", "Choose an existing beneficiary or a newly declared beneficiary, not both", 1427)
			}
			if text("beneficiaireEffectif") == "Non" && !declared && !other {
				add("idBeneficiaireEffectif", "CONDITIONAL_REQUIRED", "A director who is not the beneficiary requires either a declared beneficiary reference or a new beneficiary", 1427)
			}
		}
		if b.CatalogID == "/ds/operations/operation" {
			op := text("typeOperation")
			if op != "OT07" {
				require("emetteurTypeEmetteur", 1545)
				require("paysOrigineFonds", 1553)
				require("refFinanciereEmission", 1560)
			}
			if op != "OT06" {
				require("beneficiaireTypeBeneficiaire", 1568)
				require("paysArriveeFonds", 1576)
				require("refFinanciereReception", 1583)
			}
		}
		if b.CatalogID == "/ds/context/natureSoupcon" {
			if !contains("cadreLegal", "CL01") && !contains("cadreLegal", "CL02") && !contains("cadreLegal", "CL03") {
				add("cadreLegal", "LEGAL_GROUND_REQUIRED", "At least one principal legal-ground code CL01, CL02 or CL03 is required", 1685)
			}
			iff("natureSoupconPenale", contains("cadreLegal", "CL02"), 1695)
			iff("natureFraudeFiscale", contains("cadreLegal", "CL03"), 1709)
			iff("autreNatureSoupconPenale", contains("natureSoupconPenale", "NSP15"), 1702)
			iff("autreNatureFraudeFiscale", contains("natureFraudeFiscale", "NFF10"), 1716)
		}
		if b.CatalogID == "/ds/context" && bank {
			require("information", 1746)
		}
		if b.CatalogID == "/ds/context/information" && bank {
			require("gestionFortuneBanquePrivee", 1746)
			require("correspondanceBancaire", 1753)
		}
		if b.CatalogID == "/ds/personnes" {
			count := 0
			for _, child := range b.Children {
				if child.CatalogID == "/ds/personnes/pp" || child.CatalogID == "/ds/personnes/pm" || child.CatalogID == "/ds/personnes/pnc" {
					count++
				}
			}
			if count == 0 {
				d := validationDiagnostic("PERSON_REQUIRED", "business", "The represented persons section must contain at least one person", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID})
				d.RuleID = "xsd-L1813-documentation"
				addValidation(report, d)
			}
		}
		if strings.HasSuffix(b.CatalogID, "/typeRelation") && has("sousCategorieRelationAutre") {
			code := text("sousCategorieRelation")
			other := strings.EqualFold(code, "autre")
			for _, entry := range c.Codes["relationsentrepersonnes.csv"] {
				if entry.Value == code && strings.Contains(strings.ToLower(entry.Label), "autre") {
					other = true
				}
			}
			optionalOnly("sousCategorieRelationAutre", other, 665)
		}
		for _, child := range b.Children {
			if activeField(b, c.Fields[child.CatalogID], c) {
				visit(child)
			}
		}
	}
	visit(root)
}
