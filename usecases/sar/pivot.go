// Package sar builds suspicious activity reports from a format-neutral pivot.
// It holds the pivot vocabulary and validation, stable-path patches, backend
// derivations, the adapter contract and the agent loop. It has no database
// dependency so that adapters and the loop are testable with a simulated model.
package sar

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// Vocab is a neutral code list of the pivot, with French labels.
type Vocab struct {
	Name  string      `json:"name"`
	Codes [][2]string `json:"codes"`
}

func v(name string, codes ...string) Vocab {
	out := Vocab{Name: name}
	for i := 0; i+1 < len(codes); i += 2 {
		out.Codes = append(out.Codes, [2]string{codes[i], codes[i+1]})
	}
	return out
}

var Vocabularies = map[string]Vocab{
	"party_kind":          v("party_kind", "person", "Personne physique", "entity", "Personne morale", "unknown", "Partie non identifiée"),
	"role":                v("role", "suspect", "Suspect / personne impliquée", "victim", "Victime présumée", "client", "Client", "beneficial_owner", "Bénéficiaire effectif", "director", "Dirigeant", "signatory", "Signataire / mandataire", "counterparty", "Contrepartie", "conductor", "Donneur d'ordre / exécutant", "other", "Autre"),
	"relationship":        v("relationship", "business_relationship", "Relation d'affaires", "occasional_customer", "Client occasionnel", "prospect", "Prospect / tentative d'entrée en relation", "none", "Aucune"),
	"gender":              v("gender", "male", "Homme", "female", "Femme", "other", "Autre", "unknown", "Inconnu"),
	"yes_no":              v("yes_no", "yes", "Oui", "no", "Non", "unknown", "Je ne sais pas"),
	"boolean":             v("boolean", "true", "Oui", "false", "Non"),
	"legal_form":          v("legal_form", "company", "Société de capitaux", "partnership", "Société de personnes", "sole_trader", "Entrepreneur individuel", "foundation", "Fondation", "association", "Association", "trust", "Trust / fiducie", "public_body", "Organisme public", "other", "Autre"),
	"registration_type":   v("registration_type", "siren", "SIREN (France)", "rcs", "Registre du commerce (RCS)", "uid", "Numéro IDE/UID (Suisse)", "lei", "LEI", "other", "Autre registre", "unknown", "Inconnu"),
	"address_type":        v("address_type", "home", "Domicile", "business", "Professionnelle", "other", "Autre"),
	"id_type":             v("id_type", "passport", "Passeport", "id_card", "Carte d'identité", "driving_licence", "Permis de conduire", "residence_permit", "Titre de séjour", "other", "Autre"),
	"instrument_kind":     v("instrument_kind", "bank_account", "Compte bancaire", "card", "Carte bancaire", "prepaid_card", "Carte prépayée", "e_wallet", "Portefeuille électronique", "crypto_wallet", "Portefeuille crypto-actifs", "cheque", "Chèque", "contract", "Contrat", "cash", "Espèces", "online_pot", "Cagnotte en ligne", "other", "Autre"),
	"instrument_category": v("instrument_category", "personal", "Particulier", "business", "Professionnel"),
	"instrument_status":   v("instrument_status", "active", "Actif", "closed", "Clôturé", "blocked", "Bloqué", "dormant", "Inactif"),
	"link_kind":           v("link_kind", "holder", "Titulaire", "co_holder", "Co-titulaire", "signatory", "Mandataire / signataire", "director", "Dirigeant", "beneficial_owner", "Bénéficiaire effectif", "shareholder", "Actionnaire", "family", "Lien familial", "associate", "Associé / relation d'affaires", "employee", "Employé", "other", "Autre"),
	"direction":           v("direction", "outgoing", "Sortante (débit du client)", "incoming", "Entrante (crédit du client)", "internal", "Interne"),
	"operation_type":      v("operation_type", "transfer", "Virement", "cash_deposit", "Versement d'espèces", "cash_withdrawal", "Retrait d'espèces", "card_payment", "Paiement par carte", "cheque", "Chèque", "crypto", "Opération sur crypto-actifs", "direct_debit", "Prélèvement", "money_remittance", "Transmission de fonds", "other", "Autre"),
	"channel":             v("channel", "branch", "Agence / guichet", "online", "Banque en ligne", "atm", "Distributeur", "mobile", "Application mobile", "other", "Autre"),
	"operation_status":    v("operation_status", "executed", "Exécutée", "attempted", "Tentée", "refused", "Refusée", "pending", "En cours"),
	"document_type":       v("document_type", "identity", "Pièce d'identité", "statement", "Relevé de compte", "contract", "Contrat", "correspondence", "Correspondance", "invoice", "Facture", "screening", "Résultat de criblage", "other", "Autre"),
	"theme":               v("theme", "money_laundering", "Blanchiment", "terrorism_financing", "Financement du terrorisme", "tax_fraud", "Fraude fiscale", "fraud", "Escroquerie / fraude", "corruption", "Corruption", "drug_trafficking", "Trafic de stupéfiants", "sanctions", "Sanctions / gel des avoirs", "pep", "Personne politiquement exposée", "cash", "Espèces", "organized_crime", "Criminalité organisée", "human_trafficking", "Traite des êtres humains", "cybercrime", "Cybercriminalité", "social_fraud", "Fraude sociale", "breach_of_trust", "Abus de confiance / abus de biens sociaux", "other", "Autre"),
	"prior_relation":      v("prior_relation", "complement", "Complément d'une déclaration", "related", "Déclaration liée"),
}

func (vb Vocab) Has(code string) bool {
	for _, c := range vb.Codes {
		if c[0] == code {
			return true
		}
	}
	return false
}

func (vb Vocab) Label(code string) string {
	for _, c := range vb.Codes {
		if c[0] == code {
			return c[1]
		}
	}
	return code
}

// FieldSpec describes a pivot scalar for validation, prompts and the frontend.
type FieldSpec struct {
	Path  string `json:"path"` // e.g. parties/*/birth_date
	Label string `json:"label"`
	Kind  string `json:"kind"` // text, date, timestamp, decimal, country, currency, code, ref, boolean
	Vocab string `json:"vocab,omitempty"`
	Ref   string `json:"ref,omitempty"` // collection referenced by an ID
	List  bool   `json:"list,omitempty"`
	Max   int    `json:"max_length,omitempty"`
	Help  string `json:"help,omitempty"`
}

var PivotFields = []FieldSpec{
	{Path: "parties/*/kind", Label: "Type de partie", Kind: "code", Vocab: "party_kind"},
	{Path: "parties/*/roles", Label: "Rôles", Kind: "code", Vocab: "role", List: true},
	{Path: "parties/*/is_client", Label: "Client de l'établissement", Kind: "boolean"},
	{Path: "parties/*/relationship", Label: "Nature de la relation", Kind: "code", Vocab: "relationship"},
	{Path: "parties/*/client_since", Label: "Client depuis", Kind: "date"},
	{Path: "parties/*/client_ref", Label: "Référence client", Kind: "text", Max: 100},
	{Path: "parties/*/first_name", Label: "Prénom", Kind: "text", Max: 100},
	{Path: "parties/*/last_name", Label: "Nom", Kind: "text", Max: 100},
	{Path: "parties/*/birth_name", Label: "Nom de naissance", Kind: "text", Max: 100},
	{Path: "parties/*/aliases", Label: "Alias", Kind: "text", List: true, Max: 100},
	{Path: "parties/*/gender", Label: "Sexe", Kind: "code", Vocab: "gender"},
	{Path: "parties/*/birth_date", Label: "Date de naissance", Kind: "date"},
	{Path: "parties/*/birth_place", Label: "Lieu de naissance", Kind: "text", Max: 200},
	{Path: "parties/*/birth_country", Label: "Pays de naissance", Kind: "country"},
	{Path: "parties/*/nationalities", Label: "Nationalités", Kind: "country", List: true},
	{Path: "parties/*/residence", Label: "Pays de résidence", Kind: "country"},
	{Path: "parties/*/occupation", Label: "Profession", Kind: "text", Max: 200},
	{Path: "parties/*/employer", Label: "Employeur", Kind: "text", Max: 200},
	{Path: "parties/*/tax_number", Label: "Numéro fiscal", Kind: "text", Max: 50},
	{Path: "parties/*/pep", Label: "Personne politiquement exposée", Kind: "code", Vocab: "yes_no"},
	{Path: "parties/*/pep_function", Label: "Fonction PPE", Kind: "text", Max: 200},
	{Path: "parties/*/sanctioned", Label: "Présence sur une liste de sanctions", Kind: "code", Vocab: "yes_no"},
	{Path: "parties/*/sanction_lists", Label: "Listes de sanctions", Kind: "text", Max: 700},
	{Path: "parties/*/name", Label: "Dénomination", Kind: "text", Max: 200},
	{Path: "parties/*/trade_name", Label: "Enseigne / sigle", Kind: "text", Max: 200},
	{Path: "parties/*/legal_form", Label: "Forme juridique", Kind: "code", Vocab: "legal_form"},
	{Path: "parties/*/registration_number", Label: "Numéro d'immatriculation", Kind: "text", Max: 50},
	{Path: "parties/*/registration_country", Label: "Pays d'immatriculation", Kind: "country"},
	{Path: "parties/*/registration_type", Label: "Type de registre", Kind: "code", Vocab: "registration_type"},
	{Path: "parties/*/incorporation_date", Label: "Date de constitution", Kind: "date"},
	{Path: "parties/*/business_activity", Label: "Activité", Kind: "text", Max: 200},
	{Path: "parties/*/website", Label: "Site web", Kind: "text", Max: 255},
	{Path: "parties/*/phones", Label: "Téléphones", Kind: "text", List: true, Max: 25},
	{Path: "parties/*/emails", Label: "Courriels", Kind: "text", List: true, Max: 200},
	{Path: "parties/*/description", Label: "Description", Kind: "text", Max: 700},
	{Path: "parties/*/addresses/*/type", Label: "Type d'adresse", Kind: "code", Vocab: "address_type"},
	{Path: "parties/*/addresses/*/street", Label: "Voie", Kind: "text", Max: 100},
	{Path: "parties/*/addresses/*/number", Label: "Numéro", Kind: "text", Max: 10},
	{Path: "parties/*/addresses/*/postal_code", Label: "Code postal", Kind: "text", Max: 10},
	{Path: "parties/*/addresses/*/city", Label: "Ville", Kind: "text", Max: 200},
	{Path: "parties/*/addresses/*/region", Label: "Région", Kind: "text", Max: 200},
	{Path: "parties/*/addresses/*/country", Label: "Pays", Kind: "country"},
	{Path: "parties/*/identifications/*/type", Label: "Type de pièce", Kind: "code", Vocab: "id_type"},
	{Path: "parties/*/identifications/*/number", Label: "Numéro", Kind: "text", Max: 100},
	{Path: "parties/*/identifications/*/issue_country", Label: "Pays de délivrance", Kind: "country"},
	{Path: "parties/*/identifications/*/issued_by", Label: "Autorité de délivrance", Kind: "text", Max: 255},
	{Path: "parties/*/identifications/*/issue_date", Label: "Date de délivrance", Kind: "date"},
	{Path: "parties/*/identifications/*/expiry_date", Label: "Date d'expiration", Kind: "date"},
	{Path: "parties/*/identifications/*/attachment_id", Label: "Pièce jointe", Kind: "ref", Ref: "attachments"},
	{Path: "instruments/*/kind", Label: "Type de support", Kind: "code", Vocab: "instrument_kind"},
	{Path: "instruments/*/held_at_reporter", Label: "Tenu par l'établissement", Kind: "boolean"},
	{Path: "instruments/*/iban", Label: "IBAN", Kind: "text", Max: 34},
	{Path: "instruments/*/account_number", Label: "Numéro de compte", Kind: "text", Max: 50},
	{Path: "instruments/*/bic", Label: "BIC", Kind: "text", Max: 11},
	{Path: "instruments/*/institution", Label: "Établissement teneur", Kind: "text", Max: 255},
	{Path: "instruments/*/institution_code", Label: "Code établissement", Kind: "text", Max: 50},
	{Path: "instruments/*/country", Label: "Pays", Kind: "country"},
	{Path: "instruments/*/currency", Label: "Devise", Kind: "currency"},
	{Path: "instruments/*/category", Label: "Catégorie", Kind: "code", Vocab: "instrument_category"},
	{Path: "instruments/*/status", Label: "Statut", Kind: "code", Vocab: "instrument_status"},
	{Path: "instruments/*/opened", Label: "Date d'ouverture", Kind: "date"},
	{Path: "instruments/*/closed", Label: "Date de clôture", Kind: "date"},
	{Path: "instruments/*/balance", Label: "Solde", Kind: "decimal"},
	{Path: "instruments/*/balance_date", Label: "Date du solde", Kind: "date"},
	{Path: "instruments/*/card_number", Label: "Numéro de carte", Kind: "text", Max: 19},
	{Path: "instruments/*/wallet_address", Label: "Adresse du portefeuille", Kind: "text", Max: 200},
	{Path: "instruments/*/reference", Label: "Autre référence", Kind: "text", Max: 200},
	{Path: "instruments/*/description", Label: "Description", Kind: "text", Max: 700},
	{Path: "links/*/kind", Label: "Nature du lien", Kind: "code", Vocab: "link_kind"},
	{Path: "links/*/party_id", Label: "Partie", Kind: "ref", Ref: "parties"},
	{Path: "links/*/to_party_id", Label: "Partie liée", Kind: "ref", Ref: "parties"},
	{Path: "links/*/instrument_id", Label: "Support", Kind: "ref", Ref: "instruments"},
	{Path: "links/*/details", Label: "Précisions", Kind: "text", Max: 200},
	{Path: "links/*/since", Label: "Depuis", Kind: "date"},
	{Path: "operations/*/reference", Label: "Référence", Kind: "text", Max: 100},
	{Path: "operations/*/timestamp", Label: "Date et heure", Kind: "timestamp"},
	{Path: "operations/*/value_date", Label: "Date de valeur", Kind: "date"},
	{Path: "operations/*/amount", Label: "Montant (devise d'origine)", Kind: "decimal"},
	{Path: "operations/*/currency", Label: "Devise", Kind: "currency"},
	{Path: "operations/*/direction", Label: "Sens", Kind: "code", Vocab: "direction"},
	{Path: "operations/*/type", Label: "Type d'opération", Kind: "code", Vocab: "operation_type"},
	{Path: "operations/*/channel", Label: "Canal", Kind: "code", Vocab: "channel"},
	{Path: "operations/*/status", Label: "Statut", Kind: "code", Vocab: "operation_status"},
	{Path: "operations/*/party_id", Label: "Client concerné", Kind: "ref", Ref: "parties"},
	{Path: "operations/*/instrument_id", Label: "Support du client", Kind: "ref", Ref: "instruments"},
	{Path: "operations/*/counterparty_id", Label: "Contrepartie", Kind: "ref", Ref: "parties"},
	{Path: "operations/*/counterparty_instrument_id", Label: "Support de la contrepartie", Kind: "ref", Ref: "instruments"},
	{Path: "operations/*/counterparty_country", Label: "Pays de la contrepartie", Kind: "country"},
	{Path: "operations/*/location", Label: "Lieu", Kind: "text", Max: 255},
	{Path: "operations/*/description", Label: "Libellé", Kind: "text", Max: 700},
	{Path: "operations/*/suspicious", Label: "Opération suspecte", Kind: "boolean"},
	{Path: "operations/*/selected", Label: "Retenue dans la déclaration", Kind: "boolean"},
	{Path: "attachments/*/name", Label: "Nom du fichier", Kind: "text", Max: 120},
	{Path: "attachments/*/mime", Label: "Type MIME", Kind: "text", Max: 100},
	{Path: "attachments/*/sha256", Label: "Empreinte", Kind: "text", Max: 64},
	{Path: "attachments/*/size", Label: "Taille", Kind: "decimal"},
	{Path: "attachments/*/document_type", Label: "Type de document", Kind: "code", Vocab: "document_type"},
	{Path: "attachments/*/date", Label: "Date du document", Kind: "date"},
	{Path: "attachments/*/description", Label: "Description", Kind: "text", Max: 700},
	{Path: "attachments/*/selected", Label: "Jointe à la déclaration", Kind: "boolean"},
	{Path: "attachments/*/party_id", Label: "Partie concernée", Kind: "ref", Ref: "parties"},
	{Path: "suspicion/analysis", Label: "Analyse (rédigée par l'analyste)", Kind: "text", Max: 23000},
	{Path: "suspicion/themes", Label: "Thèmes du soupçon", Kind: "code", Vocab: "theme", List: true},
	{Path: "suspicion/summary", Label: "Résumé factuel", Kind: "text", Max: 4000},
	{Path: "suspicion/actions_taken", Label: "Mesures prises", Kind: "text", Max: 4000},
	{Path: "suspicion/detection_date", Label: "Date de détection", Kind: "date"},
	{Path: "suspicion/urgent", Label: "Urgence (opération en cours)", Kind: "boolean"},
	{Path: "suspicion/judicial_origin", Label: "Réquisition judiciaire", Kind: "boolean"},
	{Path: "prior_reports/*/fiu", Label: "Destinataire", Kind: "text", Max: 50},
	{Path: "prior_reports/*/reference", Label: "Référence", Kind: "text", Max: 50},
	{Path: "prior_reports/*/date", Label: "Date", Kind: "date"},
	{Path: "prior_reports/*/relation", Label: "Lien", Kind: "code", Vocab: "prior_relation"},
}

func FieldByPath(pattern string) (FieldSpec, bool) {
	for _, f := range PivotFields {
		if f.Path == pattern {
			return f, true
		}
	}
	return FieldSpec{}, false
}

// Pattern replaces object IDs of a concrete pivot path by "*".
func Pattern(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "pivot/"), "/")
	for i := range parts {
		if i%2 == 1 && i < len(parts)-1 && (parts[i-1] != "suspicion") {
			parts[i] = "*"
		}
	}
	p := strings.Join(parts, "/")
	// list index suffix
	if i := strings.LastIndexByte(p, '/'); i >= 0 && isIndex(p[i+1:]) {
		p = p[:i]
	}
	return p
}

func isIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
	decimalRe  = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

func checkValue(f FieldSpec, value string) string {
	switch f.Kind {
	case "date":
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "expected an ISO date YYYY-MM-DD"
		}
	case "timestamp":
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return "expected an RFC 3339 timestamp with offset"
		}
	case "decimal":
		if !decimalRe.MatchString(value) {
			return "expected an exact decimal with a dot separator"
		}
	case "country":
		if !countryRe.MatchString(value) {
			return "expected an ISO 3166 alpha-2 country code"
		}
	case "currency":
		if !currencyRe.MatchString(value) {
			return "expected an ISO 4217 currency code"
		}
	case "boolean":
		if value != "true" && value != "false" {
			return "expected true or false"
		}
	case "code":
		if !Vocabularies[f.Vocab].Has(value) {
			return fmt.Sprintf("unknown code %q for %s", value, f.Vocab)
		}
	}
	if f.Max > 0 && len([]rune(value)) > f.Max {
		return fmt.Sprintf("at most %d characters", f.Max)
	}
	return ""
}

func pivotIssue(path, code, msg string) models.SarIssue {
	return models.SarIssue{Path: path, Code: code, Message: msg, Severity: "error", Layer: "pivot", Edit: path}
}

// ValidatePivot checks neutral types, codes, references and duplicates.
func ValidatePivot(p *models.SarPivot) []models.SarIssue {
	var issues []models.SarIssue
	ids := map[string]map[string]bool{"parties": {}, "instruments": {}, "attachments": {}, "operations": {}, "links": {}, "prior_reports": {}}
	collect := func(coll, id string) {
		if id == "" {
			issues = append(issues, pivotIssue("pivot/"+coll, "PIVOT_ID_MISSING", "object without id"))
		} else if ids[coll][id] {
			issues = append(issues, pivotIssue("pivot/"+coll+"/"+id, "PIVOT_ID_DUPLICATE", "duplicate id"))
		}
		ids[coll][id] = true
	}
	for _, x := range p.Parties {
		collect("parties", x.ID)
	}
	for _, x := range p.Instruments {
		collect("instruments", x.ID)
	}
	for _, x := range p.Attachments {
		collect("attachments", x.ID)
	}
	for _, x := range p.Operations {
		collect("operations", x.ID)
	}
	for _, x := range p.Links {
		collect("links", x.ID)
	}
	for _, x := range p.PriorReports {
		collect("prior_reports", x.ID)
	}
	EachValue(p, func(path string, f FieldSpec, t *models.SarText) {
		if t == nil || t.V == "" {
			return
		}
		if msg := checkValue(f, t.V); msg != "" {
			issues = append(issues, pivotIssue(path, "PIVOT_VALUE_INVALID", f.Label+": "+msg))
		}
		if f.Kind == "ref" && !ids[f.Ref][t.V] {
			issues = append(issues, pivotIssue(path, "PIVOT_REFERENCE_UNKNOWN", f.Label+": unknown "+f.Ref+" id "+t.V))
		}
		if t.P == nil {
			issues = append(issues, models.SarIssue{Path: path, Code: "PIVOT_PROVENANCE_MISSING", Message: "value without provenance", Severity: "warning", Layer: "pivot", Edit: path})
		}
	})
	return issues
}

// EachValue visits every pivot scalar with its concrete path and spec.
func EachValue(p *models.SarPivot, fn func(path string, f FieldSpec, t *models.SarText)) {
	visit := func(prefix, pattern string, fields map[string]*models.SarText, lists map[string][]models.SarText) {
		names := make([]string, 0, len(fields))
		for k := range fields {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			f, ok := FieldByPath(pattern + "/" + k)
			if ok {
				fn(prefix+"/"+k, f, fields[k])
			}
		}
		names = names[:0]
		for k := range lists {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			f, ok := FieldByPath(pattern + "/" + k)
			if !ok {
				continue
			}
			for i := range lists[k] {
				fn(fmt.Sprintf("%s/%s/%d", prefix, k, i), f, &lists[k][i])
			}
		}
	}
	for i := range p.Parties {
		x := &p.Parties[i]
		pre := "pivot/parties/" + x.ID
		visit(pre, "parties/*", partyFields(x), map[string][]models.SarText{"roles": x.Roles, "aliases": x.Aliases, "nationalities": x.Nationalities, "phones": x.Phones, "emails": x.Emails})
		for j := range x.Addresses {
			a := &x.Addresses[j]
			visit(pre+"/addresses/"+a.ID, "parties/*/addresses/*", map[string]*models.SarText{"type": a.Type, "street": a.Street, "number": a.Number, "postal_code": a.PostalCode, "city": a.City, "region": a.Region, "country": a.Country}, nil)
		}
		for j := range x.Identifications {
			d := &x.Identifications[j]
			visit(pre+"/identifications/"+d.ID, "parties/*/identifications/*", map[string]*models.SarText{"type": d.Type, "number": d.Number, "issue_country": d.IssueCountry, "issued_by": d.IssuedBy, "issue_date": d.IssueDate, "expiry_date": d.ExpiryDate, "attachment_id": d.AttachmentID}, nil)
		}
	}
	for i := range p.Instruments {
		x := &p.Instruments[i]
		visit("pivot/instruments/"+x.ID, "instruments/*", map[string]*models.SarText{"kind": x.Kind, "held_at_reporter": x.HeldAtReporter, "iban": x.IBAN, "account_number": x.AccountNumber, "bic": x.BIC, "institution": x.Institution, "institution_code": x.InstitutionCode, "country": x.Country, "currency": x.Currency, "category": x.Category, "status": x.Status, "opened": x.Opened, "closed": x.Closed, "balance": x.Balance, "balance_date": x.BalanceDate, "card_number": x.CardNumber, "wallet_address": x.WalletAddress, "reference": x.Reference, "description": x.Description}, nil)
	}
	for i := range p.Links {
		x := &p.Links[i]
		visit("pivot/links/"+x.ID, "links/*", map[string]*models.SarText{"kind": x.Kind, "party_id": x.PartyID, "to_party_id": x.ToPartyID, "instrument_id": x.InstrumentID, "details": x.Details, "since": x.Since}, nil)
	}
	for i := range p.Operations {
		x := &p.Operations[i]
		visit("pivot/operations/"+x.ID, "operations/*", map[string]*models.SarText{"reference": x.Reference, "timestamp": x.Timestamp, "value_date": x.ValueDate, "amount": x.Amount, "currency": x.Currency, "direction": x.Direction, "type": x.Type, "channel": x.Channel, "status": x.Status, "party_id": x.PartyID, "instrument_id": x.InstrumentID, "counterparty_id": x.CounterpartyID, "counterparty_instrument_id": x.CounterpartyInstrument, "counterparty_country": x.CounterpartyCountry, "location": x.Location, "description": x.Description, "suspicious": x.Suspicious, "selected": x.Selected}, nil)
	}
	for i := range p.Attachments {
		x := &p.Attachments[i]
		visit("pivot/attachments/"+x.ID, "attachments/*", map[string]*models.SarText{"name": x.Name, "mime": x.Mime, "sha256": x.SHA256, "size": x.Size, "document_type": x.DocumentType, "date": x.Date, "description": x.Description, "selected": x.Selected, "party_id": x.PartyID}, nil)
	}
	s := &p.Suspicion
	visit("pivot/suspicion", "suspicion", map[string]*models.SarText{"analysis": s.Analysis, "summary": s.Summary, "actions_taken": s.ActionsTaken, "detection_date": s.DetectionDate, "urgent": s.Urgent, "judicial_origin": s.JudicialOrigin}, map[string][]models.SarText{"themes": s.Themes})
	for i := range p.PriorReports {
		x := &p.PriorReports[i]
		visit("pivot/prior_reports/"+x.ID, "prior_reports/*", map[string]*models.SarText{"fiu": x.FIU, "reference": x.Reference, "date": x.Date, "relation": x.Relation}, nil)
	}
}

func partyFields(x *models.SarParty) map[string]*models.SarText {
	return map[string]*models.SarText{"kind": x.Kind, "is_client": x.IsClient, "relationship": x.Relationship, "client_since": x.ClientSince, "client_ref": x.ClientRef, "first_name": x.FirstName, "last_name": x.LastName, "birth_name": x.BirthName, "gender": x.Gender, "birth_date": x.BirthDate, "birth_place": x.BirthPlace, "birth_country": x.BirthCountry, "residence": x.Residence, "occupation": x.Occupation, "employer": x.Employer, "tax_number": x.TaxNumber, "pep": x.PEP, "pep_function": x.PEPFunction, "sanctioned": x.Sanctioned, "sanction_lists": x.SanctionLists, "name": x.Name, "trade_name": x.TradeName, "legal_form": x.LegalForm, "registration_number": x.RegistrationNumber, "registration_country": x.RegistrationCountry, "registration_type": x.RegistrationType, "incorporation_date": x.IncorporationDate, "business_activity": x.BusinessActivity, "website": x.Website, "description": x.Description}
}

// Lookups used by adapters.
func (ix *Index) Party(id string) *models.SarParty           { return ix.parties[id] }
func (ix *Index) Instrument(id string) *models.SarInstrument { return ix.instruments[id] }
func (ix *Index) Attachment(id string) *models.SarAttachment { return ix.attachments[id] }

type Index struct {
	parties     map[string]*models.SarParty
	instruments map[string]*models.SarInstrument
	attachments map[string]*models.SarAttachment
}

func NewIndex(p *models.SarPivot) *Index {
	ix := &Index{parties: map[string]*models.SarParty{}, instruments: map[string]*models.SarInstrument{}, attachments: map[string]*models.SarAttachment{}}
	for i := range p.Parties {
		ix.parties[p.Parties[i].ID] = &p.Parties[i]
	}
	for i := range p.Instruments {
		ix.instruments[p.Instruments[i].ID] = &p.Instruments[i]
	}
	for i := range p.Attachments {
		ix.attachments[p.Attachments[i].ID] = &p.Attachments[i]
	}
	return ix
}

// HasRole reports a neutral role of a party.
func HasRole(x *models.SarParty, role string) bool {
	if x == nil {
		return false
	}
	for _, r := range x.Roles {
		if r.V == role {
			return true
		}
	}
	return false
}

func HasTheme(p *models.SarPivot, theme string) bool {
	for _, t := range p.Suspicion.Themes {
		if t.V == theme {
			return true
		}
	}
	return false
}

// Selected returns operations retained in the reports (selected != false).
func SelectedOperations(p *models.SarPivot) []*models.SarOperation {
	var out []*models.SarOperation
	for i := range p.Operations {
		if p.Operations[i].Selected.Val() != "false" {
			out = append(out, &p.Operations[i])
		}
	}
	return out
}

func SelectedAttachments(p *models.SarPivot) []*models.SarAttachment {
	var out []*models.SarAttachment
	for i := range p.Attachments {
		if p.Attachments[i].Selected.Val() == "true" {
			out = append(out, &p.Attachments[i])
		}
	}
	return out
}

// DecimalCmp compares two exact decimals.
func DecimalCmp(a, b string) int {
	x, ok1 := new(big.Rat).SetString(a)
	y, ok2 := new(big.Rat).SetString(b)
	if !ok1 || !ok2 {
		return 0
	}
	return x.Cmp(y)
}
