package goaml

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

func init() {
	sar.Register(New(Luxembourg()))
	sar.Register(New(Switzerland()))
}

const (
	KeyLU = "goaml:LU:5.0.2"
	KeyCH = "goaml:CH:2025.3"
)

func loc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return l
}

func settingsFields(extra ...sar.ExtField) []sar.ExtField {
	base := []sar.ExtField{
		{Key: "rentity_id", Scope: "settings", Label: "Identifiant goAML de l'entité déclarante (rentity_id)", Kind: "text", Min: 1, Max: 1},
		{Key: "rentity_branch", Scope: "settings", Label: "Succursale (rentity_branch)", Kind: "text", Max: 1},
		{Key: "reporting_user_code", Scope: "settings", Label: "Code utilisateur goAML du déclarant", Kind: "text", Max: 1},
		{Key: "reporting_person_first_name", Scope: "settings", Label: "Prénom du déclarant", Kind: "text", Max: 1},
		{Key: "reporting_person_last_name", Scope: "settings", Label: "Nom du déclarant", Kind: "text", Max: 1},
		{Key: "reporting_person_phone", Scope: "settings", Label: "Téléphone du déclarant", Kind: "text", Max: 1},
		{Key: "entity_reference_prefix", Scope: "settings", Label: "Préfixe des références internes", Kind: "text", Max: 1},
	}
	return append(base, extra...)
}

func commonExt(extra ...sar.ExtField) []sar.ExtField {
	base := []sar.ExtField{
		{Key: "report_type", Scope: "report", Label: "Type de déclaration (report_code)", Kind: "code", CodeList: "report_type", Min: 1, Max: 1, Computed: true},
		{Key: "report_type_override", Scope: "report", Label: "Type de déclaration imposé par l'analyste", Kind: "code", CodeList: "report_type", Max: 1},
		{Key: "entity_reference", Scope: "report", Label: "Référence interne (entity_reference)", Kind: "text", Min: 1, Max: 1, Computed: true},
		{Key: "submission_date", Scope: "report", Label: "Date de déclaration", Kind: "text", Min: 1, Max: 1, Computed: true},
		{Key: "fiu_ref_number", Scope: "report", Label: "Référence CRF d'un dossier lié (fiu_ref_number)", Kind: "text", MaxLength: 255, Max: 1},
		{Key: "funds_code", Scope: "operation", Label: "Nature des fonds (funds_code)", Kind: "code", CodeList: "funds_type", Min: 1, Max: 1, AI: true,
			Instructions: "Code funds_type de la transaction, identique côté from et to. Une valeur par défaut est déduite du type neutre de l'opération ; ne la changez que si les sources montrent une autre nature."},
		{Key: "transmode_code", Scope: "operation", Label: "transmode_code", Kind: "code", CodeList: "conduction_type", Max: 1, AI: true},
	}
	return append(append(base, extra...), settingsFields()...)
}

// ---------------------------------------------------------------- LU

func Luxembourg() *Profile {
	s := luSchema()
	s.CodeLists["rentity_type"] = &format.CodeList{Name: "rentity_type", Codes: []format.Code{{Value: "RE", Label: "Reporting entity"}, {Value: "SH", Label: "Shareholder"}, {Value: "FIU", Label: "FIU"}, {Value: "SUP", Label: "Supervisor"}, {Value: "IND", Label: "Individual"}}}
	p := &Profile{
		Desc:        sar.Descriptor{Key: KeyLU, Format: "goaml", Country: "LU", Version: "5.0.2 (XSD CRF 2026-10-19)", Label: "goAML Luxembourg (CRF)", FIU: "CRF Luxembourg", Currency: "EUR", Packaging: "zip", Timezone: "Europe/Luxembourg"},
		Schema:      s,
		TZ:          loc("Europe/Luxembourg"),
		MaxTx:       500,
		DateElement: "submission_date",
		Ext: commonExt(sar.ExtField{Key: "indicators", Scope: "report", Label: "Indicateurs (report_indicators)", Kind: "code", CodeList: "report_indicator_type", Min: 1, Max: -1, AI: true,
			Instructions: "Choisissez les indicateurs du catalogue CRF 2026 (préfixes 002TS déclencheur, 003MT typologie, 005PU produit, 006SP personne suspecte, 007SO infraction sous-jacente, 010RS statut de la relation, 011CR crypto, 012EC e-commerce). N'utilisez pas les anciens codes numériques. L'indicateur 008SA (tranche de montant) est calculé par le backend : ne le proposez pas."}),
		Instructions: "Format goAML de la CRF Luxembourg. Le backend choisit STR (avec transactions), SAR (sans transaction), TFTR/TFAR (financement du terrorisme). Il calcule montants en EUR, dates, comptes et références. Vous ne remplissez que les codes : indicateurs (au moins un, REJ-IND01) et, si besoin, funds_code par transaction (REJ-TRS04 : même code des deux côtés ; Q = virement exige des comptes des deux côtés, REJ-TRS03).",
		Funds: func(op *models.SarOperation) string {
			switch op.Type.Val() {
			case "transfer", "direct_debit":
				return "Q"
			case "cash_deposit":
				if op.Channel.Val() == "atm" {
					return "P"
				}
				return "O"
			case "cash_withdrawal":
				if op.Channel.Val() == "atm" {
					return "I"
				}
				return "H"
			case "card_payment":
				return "G"
			case "cheque":
				return "C"
			case "crypto":
				return "R"
			case "money_remittance":
				return "S"
			}
			return "-"
		},
		Transmode: func(op *models.SarOperation) string {
			return map[string]string{"executed": "A", "refused": "B", "pending": "C", "attempted": "F"}[op.Status.Val()]
		},
		AccountType:   map[string]string{"bank_account": "C", "card": "B", "prepaid_card": "P", "e_wallet": "E", "crypto_wallet": "U", "contract": "V", "other": "-"},
		AccountStatus: map[string]string{"active": "A", "closed": "B", "dormant": "C", "blocked": "E"},
		Gender:        map[string]string{"male": "M", "female": "F", "unknown": "-", "other": "-"},
		IDType:        map[string]string{"driving_licence": "A", "id_card": "B", "passport": "C", "residence_permit": "-", "other": "-"},
		LegalForm:     map[string]string{"company": "B", "partnership": "F", "foundation": "H", "association": "I", "trust": "L", "sole_trader": "H", "public_body": "H", "other": "H"},
		AddressType:   map[string]string{"home": "1", "business": "2", "other": "2"},
		PhoneType:     [2]string{"1", "L"},
		EntityRole:    map[string]string{"director": "A", "beneficial_owner": "G", "shareholder": "H"},
		AccountRole:   map[string]string{"holder": "A", "co_holder": "A", "signatory": "B", "beneficial_owner": "C"},
		Suspected:     true,
		ReportType: func(in sar.Input, ops []*models.SarOperation) (string, string) {
			tf := sar.HasTheme(in.Pivot, "terrorism_financing")
			switch {
			case tf && len(ops) > 0:
				return "TFTR", "financement du terrorisme avec transactions"
			case tf:
				return "TFAR", "financement du terrorisme sans transaction"
			case len(ops) > 0:
				return "STR", "déclaration avec transactions"
			}
			return "SAR", "déclaration sans transaction"
		},
		Rules: luRules(),
	}
	p.Ext = append(p.Ext, sar.ExtField{Key: "amount_band", Scope: "report", Label: "Indicateur de tranche de montant (008SA)", Kind: "code", CodeList: "report_indicator_type", Max: 1, Computed: true})
	p.PrepareHook = luPrepare
	return p
}

// amountBand computes the LU 008SA indicator from the reported total.
func amountBand(total string) string {
	bands := []struct{ max, code string }{{"0", "008SA-10-0EUR"}, {"5000", "008SA-20-Under5kEUR"}, {"10000", "008SA-30-5kTo10kEUR"}, {"15000", "008SA-40-10kTo15kEUR"}, {"25000", "008SA-50-15kTo25kEUR"}, {"100000", "008SA-60-25kTo100kEUR"}, {"1000000", "008SA-70-100KTo1MioEUR"}, {"5000000", "008SA-80-1MioTo5MioEUR"}}
	for _, b := range bands {
		if sar.DecimalCmp(total, b.max) <= 0 {
			return b.code
		}
	}
	return "008SA-90-Over5MioEUR"
}

func childValues(n *format.Node, path string) []string {
	parts := strings.Split(path, "/")
	nodes := []*format.Node{n}
	for _, p := range parts {
		var next []*format.Node
		for _, x := range nodes {
			next = append(next, x.All(p)...)
		}
		nodes = next
	}
	out := make([]string, 0, len(nodes))
	for _, x := range nodes {
		out = append(out, x.Value())
	}
	return out
}

var (
	ibanRe  = regexp.MustCompile(`^([A-Z]{2}\d{2}[A-Z0-9]{10,30})$`)
	swiftRe = regexp.MustCompile(`^([A-Z0-9]{4}[A-Z]{2}[A-Z0-9]{2}|[A-Z0-9]{4}[A-Z]{2}[A-Z0-9]{5})$`)
	rcsRe   = regexp.MustCompile(`([A-Z]{1}[1-9]\d*)$`)
)

func one(ok bool) []format.Finding {
	if ok {
		return nil
	}
	return []format.Finding{{}}
}

func accountRules(ctx string) []format.Rule {
	return []format.Rule{
		{Code: "REJ-IBAN01", Context: ctx, Message: "IBAN mal formaté (REJ-IBAN01)", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			return one(!n.Has("iban") || ibanRe.MatchString(n.Str("iban")))
		}},
		{Code: "REJ-ACC01", Context: ctx, Message: "Si un IBAN est renseigné, account doit être identique à l'IBAN (REJ-ACC01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("iban") || n.Str("iban") == n.Str("account"))
		}},
		{Code: "REJ-ACC02", Context: ctx, Message: "Un IBAN exige un code SWIFT/BIC (REJ-ACC02)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("iban") || n.Has("swift"))
		}},
		{Code: "REJ-BIC01", Context: ctx, Message: "Code SWIFT/BIC mal formaté (REJ-BIC01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("swift") || swiftRe.MatchString(n.Str("swift")))
		}},
		{Code: "REJ-ACC03", Context: ctx, Message: "Un SWIFT de 11 caractères doit être repris dans branch (REJ-ACC03)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(len(n.Str("swift")) != 11 || n.Str("branch") == n.Str("swift"))
		}},
		{Code: "REJ-ACC04", Context: ctx, Message: "Avec un institution_code, omettre institution_name (REJ-ACC04)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(len(n.Str("institution_code")) <= 5 || !n.Has("institution_name") || n.Str("institution_name") == n.Str("institution_code"))
		}},
		{Code: "REJ-ACC05", Context: ctx, Message: "Un solde exige la devise du compte (REJ-ACC05)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("balance") || n.Has("currency_code"))
		}},
		{Code: "REJ-ACC06", Context: ctx, Message: "branch n'est permis qu'avec un SWIFT de 11 caractères (REJ-ACC06)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("branch") || len(n.Str("swift")) == 11 || !n.Has("swift"))
		}},
	}
}

func luRules() []format.Rule {
	reportCode := func(doc *format.Node) string { return doc.Str("report_code") }
	in := func(v string, set ...string) bool {
		for _, s := range set {
			if v == s {
				return true
			}
		}
		return false
	}
	rules := []format.Rule{
		{Code: "REJ-REA01", Message: "La motivation (reason) ne doit pas être vide (REJ-REA01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("reason") || strings.TrimSpace(n.Str("reason")) != "" || in(reportCode(n), "XBD", "XBR"))
		}},
		{Code: "REJ-REA02", Message: "La motivation (reason) est obligatoire sauf RIRA/RIRT (REJ-REA02)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Has("reason") || in(reportCode(n), "XBD", "XBR", "RIRA", "RIRT"))
		}},
		{Code: "REJ-IND01", Message: "Au moins un indicateur est obligatoire pour STR, SAR, STRe, SARe, TFTR, TFAR (REJ-IND01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Has("report_indicators") || !in(reportCode(n), "STR", "SAR", "STRe", "SARe", "TFTR", "TFAR"))
		}},
		{Code: "REJ-TRS01", Message: "Chaque numéro de transaction doit être unique (REJ-TRS01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			seen := map[string]bool{}
			for _, v := range childValues(n, "transaction/transactionnumber") {
				if seen[v] {
					return []format.Finding{{Message: "Numéro de transaction en double : " + v + " (REJ-TRS01)"}}
				}
				seen[v] = true
			}
			return nil
		}},
		{Code: "REJ-TRS02", Message: "500 transactions au maximum par déclaration (REJ-TRS02)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(len(n.All("transaction")) < 501 || in(reportCode(n), "XBD", "XBR"))
		}},
		{Code: "REJ-TRS03", Context: "report/transaction", Message: "Un virement (Q) exige des comptes des deux côtés (REJ-TRS03)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			from := n.Str("t_from/from_funds_code") + n.Str("t_from_my_client/from_funds_code")
			to := n.Str("t_to/to_funds_code") + n.Str("t_to_my_client/to_funds_code")
			okFrom := from != "Q" || n.Has("t_from/from_account") || n.Has("t_from_my_client/from_account")
			okTo := to != "Q" || n.Has("t_to/to_account") || n.Has("t_to_my_client/to_account")
			return one(okFrom && okTo)
		}},
		{Code: "REJ-TRS04", Context: "report/transaction", Message: "funds_code doit être identique au débit et au crédit (REJ-TRS04)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			from := n.Str("t_from/from_funds_code") + n.Str("t_from_my_client/from_funds_code")
			to := n.Str("t_to/to_funds_code") + n.Str("t_to_my_client/to_funds_code")
			return one(from == to)
		}},
		{Code: "REJ-ENT01", Context: "entity_related_person_type", Message: "Une personne morale ne peut être liée à une entité elle-même liée (REJ-ENT01)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("entity/related_entities"))
		}},
		{Code: "LU-BR-STR-SAR", Message: "Règle métier CRF : un STR contient des transactions, un SAR n'en contient pas", Source: "FAQ CRF reglesbus", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			tx := len(n.All("transaction")) > 0
			rc := reportCode(n)
			return one(!(in(rc, "STR", "TFTR", "STRe") && !tx) && !(in(rc, "SAR", "TFAR", "SARe") && tx))
		}},
	}
	for _, ctx := range []string{"t_account", "t_account_my_client"} {
		rules = append(rules, accountRules(ctx)...)
	}
	for _, ctx := range []string{"t_entity", "t_entity_my_client"} {
		rules = append(rules, format.Rule{Code: "REJ-ENT02", Context: ctx, Message: "Numéro RCS luxembourgeois mal formaté : B123456 ou F123, sans zéro initial ni espace (REJ-ENT02)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("incorporation_country_code") != "LU" || !n.Has("incorporation_number") || rcsRe.MatchString(n.Str("incorporation_number")))
		}})
	}
	return rules
}

// luPrepare computes the 008SA amount band indicator.
func luPrepare(in sar.Input, out models.SarExtension) {
	var amounts []string
	for _, op := range sar.SelectedOperations(in.Pivot) {
		if v, _, ok := sar.Convert(sar.Abs(op.Amount.Val()), op.Currency.Val(), "EUR", in.Rates); ok {
			amounts = append(amounts, v)
		}
	}
	total, _ := sar.Sum(amounts)
	if total == "" {
		total = "0"
	}
	band := amountBand(total)
	out["amount_band"] = models.SarExtValue{Values: []string{band}, P: sar.Calc(fmt.Sprintf("tranche calculée sur le total de %s EUR des %d transactions retenues", total, len(amounts)))}
}

// ---------------------------------------------------------------- CH

func Switzerland() *Profile {
	s := chSchema()
	s.CodeLists["rentity_type"] = &format.CodeList{Name: "rentity_type", Codes: []format.Code{{Value: "RE", Label: "Reporting entity"}, {Value: "SH", Label: "Shareholder"}, {Value: "FIU", Label: "FIU"}, {Value: "SUP", Label: "Supervisor"}, {Value: "IND", Label: "Individual"}}}
	p := &Profile{
		Desc:        sar.Descriptor{Key: KeyCH, Format: "goaml", Country: "CH", Version: "5.0.2 (Control XSD 2025.3, guide CH 2.0.2)", Label: "goAML Suisse (MROS)", FIU: "MROS (fedpol)", Currency: "CHF", Packaging: "zip", Timezone: "Europe/Zurich"},
		Schema:      s,
		TZ:          loc("Europe/Zurich"),
		MaxTx:       100,
		DateElement: "report_date",
		Ext: commonExt(
			sar.ExtField{Key: "indicators", Scope: "report", Label: "Indicateurs MROS (M base légale, V infraction préalable, G motif, B pièces)", Kind: "code", CodeList: "report_indicator_type", Min: 3, Max: -1, AI: true,
				Instructions: "Exactement un indicateur M (base légale : 0002M art. 9 al. 1 let. a LBA pour un soupçon fondé, 0001M art. 305ter al. 2 CP pour un droit de communication, 0003M uniquement pour un SAR après rupture des négociations), au moins un V (infraction préalable, ex. 1004V escroquerie, 1130V délit fiscal qualifié, 1014V financement du terrorisme, 1206V interdit) et au moins un G (motif du soupçon, ex. 2001G espèces, 2018G monitoring, 2013G pays à risque). Ajoutez les B correspondant aux pièces jointes (3007B relevés de compte, 3012B KYC, 3016B autres). Codes interdits aux intermédiaires : 1134V-1137V, 1165V, 1206V, 2026G-2029G, 3019B-3022B."},
			sar.ExtField{Key: "transaction_type_code", Scope: "operation", Label: "Type de transaction MROS (transaction_type_code)", Kind: "code", CodeList: "transaction_type", Max: 1, AI: true},
			sar.ExtField{Key: "location_address", Scope: "settings", Label: "Adresse de l'entité déclarante (rue)", Kind: "text", Min: 1, Max: 1},
			sar.ExtField{Key: "location_city", Scope: "settings", Label: "Ville de l'entité déclarante", Kind: "text", Min: 1, Max: 1},
			sar.ExtField{Key: "domicile_company", Scope: "party", Label: "Société de domicile (tax_reg_number MROS : Oui/Non)", Kind: "text", Max: 1, AI: true,
				Instructions: "Pour chaque personne morale cliente : yes si c'est une société de domicile (sans activité opérationnelle en Suisse, adresse chez un domiciliataire), no sinon. Uniquement si les sources l'établissent."},
			sar.ExtField{Key: "location_zip", Scope: "settings", Label: "NPA de l'entité déclarante", Kind: "text", Max: 1},
			sar.ExtField{Key: "account_branch_canton", Scope: "settings", Label: "Canton de la succursale teneuse des comptes (branch)", Kind: "text", Min: 1, Max: 1},
			sar.ExtField{Key: "business_relationships", Scope: "report", Label: "Nombre de relations d'affaires déclarées (BRNR)", Kind: "text", Max: 1, Computed: true},
		),
		Instructions: "Format goAML du MROS (Suisse), guide CH 2.0.2. Le backend choisit STR (au moins une transaction bi-partie, 100 au plus) ou SAR (aucune transaction, ou indicateur 0003M), convertit en CHF, calcule BRNR (nombre de relations d'affaires) et les dates. Vous remplissez les indicateurs M/V/G/B et, si besoin, transaction_type_code et funds_code par transaction. reason (faits) et action (motifs du soupçon et mesures) sont limités à 8 000 caractères.",
		Funds: func(op *models.SarOperation) string {
			switch op.Type.Val() {
			case "cash_deposit", "cash_withdrawal":
				return "26"
			case "cheque":
				return "23"
			case "crypto":
				return "2"
			}
			return "27"
		},
		Transmode: func(op *models.SarOperation) string {
			return map[string]string{"branch": "A", "atm": "B", "online": "C", "mobile": "C", "other": "F"}[op.Channel.Val()]
		},
		TxType: func(op *models.SarOperation) string {
			switch op.Type.Val() {
			case "transfer", "direct_debit":
				return "B2BWT"
			case "cash_deposit":
				return "DEPAC"
			case "cash_withdrawal":
				return "WITHD"
			case "card_payment":
				return "POSAL"
			case "crypto":
				return "VASIO"
			case "money_remittance":
				return "CASHT"
			}
			return "B2BWT"
		},
		AccountType:   map[string]string{"bank_account": "2", "card": "10", "prepaid_card": "10", "e_wallet": "14", "crypto_wallet": "18", "contract": "12", "other": "14"},
		AccountStatus: map[string]string{"active": "1", "closed": "2", "dormant": "3", "blocked": "1"},
		Gender:        map[string]string{"male": "M", "female": "W", "unknown": "U", "other": "U"},
		IDType:        map[string]string{"driving_licence": "1", "id_card": "2", "passport": "3", "residence_permit": "5", "other": "4"},
		LegalForm:     map[string]string{"company": "8", "partnership": "11", "foundation": "14", "association": "2", "trust": "5", "sole_trader": "10", "public_body": "18", "other": "4"},
		AddressType:   map[string]string{"home": "3", "business": "4", "other": "2"},
		PhoneType:     [2]string{"3", "1"},
		EntityRole:    map[string]string{"director": "6", "beneficial_owner": "3", "shareholder": "13"},
		AccountRole:   map[string]string{"holder": "14", "co_holder": "14", "signatory": "15", "beneficial_owner": "13"},
		PartyRole: func(p *models.SarParty) string {
			switch {
			case sar.HasRole(p, "beneficial_owner"):
				return "1"
			case sar.HasRole(p, "signatory"):
				return "2"
			case isClient(p):
				return "0"
			}
			return "10"
		},
		ForeignAlways: true,
		ReportType: func(in sar.Input, ops []*models.SarOperation) (string, string) {
			for _, v := range in.Values("indicators") {
				if v == "0003M" {
					return "SAR", "indicateur 0003M (rupture des négociations) : SAR"
				}
			}
			if len(ops) > 0 {
				return "STR", "déclaration avec au moins une transaction bi-partie"
			}
			return "SAR", "déclaration sans transaction"
		},
		Rules: chRules(),
	}
	p.EntityHook = chEntityHook
	p.PrepareHook = chPrepare
	p.ReportHook = chReportHook
	p.AccountHook = chAccountHook
	p.Require = chRequire
	return p
}

func chReportHook(in sar.Input, ctx *Ctx, r *format.Node) {
	settings := func(field string) *format.Meta {
		return &format.Meta{Edit: "settings/" + KeyCH + "/" + field, Provenance: &models.SarProvenance{Kind: models.SarProvSettings}}
	}
	l := r.Add("location")
	l.Leaf("address_type", "4", sar.DerivedMeta("adresse professionnelle de l'entité déclarante", ""))
	l.Leaf("address", in.Setting("location_address"), settings("location_address"))
	l.Leaf("city", in.Setting("location_city"), settings("location_city"))
	l.Leaf("zip", in.Setting("location_zip"), settings("location_zip"))
	l.Leaf("country_code", "CH", sar.DerivedMeta("entité déclarante suisse", ""))
	if !r.Has("action") {
		r.Leaf("action", in.Pivot.Suspicion.Analysis.Val(), &format.Meta{Edit: "pivot/suspicion/actions_taken", Derived: "action obligatoire (MROS) : reprend l'analyse faute de mesures saisies"})
	}
	if n := in.Value("business_relationships"); n != "" && n != "0" {
		ai := r.Add("additional_information").Add("additional_info")
		ai.Leaf("info_type", "BRNR", sar.DerivedMeta("assert MROS n°12", ""))
		ai.Leaf("info_numeric", n, sar.ExtMeta(KeyCH, "business_relationships", in.Ext["business_relationships"]))
	}
}

func chAccountHook(in sar.Input, ctx *Ctx, x *models.SarInstrument, acc *format.Node, my bool) {
	pre := "pivot/instruments/" + x.ID
	if !my {
		if !acc.Has("institution_name") {
			acc.Leaf("institution_name", "N/A", sar.DerivedMeta("guide MROS 3.7 : N/A pour un texte obligatoire inconnu", pre+"/institution"))
		}
		acc.Leaf("comments", "Compte de la contrepartie", sar.DerivedMeta("commentaire obligatoire (t_account MROS)", ""))
		return
	}
	if !acc.Has("branch") {
		acc.Leaf("branch", in.Setting("account_branch_canton"), &format.Meta{Edit: "settings/" + KeyCH + "/account_branch_canton", Provenance: &models.SarProvenance{Kind: models.SarProvSettings}, Derived: "canton de la succursale teneuse du compte"})
	}
	cat := "ACCNT"
	switch x.Kind.Val() {
	case "crypto_wallet":
		cat = "VADDR"
	case "e_wallet":
		cat = "OTHER"
	}
	acc.Leaf("account_category", cat, sar.DerivedMeta("catégorie déduite du type de support", pre+"/kind"))
	acc.Leaf("client_number", firstNonEmpty(x.Reference.Val(), x.ID), sar.DerivedMeta("numéro de client", pre+"/reference"))
	// related_persons (assert 17) mirror the signatories.
	if sigs := acc.All("signatory"); len(sigs) > 0 {
		rp := acc.Add("related_persons")
		for _, sg := range sigs {
			arp := rp.Add("account_related_person")
			arp.Children = append(arp.Children, sg.Children...)
		}
		var kept []*format.Node
		for _, c := range acc.Children {
			if c.Name != "signatory" {
				kept = append(kept, c)
			}
		}
		acc.Children = kept
	}
	comment := "Compte en CHF"
	if x.Currency.Val() != "" && x.Currency.Val() != "CHF" && x.Status.Val() != "closed" && x.Balance.Val() != "" {
		acc.Leaf("beneficiary", x.Balance.Val(), sar.DerivedMeta("solde en devise du compte (assert MROS n°18)", pre+"/balance"))
		comment = "Solde en " + x.Currency.Val()
	}
	acc.Leaf("beneficiary_comment", comment, sar.DerivedMeta("commentaire obligatoire du solde en devise (MROS)", pre+"/currency"))
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// chEntityHook fills the diverted tax_reg_number (domicile company yes/no).
func chEntityHook(in sar.Input, p *models.SarParty, n *format.Node, my bool) {
	v := in.Value(sar.ExtKey("domicile_company", p.ID))
	label := map[string]string{"yes": "Yes", "no": "No"}[strings.ToLower(v)]
	if label != "" {
		n.Leaf("tax_reg_number", label, sar.ExtMeta(KeyCH, sar.ExtKey("domicile_company", p.ID), in.Ext[sar.ExtKey("domicile_company", p.ID)]))
	}
}

func chRequire(in sar.Input) []models.SarIssue {
	var out []models.SarIssue
	for _, p := range in.Pivot.Parties {
		if p.Kind.Val() != "entity" || !isClientOrHolder(in, &p) {
			continue
		}
		pre := "pivot/parties/" + p.ID
		if p.LegalForm.Val() == "" {
			out = append(out, models.SarIssue{Path: pre + "/legal_form", Code: "MROS-LEGAL_FORM", Message: "Personne morale cliente : forme juridique obligatoire (incorporation_legal_form)", Severity: "error", Layer: "pivot", Edit: pre + "/legal_form"})
		}
		if v := strings.ToLower(in.Value(sar.ExtKey("domicile_company", p.ID))); v != "yes" && v != "no" {
			path := "extension/" + KeyCH + "/" + sar.ExtKey("domicile_company", p.ID)
			out = append(out, models.SarIssue{Path: path, Code: "MROS-DOMICILE", Message: "Personne morale cliente : indiquer s'il s'agit d'une société de domicile (tax_reg_number Oui/Non)", Severity: "error", Layer: "extension", Edit: path})
		}
		if len(p.Addresses) == 0 {
			out = append(out, models.SarIssue{Path: pre + "/addresses", Code: "MROS-ADDRESS", Message: "Personne morale cliente : adresse obligatoire", Severity: "error", Layer: "pivot", Edit: pre + "/addresses"})
		}
	}
	for _, p := range in.Pivot.Parties {
		if p.Kind.Val() == "person" && isClient(&p) && len(p.Identifications) == 0 {
			path := "pivot/parties/" + p.ID + "/identifications"
			out = append(out, models.SarIssue{Path: path, Code: "MROS-ID", Message: "Personne cliente : une pièce d'identité est obligatoire (t_person identification, MROS)", Severity: "error", Layer: "pivot", Edit: path})
		}
	}
	for _, x := range in.Pivot.Instruments {
		if !heldHere(&x) {
			continue
		}
		pre := "pivot/instruments/" + x.ID
		if x.Status.Val() != "closed" && (x.Balance.Val() == "" || x.BalanceDate.Val() == "") {
			out = append(out, models.SarIssue{Path: pre + "/balance", Code: "MROS-A14", Message: "Compte ouvert : solde et date du solde obligatoires (assert MROS n°14)", Severity: "error", Layer: "pivot", Edit: pre + "/balance"})
		}
		if x.BIC.Val() == "" {
			out = append(out, models.SarIssue{Path: pre + "/bic", Code: "MROS-BIC", Message: "BIC/SWIFT obligatoire pour un compte bancaire (guide MROS 7.1)", Severity: "error", Layer: "pivot", Edit: pre + "/bic"})
		}
		if x.Status.Val() == "" {
			out = append(out, models.SarIssue{Path: pre + "/status", Code: "MROS-STATUS", Message: "Statut du compte obligatoire (status_code)", Severity: "error", Layer: "pivot", Edit: pre + "/status"})
		}
		for _, h := range holders(in, &x) {
			if h.Party.Kind.Val() != "person" {
				continue
			}
			pp := "pivot/parties/" + h.Party.ID
			if h.Party.Gender.Val() == "" {
				out = append(out, models.SarIssue{Path: pp + "/gender", Code: "MROS-PERSON_GENDER", Message: "Personne liée à un compte client : sexe obligatoire (gender, « unknown » accepté)", Severity: "error", Layer: "pivot", Edit: pp + "/gender"})
			}
			if len(h.Party.Nationalities) == 0 {
				out = append(out, models.SarIssue{Path: pp + "/nationalities", Code: "MROS-PERSON_NATIONALITY", Message: "Personne liée à un compte client : nationalité obligatoire (nationality1)", Severity: "error", Layer: "pivot", Edit: pp + "/nationalities"})
			}
		}
		if x.Opened.Val() == "" {
			out = append(out, models.SarIssue{Path: pre + "/opened", Code: "MROS-OPENED", Message: "Date d'ouverture obligatoire pour un compte my_client", Severity: "error", Layer: "pivot", Edit: pre + "/opened"})
		}
	}
	return out
}

// isClientOrHolder: client parties and holders of accounts held at the reporter
// are projected as my_client in MROS reports.
func isClientOrHolder(in sar.Input, p *models.SarParty) bool {
	if isClient(p) {
		return true
	}
	for _, l := range in.Pivot.Links {
		if l.PartyID.Val() != p.ID {
			continue
		}
		for _, x := range in.Pivot.Instruments {
			if x.ID == l.InstrumentID.Val() && heldHere(&x) {
				return true
			}
		}
	}
	return false
}

// chPrepare computes BRNR: the number of reported business relationships.
func chPrepare(in sar.Input, out models.SarExtension) {
	n := 0
	for _, p := range in.Pivot.Parties {
		if isClient(&p) && p.Relationship.Val() != "occasional_customer" && p.Relationship.Val() != "prospect" {
			n++
		}
	}
	out["business_relationships"] = models.SarExtValue{Values: []string{strconv.Itoa(n)}, P: sar.Calc(fmt.Sprintf("%d partie(s) cliente(s) en relation d'affaires dans le pivot", n))}
}

func chRules() []format.Rule {
	rc := func(doc *format.Node) string { return doc.Str("report_code") }
	count := func(n *format.Node, suffix string) int {
		c := 0
		for _, v := range childValues(n, "indicator") {
			if strings.HasSuffix(v, suffix) {
				c++
			}
		}
		return c
	}
	hasInd := func(doc *format.Node, code string) bool {
		for _, v := range childValues(doc, "report_indicators/indicator") {
			if v == code {
				return true
			}
		}
		return false
	}
	return []format.Rule{
		{Code: "MROS-A01", Context: "report/transaction", Message: "Le montant de la transaction doit être supérieur à 0 (assert MROS n°1)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(sar.DecimalCmp(n.Str("amount_local"), "0") != 0 || n.Has("involved_parties"))
		}},
		{Code: "MROS-A04", Context: "report/transaction", Message: "Transaction bi-partie : au moins un côté doit être my_client (assert MROS n°4)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Has("involved_parties") || n.Has("t_from_my_client") || n.Has("t_to_my_client"))
		}},
		{Code: "MROS-A05", Context: "report/report_indicators", Message: "Exactement un indicateur de type M (assert MROS n°5)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(count(n, "M") == 1)
		}},
		{Code: "MROS-A06", Context: "report/report_indicators", Message: "Au moins un indicateur de type V (assert MROS n°6)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(count(n, "V") > 0)
		}},
		{Code: "MROS-A07", Context: "report/report_indicators", Message: "Au moins un indicateur de type G (assert MROS n°7)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(count(n, "G") > 0)
		}},
		{Code: "MROS-A08", Message: "CANCL/CANCT : fiu_ref_number obligatoire (assert MROS n°8)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!(rc(n) == "CANCL" || rc(n) == "CANCT") || strings.TrimSpace(n.Str("fiu_ref_number")) != "")
		}},
		{Code: "MROS-A09", Message: "CANCL/CANCT : indicateurs 0024M, 1207V et 2103G obligatoires (assert MROS n°9)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!(rc(n) == "CANCL" || rc(n) == "CANCT") || hasInd(n, "0024M") && hasInd(n, "1207V") && hasInd(n, "2103G"))
		}},
		{Code: "MROS-A10", Message: "L'indicateur 0003M n'est permis que pour un SAR (assert MROS n°10)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!hasInd(n, "0003M") || rc(n) == "SAR")
		}},
		{Code: "MROS-A11", Message: "Un STR/AIFT doit contenir au moins une transaction bi-partie (assert MROS n°11)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			if rc(n) != "STR" && rc(n) != "AIFT" {
				return nil
			}
			for _, t := range n.All("transaction") {
				if t.Has("t_from_my_client") || t.Has("t_to_my_client") || t.Has("t_from") || t.Has("t_to") {
					return nil
				}
			}
			return one(false)
		}},
		{Code: "MROS-A12", Message: "STR/SAR : indiquer le nombre de relations d'affaires (additional_info BRNR > 0, assert MROS n°12)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			if hasInd(n, "0003M") || (rc(n) != "SAR" && rc(n) != "STR") {
				return nil
			}
			for _, ai := range n.Get("additional_information").All("additional_info") {
				if ai.Str("info_type") == "BRNR" && sar.DecimalCmp(ai.Str("info_numeric"), "0") > 0 {
					return nil
				}
			}
			return one(false)
		}},
		{Code: "MROS-A13", Context: "t_account_my_client", Message: "La date du solde d'un compte actif doit dater de moins de 14 jours (assert MROS n°13)", Check: func(n *format.Node, _ string, doc *format.Node) []format.Finding {
			if n.Str("account_category") != "ACCNT" || n.Str("status_code") == "2" {
				return nil
			}
			// The XSD compares with current-dateTime(); the report date (generation
			// date of the projection) keeps validation deterministic.
			zurich := loc("Europe/Zurich")
			t, err := time.ParseInLocation("2006-01-02T15:04:05", n.Str("date_balance"), zurich)
			ref, err2 := time.ParseInLocation("2006-01-02T15:04:05", doc.Str("report_date"), zurich)
			if err2 != nil {
				ref = time.Now()
			}
			return one(err == nil && !t.Before(ref.Add(-14*24*time.Hour)))
		}},
		{Code: "MROS-A14", Context: "t_account_my_client", Message: "Un compte ouvert doit avoir un solde, même nul (assert MROS n°14)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("status_code") == "2" || n.Has("balance"))
		}},
		{Code: "MROS-A15", Context: "t_account_my_client", Message: "Un compte clôturé doit avoir sa date de clôture (assert MROS n°15)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("status_code") != "2" || n.Has("closed"))
		}},
		{Code: "MROS-A16", Context: "t_account_my_client", Message: "Le solde d'un compte clôturé doit être nul ou vide (assert MROS n°16)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("status_code") != "2" || !n.Has("balance") || sar.DecimalCmp(n.Str("balance"), "0") == 0)
		}},
		{Code: "MROS-A17", Context: "t_account_my_client", Message: "Indiquer les personnes liées au compte : titulaire, ayant droit, mandataires (assert MROS n°17)", Check: func(n *format.Node, _ string, doc *format.Node) []format.Finding {
			r := rc(doc)
			return one(!(r == "STR" || r == "SAR" || r == "AIF" || r == "AIFT") || n.Has("related_persons"))
		}},
		{Code: "MROS-A18", Context: "t_account_my_client", Message: "Compte ouvert hors CHF : solde en devise (beneficiary) obligatoire (assert MROS n°18)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("currency_code") == "CHF" || n.Has("closed") || n.Has("beneficiary") || n.Str("account_type") == "17")
		}},
		{Code: "MROS-A19", Context: "t_account_my_client", Message: "Une date de clôture n'est permise qu'avec status_code 2 (assert MROS n°19)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!n.Has("closed") || n.Str("status_code") == "2")
		}},
		{Code: "MROS-A20", Message: "SAR : au moins une personne, un compte ou une entité my_client (assert MROS n°20)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			if rc(n) != "SAR" {
				return nil
			}
			for _, rp := range n.Get("activity/report_parties").All("report_party") {
				if rp.Has("person_my_client") || rp.Has("account_my_client") || rp.Has("entity_my_client") {
					return nil
				}
			}
			return one(false)
		}},
		{Code: "MROS-SIGN", Context: "report/transaction", Message: "Montant local et montants en devise doivent être de même signe (assert XSD MROS)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			for _, side := range []string{"t_from_my_client/from_foreign_currency", "t_from/from_foreign_currency", "t_to_my_client/to_foreign_currency", "t_to/to_foreign_currency"} {
				if f := n.Get(side + "/foreign_amount"); f != nil && sar.DecimalCmp(n.Str("amount_local"), "0")*sar.DecimalCmp(f.Value(), "0") < 0 {
					return one(false)
				}
			}
			return nil
		}},
		{Code: "MROS-LEN", Message: "reason et action sont limités à 8 000 caractères (guide MROS 6.1)", Check: func(n *format.Node, path string, _ *format.Node) []format.Finding {
			var out []format.Finding
			for _, f := range []string{"reason", "action"} {
				if len([]rune(n.Str(f))) > 8000 {
					out = append(out, format.Finding{Path: path + "/" + f + "[1]"})
				}
			}
			return out
		}},
		{Code: "MROS-A22", Context: "t_trans_item", Message: "Type ou statut « autre » : commentaire obligatoire (assert MROS n°22)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(!(n.Str("status_code") == "9" || n.Str("item_type") == "2") || strings.TrimSpace(n.Str("comments")) != "")
		}},
		{Code: "MROS-A23", Context: "t_trans_item", Message: "Bien immobilier : adresse obligatoire (assert MROS n°23)", Check: func(n *format.Node, _ string, _ *format.Node) []format.Finding {
			return one(n.Str("item_type") != "7" || n.Has("address"))
		}},
	}
}
