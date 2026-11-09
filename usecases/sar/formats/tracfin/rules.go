package tracfin

import (
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

// rules re-implements the conditional constraints documented in the
// financier XSD (annotations) and the TRACFIN business rules. Codes cite the
// XSD documentation line of the rule.
func rules() []format.Rule {
	has := func(n *format.Node, name string) bool {
		c := n.Get(name)
		return c != nil && (len(c.Children) > 0 || strings.TrimSpace(c.Value()) != "")
	}
	contains := func(n *format.Node, name, value string) bool {
		for _, c := range n.All(name) {
			if c.Value() == value {
				return true
			}
		}
		return false
	}
	req := func(code, ctx, field, msg string, when func(n *format.Node) bool) format.Rule {
		return format.Rule{Code: code, Context: ctx, Message: msg, Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			if when(n) && !has(n, field) {
				return []format.Finding{{Path: path + "/" + field + "[1]"}}
			}
			return nil
		}}
	}
	forbid := func(code, ctx, field, msg string, when func(n *format.Node) bool) format.Rule {
		return format.Rule{Code: code, Context: ctx, Message: msg, Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			if when(n) && has(n, field) {
				return []format.Finding{{Path: path + "/" + field + "[1]"}}
			}
			return nil
		}}
	}
	register := func(n *format.Node) string {
		if c := n.Get("numeroIdentification"); c != nil {
			return c.Str("type")
		}
		return ""
	}
	known := func(n *format.Node) bool { r := register(n); return r != "" && r != "Je ne sais pas" }
	op := func(n *format.Node) string { return n.Str("typeOperation") }
	ns := "/ds/context/natureSoupcon"
	out := []format.Rule{
		{Code: "TF-L1685-LEGAL_GROUND", Context: ns, Message: "Au moins un cadre légal principal CL01, CL02 ou CL03 est obligatoire", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			if contains(n, "cadreLegal", "CL01") || contains(n, "cadreLegal", "CL02") || contains(n, "cadreLegal", "CL03") {
				return nil
			}
			return []format.Finding{{Path: path + "/cadreLegal[1]"}}
		}},
		req("TF-L1695-PENAL_REQUIRED", ns, "natureSoupconPenale", "Nature de l'infraction pénale obligatoire avec CL02", func(n *format.Node) bool { return contains(n, "cadreLegal", "CL02") }),
		forbid("TF-L1695-PENAL_NOT_APPLICABLE", ns, "natureSoupconPenale", "Nature de l'infraction pénale réservée au cadre CL02", func(n *format.Node) bool { return !contains(n, "cadreLegal", "CL02") }),
		req("TF-L1709-TAX_REQUIRED", ns, "natureFraudeFiscale", "Nature de la fraude fiscale obligatoire avec CL03", func(n *format.Node) bool { return contains(n, "cadreLegal", "CL03") }),
		forbid("TF-L1709-TAX_NOT_APPLICABLE", ns, "natureFraudeFiscale", "Nature de la fraude fiscale réservée au cadre CL03", func(n *format.Node) bool { return !contains(n, "cadreLegal", "CL03") }),
		req("TF-L1702-OTHER_PENAL", ns, "autreNatureSoupconPenale", "Précision obligatoire avec NSP15 (autre infraction)", func(n *format.Node) bool { return contains(n, "natureSoupconPenale", "NSP15") }),
		forbid("TF-L1702-OTHER_PENAL_NA", ns, "autreNatureSoupconPenale", "Précision réservée à NSP15", func(n *format.Node) bool { return !contains(n, "natureSoupconPenale", "NSP15") }),
		req("TF-L1716-OTHER_TAX", ns, "autreNatureFraudeFiscale", "Précision obligatoire avec NFF10 (autre fraude)", func(n *format.Node) bool { return contains(n, "natureFraudeFiscale", "NFF10") }),
		forbid("TF-L1716-OTHER_TAX_NA", ns, "autreNatureFraudeFiscale", "Précision réservée à NFF10", func(n *format.Node) bool { return !contains(n, "natureFraudeFiscale", "NFF10") }),
		{Code: "TF-L1813-PERSON_REQUIRED", Context: "/ds", Message: "La déclaration doit contenir au moins une personne", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			p := n.Get("personnes")
			if p != nil && len(p.All("pp"))+len(p.All("pm"))+len(p.All("pnc")) > 0 {
				return nil
			}
			return []format.Finding{{Path: path + "/personnes[1]"}}
		}},
		req("TF-L890-BIRTH_OR_USAGE_NAME", "/ds/personnes/pp/etatCivil", "nomDusage", "Sans nom de naissance, le nom d'usage est obligatoire", func(n *format.Node) bool { return !has(n, "nomNaissance") }),
		req("TF-L898-FIRSTNAME_OR_ALIAS", "/ds/personnes/pp/etatCivil", "alias", "Sans prénom, un alias est obligatoire", func(n *format.Node) bool { return !has(n, "prenom") }),
		forbid("TF-L907-BIRTH_DATE_ALTERNATIVE", "/ds/personnes/pp/etatCivil", "dateNaissanceEstimation", "Date de naissance précise et estimée sont exclusives", func(n *format.Node) bool { return has(n, "dateNaissance") }),
		req("TF-L1273-DENOMINATION", "/ds/personnes/pm/activiteCoordonnee", "denominationSociale", "Dénomination obligatoire si le type de registre est connu", known),
		req("TF-L1280-SECTOR", "/ds/personnes/pm/activiteCoordonnee", "secteurActivite", "Secteur d'activité obligatoire avec un SIREN", func(n *format.Node) bool { return register(n) == "SIREN" }),
		req("TF-L1322-OTHER_REGISTER_NAME", "/ds/personnes/pm/activiteCoordonnee", "nomAutreTypeRegistre", "Nom du registre obligatoire pour RN6/RN7", func(n *format.Node) bool { return register(n) == "RN6" || register(n) == "RN7" }),
		req("TF-L1329-OTHER_REGISTER_COUNTRY", "/ds/personnes/pm/activiteCoordonnee", "paysAutreTypeRegistre", "Pays du registre obligatoire pour RN6/RN7", func(n *format.Node) bool { return register(n) == "RN6" || register(n) == "RN7" }),
		forbid("TF-L1287-TRADE_NAME_SIREN", "/ds/personnes/pm/activiteCoordonnee", "enseigneSigle", "Enseigne non applicable avec un SIREN", func(n *format.Node) bool { return register(n) == "SIREN" }),
		forbid("TF-L1294-LEGAL_FORM_SIREN", "/ds/personnes/pm/activiteCoordonnee", "formeJuridique", "Forme juridique non applicable avec un SIREN", func(n *format.Node) bool { return register(n) == "SIREN" }),
		forbid("TF-L1301-ACTIVITY_SIREN", "/ds/personnes/pm/activiteCoordonnee", "activitePrincipale", "Activité principale non applicable avec un SIREN", func(n *format.Node) bool { return register(n) == "SIREN" }),
		req("TF-L1308-ADDRESS", "/ds/personnes/pm/activiteCoordonnee", "adresse", "Adresse obligatoire avec un identifiant professionnel", func(n *format.Node) bool {
			return has(n, "numeroIdentification") && register(n) != "Non" && n.Str("numeroIdentification") != ""
		}),
		forbid("TF-L1315-OTHER_ADDRESS_SIREN", "/ds/personnes/pm/activiteCoordonnee", "autreAdresse", "Autre adresse réservée au SIREN", func(n *format.Node) bool { return register(n) != "SIREN" }),
		req("TF-L1394-OTHER_FUNCTION", "/ds/personnes/pm/dirigeant", "autreFonctionDirigeant", "Fonction « Autre » (MF14) : précision obligatoire", func(n *format.Node) bool { return n.Str("fonctionDirigeant") == "MF14" }),
		{Code: "TF-L1427-BENEFICIARY", Context: "/ds/personnes/pm/dirigeant", Message: "Un dirigeant non bénéficiaire exige un bénéficiaire déclaré ou nouveau, pas les deux", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			declared, other := has(n, "idBeneficiaireEffectif"), has(n, "autreBeneficiaireEffectif")
			if declared && other || n.Str("beneficiaireEffectif") == "Non" && !declared && !other {
				return []format.Finding{{Path: path + "/idBeneficiaireEffectif[1]"}}
			}
			return nil
		}},
		req("TF-L1545-EMITTER", "/ds/operations/operation", "emetteurTypeEmetteur", "Émetteur obligatoire sauf versement d'espèces (OT07)", func(n *format.Node) bool { return op(n) != "OT07" }),
		req("TF-L1553-ORIGIN_COUNTRY", "/ds/operations/operation", "paysOrigineFonds", "Pays d'origine obligatoire sauf versement d'espèces", func(n *format.Node) bool { return op(n) != "OT07" }),
		req("TF-L1560-EMISSION_REF", "/ds/operations/operation", "refFinanciereEmission", "Référence financière d'émission obligatoire sauf versement d'espèces", func(n *format.Node) bool { return op(n) != "OT07" }),
		req("TF-L1568-BENEFICIARY", "/ds/operations/operation", "beneficiaireTypeBeneficiaire", "Bénéficiaire obligatoire sauf retrait d'espèces (OT06)", func(n *format.Node) bool { return op(n) != "OT06" }),
		req("TF-L1576-ARRIVAL_COUNTRY", "/ds/operations/operation", "paysArriveeFonds", "Pays d'arrivée obligatoire sauf retrait d'espèces", func(n *format.Node) bool { return op(n) != "OT06" }),
		req("TF-L1583-RECEPTION_REF", "/ds/operations/operation", "refFinanciereReception", "Référence financière de réception obligatoire sauf retrait d'espèces", func(n *format.Node) bool { return op(n) != "OT06" }),
		req("TF-OTHER_OPERATION", "/ds/operations/operation", "autretypeOperation", "Opération « Autre » (OT11) : description obligatoire", func(n *format.Node) bool { return op(n) == "OT11" }),
		{Code: "TF-L1746-BANK_INFORMATION", Context: "/ds", Message: "Établissement de crédit (TPI1) : section information (gestion de fortune, correspondance bancaire) obligatoire", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			if n.Str("organisme/profession") == "TPI1" && !n.Has("context/information") {
				return []format.Finding{{Path: path + "/context[1]/information[1]"}}
			}
			return nil
		}},
		{Code: "TF-IDENTITY_PIECE", Context: "/ds/pieceJointe/piece", Message: "Une pièce d'identité (TA2, TA3, TA5) exige ses informations d'identité", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			t := n.Str("type")
			if (t == "TA2" || t == "TA3" || t == "TA5") && !n.Has("infoPieceIdentite") {
				return []format.Finding{{Path: path + "/infoPieceIdentite[1]"}}
			}
			return nil
		}},
		{Code: "TF-FINANCIAL_REFERENCE_LIMIT", Context: "/ds/personnes/pp/referencesFinancieres", Message: "20 références financières au plus par personne", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			if len(n.Children) > 20 {
				return []format.Finding{{}}
			}
			return nil
		}},
		{Code: "TF-ANALYSIS_EXACT", Context: "/ds", Message: "Le champ analyse doit reprendre l'analyse de l'analyste", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			if strings.TrimSpace(n.Str("analyse")) == "" {
				return []format.Finding{{Path: path + "/analyse[1]"}}
			}
			return nil
		}},
	}
	return out
}
