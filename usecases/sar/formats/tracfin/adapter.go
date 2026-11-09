// Package tracfin is the TRACFIN (France) adapter, module financier 1.4.0.
// The pivot's unit operations are aggregated into at most 30 lines in EUR;
// persons, financial references and attachments get declaration-local IDs;
// attachments are embedded in the XML as base64.
package tracfin

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

const Key = "tracfin:FR:1.4.0"

func init() { sar.Register(New()) }

type Adapter struct{ schema *format.Schema }

func New() *Adapter {
	s := generatedSchema()
	// Labels of inline enumerations documented in the XSD comments.
	for _, id := range []string{"/ds/personnes/pp/activite/numeroIdentification/@type", "/ds/personnes/pm/activiteCoordonnee/numeroIdentification/@type", "/ds/personnes/pm/dirigeant/autreDirigeantM/numeroIdentification/@type"} {
		if l := s.CodeLists[id]; l != nil {
			labels := map[string]string{"SIREN": "SIREN", "RN1": "RIDET (Registre INSEE Nouvelle-Calédonie)", "RN2": "COM", "RN3": "RNA (Registre national des associations)", "RN4": "RNM (Répertoire national des métiers)", "RN5": "LEI (Legal Entity Identifier)", "RN6": "Registre étranger", "RN7": "Autre registre", "Je ne sais pas": "Je ne sais pas", "Non": "Non"}
			for i := range l.Codes {
				if v, ok := labels[l.Codes[i].Value]; ok {
					l.Codes[i].Label = v
				}
			}
		}
	}
	s.Rules = rules()
	return &Adapter{schema: s}
}

func (a *Adapter) Descriptor() sar.Descriptor {
	return sar.Descriptor{Key: Key, Format: "tracfin", Country: "FR", Version: "financier 1.4.0 (2026-01-21)", Label: "TRACFIN (France), module financier", FIU: "TRACFIN", Currency: "EUR", Packaging: "xml_base64_attachments", Timezone: "Europe/Paris"}
}
func (a *Adapter) Schema() *format.Schema          { return a.schema }
func (a *Adapter) Hint(norm, parent string) string { return parent }

func (a *Adapter) Instructions() string {
	return "Déclaration de soupçon TRACFIN (ERMES, module financier). Le backend agrège les opérations unitaires du pivot en 30 lignes au plus, en EUR, calcule période, montants, nombres d'opérations, identifiants locaux et dates JJ/MM/AAAA. Vous remplissez uniquement : le cadre légal (CL01 terrorisme, CL02 infraction pénale, CL03 fraude fiscale ; au moins un des trois ; CL04 examen renforcé et CL05 tentative en complément), les natures d'infraction pénale (obligatoires si CL02) et de fraude fiscale (obligatoires si CL03), leurs précisions « autre » (NSP15, NFF10), le type de document de chaque pièce jointe et la fonction PPE des personnes exposées."
}

func (a *Adapter) Extension() []sar.ExtField {
	return []sar.ExtField{
		{Key: "cadre_legal", Scope: "report", Label: "Cadre légal (naturessoupcons)", Kind: "code", CodeList: "naturessoupcons.csv", Min: 1, Max: 5, AI: true, Instructions: "Au moins un code CL01, CL02 ou CL03. CL01 si les thèmes contiennent le financement du terrorisme, CL03 si fraude fiscale, CL02 pour toute autre infraction pénale. Ajouter CL05 si une opération a été tentée ou refusée."},
		{Key: "nature_soupcon_penale", Scope: "report", Label: "Nature de l'infraction pénale", Kind: "code", CodeList: "violationspenaltype.csv", Max: 30, AI: true, Instructions: "Obligatoire si CL02, interdit sinon."},
		{Key: "autre_nature_soupcon_penale", Scope: "report", Label: "Autre infraction pénale (précision)", Kind: "text", MaxLength: 200, Max: 1, AI: true, Instructions: "Uniquement avec NSP15."},
		{Key: "nature_fraude_fiscale", Scope: "report", Label: "Nature de la fraude fiscale", Kind: "code", CodeList: "violationstypefiscal.csv", Max: 30, AI: true, Instructions: "Obligatoire si CL03, interdit sinon."},
		{Key: "autre_nature_fraude_fiscale", Scope: "report", Label: "Autre fraude fiscale (précision)", Kind: "text", MaxLength: 200, Max: 1, AI: true, Instructions: "Uniquement avec NFF10."},
		{Key: "ref_interne", Scope: "report", Label: "Référence interne", Kind: "text", Min: 1, Max: 1, Computed: true},
		{Key: "complement", Scope: "report", Label: "Référence TRACFIN de la déclaration complétée", Kind: "text", Max: 1, Computed: true},
		{Key: "precision_signalement", Scope: "report", Label: "Précision du signalement", Kind: "text", MaxLength: 700, Max: 1},
		{Key: "gestion_fortune", Scope: "report", Label: "Gestion de fortune / banque privée", Kind: "boolean", Max: 1},
		{Key: "correspondance_bancaire", Scope: "report", Label: "Correspondance bancaire", Kind: "boolean", Max: 1},
		{Key: "operation_type", Scope: "operation", Label: "Type d'opération TRACFIN", Kind: "code", CodeList: "typesoperations.csv", Max: 1, AI: true, Instructions: "Code typesoperations ; une valeur est déduite du type neutre de l'opération."},
		{Key: "document_type", Scope: "attachment", Label: "Type de pièce jointe", Kind: "code", CodeList: "typesdedocuments.csv", Max: 1, AI: true, Instructions: "Code typesdedocuments selon le contenu du document (TA17 interdit)."},
		{Key: "ppe_function", Scope: "party", Label: "Fonction PPE", Kind: "code", CodeList: "fonctionsppe.csv", Max: 20, AI: true, Instructions: "Uniquement pour une personne dont pep = yes."},
		{Key: "organisme_profession", Scope: "settings", Label: "Profession de l'organisme déclarant", Kind: "code", CodeList: "professions.csv", Min: 1, Max: 1},
		{Key: "organisme_libelle", Scope: "settings", Label: "Dénomination de l'organisme", Kind: "text", Min: 1, Max: 1},
		{Key: "organisme_num_id_pro", Scope: "settings", Label: "Identifiant professionnel (SIREN, CIB...)", Kind: "text", Min: 1, Max: 1},
		{Key: "organisme_voie", Scope: "settings", Label: "Adresse de l'organisme (voie)", Kind: "text", Min: 1, Max: 1},
		{Key: "organisme_code_postal", Scope: "settings", Label: "Code postal", Kind: "text", Min: 1, Max: 1},
		{Key: "organisme_ville", Scope: "settings", Label: "Ville", Kind: "text", Min: 1, Max: 1},
		{Key: "organisme_tel", Scope: "settings", Label: "Téléphone de l'organisme", Kind: "text", Min: 1, Max: 1},
		{Key: "declarant_nom", Scope: "settings", Label: "Nom du déclarant", Kind: "text", Min: 1, Max: 1},
		{Key: "declarant_prenom", Scope: "settings", Label: "Prénom du déclarant", Kind: "text", Min: 1, Max: 1},
		{Key: "declarant_tel", Scope: "settings", Label: "Téléphone du déclarant", Kind: "text", Min: 1, Max: 1},
		{Key: "declarant_courriel", Scope: "settings", Label: "Courriel du déclarant", Kind: "text", Min: 1, Max: 1},
		{Key: "declarant_fonction", Scope: "settings", Label: "Fonction du déclarant", Kind: "text", Max: 1},
	}
}

var opTypes = map[string]string{"transfer": "OT10", "cash_deposit": "OT07", "cash_withdrawal": "OT06", "card_payment": "OT03", "cheque": "OT04", "crypto": "OT11", "direct_debit": "OT10", "money_remittance": "OT11", "other": "OT11"}
var docTypes = map[string]string{"identity": "TA8", "statement": "TA12", "contract": "TA14", "correspondence": "TA19", "invoice": "TA14", "screening": "TA11", "other": "TA19"}

func (a *Adapter) Prepare(in sar.Input) models.SarExtension {
	out := models.SarExtension{}
	ref := strings.ToUpper(strings.ReplaceAll(in.Pivot.CaseID, "-", ""))
	out["ref_interne"] = models.SarExtValue{Values: []string{"MARBLE-" + ref[:min(16, len(ref))]}, P: sar.Calc("identifiant du case")}
	for _, pr := range in.Pivot.PriorReports {
		if strings.EqualFold(pr.FIU.Val(), "TRACFIN") && pr.Relation.Val() == "complement" && regexp.MustCompile(`^\d{1,19}$`).MatchString(pr.Reference.Val()) {
			out["complement"] = models.SarExtValue{Values: []string{pr.Reference.Val()}, P: sar.Calc("déclaration TRACFIN antérieure complétée " + pr.ID)}
		}
	}
	def := func(key, value, why string) {
		if value == "" {
			return
		}
		if cur, ok := in.Ext[key]; ok && cur.P != nil && cur.P.Kind != models.SarProvCalculated {
			return
		}
		out[key] = models.SarExtValue{Values: []string{value}, P: sar.Calc(why)}
	}
	for _, op := range sar.SelectedOperations(in.Pivot) {
		def(sar.ExtKey("operation_type", op.ID), opTypes[op.Type.Val()], "déduit du type d'opération "+op.Type.Val())
	}
	for _, at := range sar.SelectedAttachments(in.Pivot) {
		def(sar.ExtKey("document_type", at.ID), docTypes[at.DocumentType.Val()], "déduit du type de document "+at.DocumentType.Val())
	}
	return out
}

func (a *Adapter) Requirements(in sar.Input) []models.SarIssue {
	var out []models.SarIssue
	req := func(path, code, msg string) {
		out = append(out, models.SarIssue{Path: path, Code: code, Message: msg, Severity: "error", Layer: "pivot", Edit: path})
	}
	if in.Pivot.Suspicion.Analysis.Val() == "" {
		req("pivot/suspicion/analysis", "REQ_ANALYSIS", "L'analyse est obligatoire (champ analyse TRACFIN)")
	}
	if len(in.Pivot.Parties) == 0 {
		req("pivot/parties", "REQ_PERSON", "Au moins une personne doit être déclarée")
	}
	for _, op := range sar.SelectedOperations(in.Pivot) {
		pp := "pivot/operations/" + op.ID
		if op.Amount.Val() == "" || op.Currency.Val() == "" {
			req(pp+"/amount", "REQ_OPERATION_AMOUNT", "Montant et devise requis pour l'agrégation en EUR")
		} else if _, _, ok := sar.Convert("1", op.Currency.Val(), "EUR", in.Rates); !ok {
			req(pp+"/currency", "REQ_RATE", "Taux "+op.Currency.Val()+"/EUR non configuré : conversion impossible")
		}
		if op.Timestamp.Val() == "" {
			req(pp+"/timestamp", "REQ_OPERATION_DATE", "Date requise pour la période des opérations")
		}
	}
	for _, at := range sar.SelectedAttachments(in.Pivot) {
		if at.Mime.Val() != "application/pdf" {
			req("pivot/attachments/"+at.ID+"/selected", "REQ_ATTACHMENT_MIME", "Seules les pièces PDF sont jointes dans ce POC (référentiel MIME TRACFIN absent)")
		}
	}
	if n := len(sar.SelectedAttachments(in.Pivot)); n > 15 {
		req("pivot/attachments", "REQ_ATTACHMENT_LIMIT", fmt.Sprintf("15 pièces jointes au plus (%d sélectionnées)", n))
	}
	return out
}

type ids struct {
	person map[string]int
	ref    map[string]int // instrument id -> local reference id
	piece  map[string]int
}

func pm(path string, t *models.SarText) *format.Meta { return sar.PivotMeta(path, t) }

func extMeta(in sar.Input, k string) *format.Meta { return sar.ExtMeta(Key, k, in.Ext[k]) }

func settings(in sar.Input, n *format.Node, name, key string) {
	n.Leaf(name, in.Setting(key), &format.Meta{Edit: "settings/" + Key + "/" + key, Provenance: &models.SarProvenance{Kind: models.SarProvSettings}})
}

func (a *Adapter) Project(in sar.Input) ([]*format.Node, []models.SarIssue) {
	var issues []models.SarIssue
	p := in.Pivot
	ix := sar.NewIndex(p)
	id := ids{person: map[string]int{}, ref: map[string]int{}, piece: map[string]int{}}
	for i, x := range p.Parties {
		id.person[x.ID] = i + 1
	}
	for i, x := range sar.SelectedAttachments(p) {
		id.piece[x.ID] = i + 1
	}
	ds := format.NewNode("ds")
	org := ds.Add("organisme")
	settings(in, org, "profession", "organisme_profession")
	settings(in, org, "libelle", "organisme_libelle")
	settings(in, org, "numIdPro", "organisme_num_id_pro")
	ad := org.Add("adresse")
	settings(in, ad, "nomVoie", "organisme_voie")
	settings(in, ad, "codePostal", "organisme_code_postal")
	settings(in, ad, "ville", "organisme_ville")
	ad.Leaf("pays", firstNonEmpty(in.Setting("organisme_pays"), "FR"), sar.DerivedMeta("organisme établi en France", ""))
	settings(in, ad, "tel", "organisme_tel")
	dec := ds.Add("declarant")
	co := dec.Add("coordonnees")
	settings(in, co, "nom", "declarant_nom")
	settings(in, co, "prenom", "declarant_prenom")
	settings(in, co, "tel", "declarant_tel")
	settings(in, co, "courriel", "declarant_courriel")
	if in.Setting("declarant_fonction") != "" {
		settings(in, dec, "fonction", "declarant_fonction")
	}
	ctx := ds.Add("context")
	ns := ctx.Add("natureSoupcon")
	for i, v := range in.Values("cadre_legal") {
		m := extMeta(in, "cadre_legal")
		m.Edit += "/" + strconv.Itoa(i)
		ns.Leaf("cadreLegal", v, m)
	}
	for _, v := range in.Values("nature_soupcon_penale") {
		ns.Leaf("natureSoupconPenale", v, extMeta(in, "nature_soupcon_penale"))
	}
	ns.Leaf("autreNatureSoupconPenale", in.Value("autre_nature_soupcon_penale"), extMeta(in, "autre_nature_soupcon_penale"))
	for _, v := range in.Values("nature_fraude_fiscale") {
		ns.Leaf("natureFraudeFiscale", v, extMeta(in, "nature_fraude_fiscale"))
	}
	ns.Leaf("autreNatureFraudeFiscale", in.Value("autre_nature_fraude_fiscale"), extMeta(in, "autre_nature_fraude_fiscale"))
	gen := ctx.Add("general")
	gen.Leaf("complement", in.Value("complement"), extMeta(in, "complement"))
	gen.Leaf("refInterne", in.Value("ref_interne"), extMeta(in, "ref_interne"))
	precision := in.Value("precision_signalement")
	gen.Leaf("estPrecisionSignalement", boolStr(precision != ""), sar.DerivedMeta("vrai si une précision de signalement est saisie", "extension/"+Key+"/precision_signalement"))
	gen.Leaf("precisionSignalement", precision, extMeta(in, "precision_signalement"))
	gen.Leaf("estRequisitionJudiciaire", boolStr(p.Suspicion.JudicialOrigin.Val() == "true"), &format.Meta{Edit: "pivot/suspicion/judicial_origin", Provenance: p.Suspicion.JudicialOrigin.Prov(), Derived: "faux par défaut"})
	// The information section is mandatory (1..1); its fields default to false.
	inf := ctx.Add("information")
	inf.Leaf("gestionFortuneBanquePrivee", firstNonEmpty(in.Value("gestion_fortune"), "false"), extMeta(in, "gestion_fortune"))
	inf.Leaf("correspondanceBancaire", firstNonEmpty(in.Value("correspondance_bancaire"), "false"), extMeta(in, "correspondance_bancaire"))
	ds.Leaf("analyse", p.Suspicion.Analysis.Val(), pm("pivot/suspicion/analysis", p.Suspicion.Analysis))
	// Persons and their financial references.
	nextRef := 1
	holdersOf := map[string][]*models.SarLink{}
	for i := range p.Links {
		l := &p.Links[i]
		if l.InstrumentID.Val() != "" {
			holdersOf[l.PartyID.Val()] = append(holdersOf[l.PartyID.Val()], l)
		}
	}
	// Counterparty instruments without an explicit link belong to the counterparty.
	for _, op := range sar.SelectedOperations(p) {
		for _, pair := range [][2]string{{op.PartyID.Val(), op.InstrumentID.Val()}, {op.CounterpartyID.Val(), op.CounterpartyInstrument.Val()}} {
			if pair[0] == "" || pair[1] == "" {
				continue
			}
			found := false
			for _, l := range holdersOf[pair[0]] {
				found = found || l.InstrumentID.Val() == pair[1]
			}
			if !found {
				holdersOf[pair[0]] = append(holdersOf[pair[0]], &models.SarLink{ID: "implicit-" + pair[1], Kind: &models.SarText{V: "holder"}, PartyID: &models.SarText{V: pair[0]}, InstrumentID: &models.SarText{V: pair[1]}})
			}
		}
	}
	if len(p.Parties) > 0 {
		pers := ds.Add("personnes")
		for i := range p.Parties {
			x := &p.Parties[i]
			var n *format.Node
			if x.Kind.Val() == "entity" {
				n = pers.Add("pm")
			} else {
				n = pers.Add("pp")
			}
			a.person(in, ix, n, x, id, holdersOf[x.ID], &nextRef)
		}
	}
	a.operations(in, ix, ds, id, &issues)
	if atts := sar.SelectedAttachments(p); len(atts) > 0 {
		pj := ds.Add("pieceJointe")
		for _, at := range atts {
			pre := "pivot/attachments/" + at.ID
			pc := pj.Add("piece")
			pc.Attr("id", strconv.Itoa(id.piece[at.ID]), sar.DerivedMeta("identifiant local de la pièce", pre))
			pc.Leaf("nom", truncate(at.Name.Val(), 120), pm(pre+"/name", at.Name))
			pc.Leaf("type", in.Value(sar.ExtKey("document_type", at.ID)), extMeta(in, sar.ExtKey("document_type", at.ID)))
			date := at.Date.Val()
			if date == "" {
				date = in.Now.Format("2006-01-02")
			}
			pc.Leaf("date", sar.FrenchDate(date), &format.Meta{Edit: pre + "/date", Provenance: at.Date.Prov(), Derived: "date JJ/MM/AAAA"})
			pc.Leaf("libelle", truncate(firstNonEmpty(at.Description.Val(), at.Name.Val()), 30), sar.DerivedMeta("libellé court (30 caractères) tiré de la description", pre+"/description"))
			pc.Leaf("type_mime", at.Mime.Val(), pm(pre+"/mime", at.Mime))
			content := ""
			if f, ok := in.Files[at.ID]; ok {
				content = base64.StdEncoding.EncodeToString(f.Data)
			}
			c := &format.Node{Name: "contenu", Text: &content, Meta: sar.DerivedMeta("contenu du fichier en base64, ajouté à l'export", pre)}
			pc.Children = append(pc.Children, c)
		}
	}
	return []*format.Node{ds}, issues
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

var digits = regexp.MustCompile(`^\d{1,6}$`)
var nirRe = regexp.MustCompile(`^([0-3])\d( ?)\d{2}( ?)\d{3}( ?)\d{3}( ?)\d{3}$`)

func implication(x *models.SarParty) string {
	switch {
	case sar.HasRole(x, "suspect"):
		return "Personne soupçonnée / Personne impliquée"
	case sar.HasRole(x, "victim"):
		return "Victime présumée"
	}
	return "Je ne sais pas"
}

var relations = map[string]string{"business_relationship": "Relation d'affaire", "occasional_customer": "Client occasionnel", "prospect": "Tentative d'entrée en relation ou prospect", "none": "Aucune"}
var genders = map[string]string{"male": "SEX1", "female": "SEX2", "unknown": "SEX3", "other": "SEX3"}
var yesNo = map[string]string{"yes": "Oui", "no": "Non", "unknown": "Je ne sais pas"}
var legalForms = map[string]string{"company": "FJ17", "partnership": "FJ6", "sole_trader": "FJ1", "foundation": "FJ21", "association": "FJ19", "trust": "FJ23", "public_body": "FJ23", "other": "FJ24"}
var links = map[string]string{"holder": "Titulaire", "co_holder": "Co-titulaire", "signatory": "Mandataire"}

func address(n *format.Node, pre string, ad models.SarAddress) {
	ap := pre + "/addresses/" + ad.ID
	street := ad.Street.Val()
	if digits.MatchString(ad.Number.Val()) {
		n.Leaf("numero", ad.Number.Val(), pm(ap+"/number", ad.Number))
	} else if ad.Number.Val() != "" {
		street = ad.Number.Val() + " " + street
	}
	n.Leaf("nomVoie", street, pm(ap+"/street", ad.Street))
	n.Leaf("codePostal", ad.PostalCode.Val(), pm(ap+"/postal_code", ad.PostalCode))
	n.Leaf("ville", ad.City.Val(), pm(ap+"/city", ad.City))
	n.Leaf("pays", ad.Country.Val(), pm(ap+"/country", ad.Country))
}

func (a *Adapter) person(in sar.Input, ix *sar.Index, n *format.Node, x *models.SarParty, id ids, holds []*models.SarLink, nextRef *int) {
	pre := "pivot/parties/" + x.ID
	n.Leaf("id", strconv.Itoa(id.person[x.ID]), sar.DerivedMeta("identifiant local de la personne", pre))
	n.Leaf("implication", implication(x), sar.DerivedMeta("déduit des rôles neutres", pre+"/roles"))
	n.Leaf("natureRelation", relations[x.Relationship.Val()], pm(pre+"/relationship", x.Relationship))
	if x.Kind.Val() == "entity" {
		ac := n.Add("activiteCoordonnee")
		typ := map[string]string{"siren": "SIREN", "rcs": "SIREN", "lei": "RN5", "uid": "RN6", "other": "RN7", "unknown": "Je ne sais pas"}[x.RegistrationType.Val()]
		if typ == "" && x.RegistrationNumber.Val() != "" {
			typ = "RN6"
			if x.RegistrationCountry.Val() == "FR" {
				typ = "RN7"
			}
		}
		if typ == "" {
			typ = "Je ne sais pas"
		}
		ni := &format.Node{Name: "numeroIdentification"}
		ni.SetText(x.RegistrationNumber.Val(), pm(pre+"/registration_number", x.RegistrationNumber))
		ni.Attr("type", typ, sar.DerivedMeta("type de registre", pre+"/registration_type"))
		if x.RegistrationNumber.Val() != "" || typ != "" {
			ac.Children = append(ac.Children, ni)
		}
		ac.Leaf("denominationSociale", x.Name.Val(), pm(pre+"/name", x.Name))
		if typ == "SIREN" {
			ac.Leaf("secteurActivite", truncate(x.BusinessActivity.Val(), 50), pm(pre+"/business_activity", x.BusinessActivity))
		} else {
			ac.Leaf("enseigneSigle", x.TradeName.Val(), pm(pre+"/trade_name", x.TradeName))
			ac.Leaf("formeJuridique", legalForms[x.LegalForm.Val()], sar.DerivedMeta("forme juridique "+x.LegalForm.Val(), pre+"/legal_form"))
			ac.Leaf("activitePrincipale", truncate(x.BusinessActivity.Val(), 50), pm(pre+"/business_activity", x.BusinessActivity))
		}
		if typ == "RN6" || typ == "RN7" {
			ac.Leaf("nomAutreTypeRegistre", firstNonEmpty(x.RegistrationType.Val(), "registre"), sar.DerivedMeta("nom du registre", pre+"/registration_type"))
			ac.Leaf("paysAutreTypeRegistre", x.RegistrationCountry.Val(), pm(pre+"/registration_country", x.RegistrationCountry))
		}
		if len(x.Addresses) > 0 && (x.RegistrationNumber.Val() != "" && typ != "Non") {
			address(ac.Add("adresse"), pre, x.Addresses[0])
		}
		for i := 0; i < len(x.Phones) && i < 3; i++ {
			ac.Leaf("telephoneAvecIndicatif", x.Phones[i].V, pm(fmt.Sprintf("%s/phones/%d", pre, i), &x.Phones[i]))
		}
		if len(x.Emails) > 0 {
			ac.Leaf("courriel", x.Emails[0].V, pm(pre+"/emails/0", &x.Emails[0]))
		}
		ac.Leaf("urlSiteWeb", x.Website.Val(), pm(pre+"/website", x.Website))
		count := 0
		for i := range in.Pivot.Links {
			l := &in.Pivot.Links[i]
			if l.ToPartyID.Val() != x.ID || l.Kind.Val() != "director" || count >= 3 {
				continue
			}
			if _, ok := id.person[l.PartyID.Val()]; !ok {
				continue
			}
			count++
			d := n.Add("dirigeant")
			d.Leaf("fonctionDirigeant", "MF14", sar.DerivedMeta("fonction non précisée dans le pivot", "pivot/links/"+l.ID+"/details"))
			d.Leaf("autreFonctionDirigeant", truncate(firstNonEmpty(l.Details.Val(), "Dirigeant"), 200), pm("pivot/links/"+l.ID+"/details", l.Details))
			d.Leaf("idPersonne", strconv.Itoa(id.person[l.PartyID.Val()]), sar.DerivedMeta("identifiant local du dirigeant déclaré", "pivot/links/"+l.ID+"/party_id"))
			bo := "Non"
			for _, l2 := range in.Pivot.Links {
				if l2.Kind.Val() == "beneficial_owner" && l2.PartyID.Val() == l.PartyID.Val() && l2.ToPartyID.Val() == x.ID {
					bo = "Oui"
				}
			}
			d.Leaf("beneficiaireEffectif", bo, sar.DerivedMeta("lien bénéficiaire effectif dans le pivot", "pivot/links"))
			if bo == "Non" {
				d.Leaf("idBeneficiaireEffectif", strconv.Itoa(id.person[l.PartyID.Val()]), sar.DerivedMeta("à défaut, le dirigeant déclaré", "pivot/links"))
			}
		}
	} else {
		ec := n.Add("etatCivil")
		if x.BirthName.Val() != "" {
			ec.Leaf("nomNaissance", x.BirthName.Val(), pm(pre+"/birth_name", x.BirthName))
			if x.LastName.Val() != x.BirthName.Val() {
				ec.Leaf("nomDusage", x.LastName.Val(), pm(pre+"/last_name", x.LastName))
			}
		} else {
			ec.Leaf("nomDusage", x.LastName.Val(), pm(pre+"/last_name", x.LastName))
		}
		ec.Leaf("prenom", x.FirstName.Val(), pm(pre+"/first_name", x.FirstName))
		for i := 0; i < len(x.Aliases) && i < 10; i++ {
			ec.Leaf("alias", x.Aliases[i].V, pm(fmt.Sprintf("%s/aliases/%d", pre, i), &x.Aliases[i]))
		}
		ec.Leaf("sexe", genders[x.Gender.Val()], sar.DerivedMeta("sexe "+x.Gender.Val(), pre+"/gender"))
		ec.Leaf("dateNaissance", sar.FrenchDate(x.BirthDate.Val()), &format.Meta{Edit: pre + "/birth_date", Provenance: x.BirthDate.Prov(), Derived: "date JJ/MM/AAAA"})
		ec.Leaf("lieuNaissance", x.BirthPlace.Val(), pm(pre+"/birth_place", x.BirthPlace))
		ec.Leaf("paysNaissance", x.BirthCountry.Val(), pm(pre+"/birth_country", x.BirthCountry))
		for i := 0; i < len(x.Nationalities) && i < 5; i++ {
			ec.Leaf("nationalite", x.Nationalities[i].V, pm(fmt.Sprintf("%s/nationalities/%d", pre, i), &x.Nationalities[i]))
		}
		for _, d := range x.Identifications {
			if pid, ok := id.piece[d.AttachmentID.Val()]; ok {
				ec.Leaf("idPieceJointe", strconv.Itoa(pid), pm(pre+"/identifications/"+d.ID+"/attachment_id", d.AttachmentID))
			}
		}
		if len(x.Addresses) > 0 || len(x.Phones) > 0 || len(x.Emails) > 0 {
			co := n.Add("coordonnees")
			if len(x.Addresses) > 0 {
				address(co.Add("adresse"), pre, x.Addresses[0])
			}
			for i := 0; i < len(x.Phones) && i < 3; i++ {
				co.Leaf("tel", x.Phones[i].V, pm(fmt.Sprintf("%s/phones/%d", pre, i), &x.Phones[i]))
			}
			if len(x.Emails) > 0 {
				co.Leaf("courriel", x.Emails[0].V, pm(pre+"/emails/0", &x.Emails[0]))
			}
		}
		if x.PEP.Val() != "" {
			pp := n.Add("ppe")
			pp.Leaf("politiquementExpose", yesNo[x.PEP.Val()], pm(pre+"/pep", x.PEP))
			for _, f := range in.Values(sar.ExtKey("ppe_function", x.ID)) {
				pp.Leaf("function", f, extMeta(in, sar.ExtKey("ppe_function", x.ID)))
			}
		}
		if x.Occupation.Val() != "" {
			n.Add("activite").Leaf("profession", x.Occupation.Val(), pm(pre+"/occupation", x.Occupation))
		}
	}
	if len(holds) > 0 {
		rf := n.Add("referencesFinancieres")
		for _, l := range holds {
			x := ix.Instrument(l.InstrumentID.Val())
			if x == nil {
				continue
			}
			ref, ok := id.ref[x.ID]
			if !ok {
				ref = *nextRef
				*nextRef++
				id.ref[x.ID] = ref
			}
			ipre := "pivot/instruments/" + x.ID
			var e *format.Node
			switch {
			case x.IBAN.Val() != "":
				e = rf.Add("referenceFinanciereIban")
				e.Attr("iban", x.IBAN.Val(), pm(ipre+"/iban", x.IBAN))
			case x.Kind.Val() == "bank_account" && x.BIC.Val() != "":
				e = rf.Add("referenceFinanciereIdentifiantCompte")
				e.Attr("bic", x.BIC.Val(), pm(ipre+"/bic", x.BIC))
				e.Attr("numeroCompte", x.AccountNumber.Val(), pm(ipre+"/account_number", x.AccountNumber))
			case x.Kind.Val() == "card":
				e = rf.Add("referenceFinanciereNumeroCarteBancaire")
				e.Attr("numeroCarteBancaire", x.CardNumber.Val(), pm(ipre+"/card_number", x.CardNumber))
			case x.Kind.Val() == "crypto_wallet":
				e = rf.Add("referenceFinanciereCryptoActif")
				e.Attr("CryptoActif", alnum(x.WalletAddress.Val()), pm(ipre+"/wallet_address", x.WalletAddress))
			case x.Kind.Val() == "e_wallet":
				e = rf.Add("referenceFinancierePortefeuilleElectronique")
				e.Attr("PortefeuilleElectronique", alnum(firstNonEmpty(x.WalletAddress.Val(), x.Reference.Val())), pm(ipre+"/wallet_address", x.WalletAddress))
			case x.Kind.Val() == "cash":
				e = rf.Add("referenceFinanciereEspeces")
			default:
				e = rf.Add("referenceFinanciereAutre")
				e.Attr("AutreType", sar.Vocabularies["instrument_kind"].Label(x.Kind.Val()), sar.DerivedMeta("type de support", ipre+"/kind"))
				e.Attr("NumeroReference", firstNonEmpty(x.AccountNumber.Val(), x.Reference.Val(), x.CardNumber.Val(), x.ID), pm(ipre+"/reference", x.Reference))
			}
			e.Attr("id", strconv.Itoa(ref), sar.DerivedMeta("identifiant local de la référence financière", ipre))
			e.Attr("lien", links[l.Kind.Val()], sar.DerivedMeta("nature du lien "+l.Kind.Val(), "pivot/links/"+l.ID+"/kind"))
			if x.Balance.Val() != "" && x.Currency.Val() == "EUR" {
				e.Attr("soldeRestant", x.Balance.Val(), pm(ipre+"/balance", x.Balance))
			}
		}
	}
	if x.Kind.Val() != "entity" && nirRe.MatchString(x.TaxNumber.Val()) {
		n.Leaf("numeroIdentFiscal", x.TaxNumber.Val(), pm(pre+"/tax_number", x.TaxNumber))
	}
	if x.Sanctioned.Val() != "" {
		nd := n.Add("notorieteDefavorable")
		nd.Leaf("presenceSanction", yesNo[x.Sanctioned.Val()], pm(pre+"/sanctioned", x.Sanctioned))
		if x.Sanctioned.Val() == "yes" {
			nd.Leaf("listesSanctions", x.SanctionLists.Val(), pm(pre+"/sanction_lists", x.SanctionLists))
		}
	}
	n.Leaf("description", truncate(x.Description.Val(), 700), pm(pre+"/description", x.Description))
}

var alnumRe = regexp.MustCompile(`[^a-zA-Z0-9 ]`)

func alnum(s string) string { return alnumRe.ReplaceAllString(s, "") }

type line struct {
	key        string
	typ        string
	ops        []*models.SarOperation
	amounts    []string
	emit, recv string
	from, to   string // countries
	refE, refR string
}

// operations aggregates unit operations into at most 30 lines in EUR.
func (a *Adapter) operations(in sar.Input, ix *sar.Index, ds *format.Node, id ids, issues *[]models.SarIssue) {
	ops := sar.SelectedOperations(in.Pivot)
	if len(ops) == 0 {
		return
	}
	partyText := func(pid string) string {
		if n, ok := id.person[pid]; ok {
			return strconv.Itoa(n)
		}
		return ""
	}
	refText := func(iid string) string {
		if n, ok := id.ref[iid]; ok {
			return strconv.Itoa(n)
		}
		if x := ix.Instrument(iid); x != nil {
			return firstNonEmpty(x.IBAN.Val(), x.AccountNumber.Val(), x.Reference.Val())
		}
		return ""
	}
	build := func(coarse bool) []*line {
		byKey := map[string]*line{}
		var out []*line
		for _, op := range ops {
			typ := in.Value(sar.ExtKey("operation_type", op.ID))
			client, cpty := op.PartyID.Val(), op.CounterpartyID.Val()
			ci, ki := op.InstrumentID.Val(), op.CounterpartyInstrument.Val()
			clientCountry := ""
			if x := ix.Instrument(ci); x != nil {
				clientCountry = x.Country.Val()
			}
			cptyCountry := op.CounterpartyCountry.Val()
			if x := ix.Instrument(ki); x != nil && cptyCountry == "" {
				cptyCountry = x.Country.Val()
			}
			l := line{typ: typ, emit: partyText(client), recv: partyText(cpty), from: clientCountry, to: cptyCountry, refE: refText(ci), refR: refText(ki)}
			if op.Direction.Val() == "incoming" {
				l.emit, l.recv, l.from, l.to, l.refE, l.refR = l.recv, l.emit, l.to, l.from, l.refR, l.refE
			}
			l.key = strings.Join([]string{typ, l.emit, l.recv, l.from, l.to, l.refE, l.refR}, "|")
			if coarse {
				l.key = typ + "|" + op.Direction.Val()
			}
			cur := byKey[l.key]
			if cur == nil {
				cur = &l
				byKey[l.key] = cur
				out = append(out, cur)
			} else if coarse {
				if cur.emit != l.emit {
					cur.emit = ""
				}
				if cur.recv != l.recv {
					cur.recv = ""
				}
			}
			cur.ops = append(cur.ops, op)
			if v, _, ok := sar.Convert(sar.Abs(op.Amount.Val()), op.Currency.Val(), "EUR", in.Rates); ok {
				cur.amounts = append(cur.amounts, v)
			}
		}
		return out
	}
	lines := build(false)
	coarse := false
	if len(lines) > 30 {
		lines, coarse = build(true), true
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].ops[0].Timestamp.Val() < lines[j].ops[0].Timestamp.Val() })
	os := ds.Add("operations")
	first, last, ok := sar.Period(ops)
	if ok {
		os.Leaf("dateDebut", first.In(sar.Paris).Format("02/01/2006"), sar.DerivedMeta("première opération retenue (heure de Paris)", "pivot/operations"))
		os.Leaf("dateFin", last.In(sar.Paris).Format("02/01/2006"), sar.DerivedMeta("dernière opération retenue (heure de Paris)", "pivot/operations"))
	}
	var all []string
	for _, l := range lines {
		all = append(all, l.amounts...)
	}
	if total, ok := sar.Sum(all); ok && len(all) == len(ops) {
		os.Leaf("montantTotal", total, sar.DerivedMeta(fmt.Sprintf("somme en EUR des %d opérations retenues", len(ops)), "pivot/operations"))
	}
	for _, l := range lines {
		on := os.Add("operation")
		ref := "pivot/operations/" + l.ops[0].ID
		on.Leaf("typeOperation", l.typ, extMeta(in, sar.ExtKey("operation_type", l.ops[0].ID)))
		if l.typ == "OT11" {
			on.Leaf("autretypeOperation", sar.Vocabularies["operation_type"].Label(l.ops[0].Type.Val()), sar.DerivedMeta("type neutre de l'opération", ref+"/type"))
		}
		if l.typ != "OT07" {
			on.Leaf("emetteurTypeEmetteur", firstNonEmpty(l.emit, "Inconnu"), sar.DerivedMeta("identifiant local de l'émetteur ou texte libre", ref+"/party_id"))
			on.Leaf("paysOrigineFonds", firstNonEmpty(l.from, "FR"), sar.DerivedMeta("pays du support émetteur", ref+"/instrument_id"))
			on.Leaf("refFinanciereEmission", firstNonEmpty(l.refE, "Inconnue"), sar.DerivedMeta("référence financière émettrice", ref+"/instrument_id"))
		}
		if l.typ != "OT06" {
			on.Leaf("beneficiaireTypeBeneficiaire", firstNonEmpty(l.recv, "Inconnu"), sar.DerivedMeta("identifiant local du bénéficiaire ou texte libre", ref+"/counterparty_id"))
			on.Leaf("paysArriveeFonds", firstNonEmpty(l.to, "FR"), sar.DerivedMeta("pays du support bénéficiaire", ref+"/counterparty_country"))
			on.Leaf("refFinanciereReception", firstNonEmpty(l.refR, "Inconnue"), sar.DerivedMeta("référence financière réceptrice", ref+"/counterparty_instrument_id"))
		}
		if total, ok := sar.Sum(l.amounts); ok && len(l.amounts) == len(l.ops) {
			on.Leaf("montantTotal", total, sar.DerivedMeta(fmt.Sprintf("somme en EUR de %d opération(s)", len(l.ops)), ref+"/amount"))
		}
		on.Leaf("nombreOperations", strconv.Itoa(len(l.ops)), sar.DerivedMeta("nombre d'opérations unitaires agrégées", "pivot/operations"))
		statuses := map[string]int{}
		for _, op := range l.ops {
			statuses[sar.Vocabularies["operation_status"].Label(op.Status.Val())]++
		}
		var parts []string
		for k, v := range statuses {
			parts = append(parts, fmt.Sprintf("%d %s", v, strings.ToLower(k)))
		}
		sort.Strings(parts)
		comment := fmt.Sprintf("%d opération(s) du %s au %s : %s", len(l.ops), sar.FrenchDate(l.ops[0].Timestamp.Val()), sar.FrenchDate(l.ops[len(l.ops)-1].Timestamp.Val()), strings.Join(parts, ", "))
		if coarse {
			comment += " (regroupement par type, plus de 30 lignes détaillées)"
		}
		on.Leaf("commentaires", truncate(comment, 700), sar.DerivedMeta("synthèse calculée", "pivot/operations"))
	}
}

func (a *Adapter) Package(docs [][]byte, _ []sar.AttachmentContent, in sar.Input) (sar.Artifact, error) {
	return sar.Artifact{Name: "tracfin-" + in.Value("ref_interne") + ".xml", Mime: "application/xml", Data: docs[0]}, nil
}
