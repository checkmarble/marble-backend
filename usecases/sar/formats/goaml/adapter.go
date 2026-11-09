// Package goaml is the single goAML 5.0.2 adapter. Countries are profiles:
// a generated schema (structure, enumerations, labels), code mappings from
// the neutral pivot, report type choice, extension fields and Go asserts.
package goaml

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math/big"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

type Profile struct {
	Desc         sar.Descriptor
	Schema       *format.Schema
	TZ           *time.Location
	MaxTx        int    // transactions per report, 0 = unlimited
	DateElement  string // submission_date or report_date
	Ext          []sar.ExtField
	Instructions string

	Funds         func(op *models.SarOperation) string
	Transmode     func(op *models.SarOperation) string
	TxType        func(op *models.SarOperation) string // nil when transaction_type_code is not accepted
	AccountType   map[string]string                    // instrument kind -> code
	AccountStatus map[string]string
	Gender        map[string]string
	IDType        map[string]string
	LegalForm     map[string]string
	AddressType   map[string]string
	PhoneType     [2]string                       // contact type, communication type
	EntityRole    map[string]string               // link kind -> entity_person_role_type
	AccountRole   map[string]string               // link kind -> account_person_role_type
	PartyRole     func(p *models.SarParty) string // report_party role (CH), nil when unsupported
	Suspected     bool                            // *_is_suspected / is_suspected accepted
	ForeignAlways bool                            // my_client side always carries foreign_currency (CH)
	ReportType    func(in sar.Input, ops []*models.SarOperation) (string, string)
	// Hooks for country specific nodes.
	PrepareHook func(in sar.Input, out models.SarExtension)
	ReportHook  func(in sar.Input, ctx *Ctx, report *format.Node)
	AccountHook func(in sar.Input, ctx *Ctx, x *models.SarInstrument, acc *format.Node, myClient bool)
	EntityHook  func(in sar.Input, p *models.SarParty, n *format.Node, myClient bool)
	Require     func(in sar.Input) []models.SarIssue
	Rules       []format.Rule
}

type Adapter struct{ p *Profile }

func New(p *Profile) *Adapter {
	p.Schema.Rules = p.Rules
	return &Adapter{p: p}
}

func (a *Adapter) Descriptor() sar.Descriptor      { return a.p.Desc }
func (a *Adapter) Schema() *format.Schema          { return a.p.Schema }
func (a *Adapter) Extension() []sar.ExtField       { return a.p.Ext }
func (a *Adapter) Instructions() string            { return a.p.Instructions }
func (a *Adapter) Hint(norm, parent string) string { return parent }

// Ctx carries the projection state of one document.
type Ctx struct {
	in   sar.Input
	ix   *sar.Index
	key  string
	p    *Profile
	Ops  []*models.SarOperation
	Errs []models.SarIssue
}

func pm(path string, t *models.SarText) *format.Meta { return sar.PivotMeta(path, t) }

func (a *Adapter) extMeta(in sar.Input, field string) *format.Meta {
	return sar.ExtMeta(a.p.Desc.Key, field, in.Ext[field])
}

// Prepare computes backend values and defaults derived from the pivot.
func (a *Adapter) Prepare(in sar.Input) models.SarExtension {
	out := models.SarExtension{}
	now := in.Now
	ops := a.operations(in)
	rt, why := a.p.ReportType(in, ops)
	if o := in.Value("report_type_override"); o != "" {
		rt, why = o, "type choisi par l'analyste"
	}
	out["report_type"] = models.SarExtValue{Values: []string{rt}, P: sar.Calc(why)}
	out["submission_date"] = models.SarExtValue{Values: []string{now.In(a.p.TZ).Format("2006-01-02T15:04:05")}, P: sar.Calc("date de génération")}
	ref := in.Setting("entity_reference_prefix") + strings.ToUpper(strings.ReplaceAll(in.Pivot.CaseID, "-", ""))[:min(12, len(strings.ReplaceAll(in.Pivot.CaseID, "-", "")))]
	out["entity_reference"] = models.SarExtValue{Values: []string{ref}, P: sar.Calc("préfixe configuré + identifiant du case")}
	for _, op := range ops {
		def := func(field, value, why string) {
			k := sar.ExtKey(field, op.ID)
			if value == "" {
				return
			}
			if cur, ok := in.Ext[k]; ok && cur.P != nil && cur.P.Kind != models.SarProvCalculated {
				return
			}
			out[k] = models.SarExtValue{Values: []string{value}, P: sar.Calc(why)}
		}
		def("funds_code", a.p.Funds(op), "déduit du type d'opération "+op.Type.Val())
		def("transmode_code", a.p.Transmode(op), "déduit du statut/canal de l'opération")
		if a.p.TxType != nil {
			def("transaction_type_code", a.p.TxType(op), "déduit du type d'opération "+op.Type.Val())
		}
	}
	if a.p.PrepareHook != nil {
		a.p.PrepareHook(in, out)
	}
	return out
}

func (a *Adapter) operations(in sar.Input) []*models.SarOperation {
	return sar.SelectedOperations(in.Pivot)
}

// Requirements lists pivot facts this format needs.
func (a *Adapter) Requirements(in sar.Input) []models.SarIssue {
	var out []models.SarIssue
	if in.Pivot.Suspicion.Analysis.Val() == "" {
		out = append(out, models.SarIssue{Path: "pivot/suspicion/analysis", Code: "REQ_ANALYSIS", Message: "L'analyse de l'analyste alimente reason et est obligatoire", Severity: "error", Layer: "pivot", Edit: "pivot/suspicion/analysis"})
	}
	ix := sar.NewIndex(in.Pivot)
	for _, op := range a.operations(in) {
		pp := "pivot/operations/" + op.ID
		for _, f := range [][2]string{{"timestamp", op.Timestamp.Val()}, {"amount", op.Amount.Val()}, {"currency", op.Currency.Val()}, {"direction", op.Direction.Val()}, {"party_id", op.PartyID.Val()}} {
			if f[1] == "" {
				out = append(out, models.SarIssue{Path: pp + "/" + f[0], Code: "REQ_OPERATION_FIELD", Message: "goAML exige " + f[0] + " pour chaque transaction", Severity: "error", Layer: "pivot", Edit: pp + "/" + f[0]})
			}
		}
		if op.Currency.Val() != "" && op.Currency.Val() != a.p.Desc.Currency {
			if _, _, ok := sar.Convert("1", op.Currency.Val(), a.p.Desc.Currency, in.Rates); !ok {
				out = append(out, models.SarIssue{Path: pp + "/currency", Code: "REQ_RATE", Message: fmt.Sprintf("Taux %s/%s non configuré : conversion impossible", op.Currency.Val(), a.p.Desc.Currency), Severity: "error", Layer: "pivot", Edit: "configuration/rates/" + op.Currency.Val() + "/" + a.p.Desc.Currency})
			}
		}
		if op.CounterpartyID.Val() == "" && op.CounterpartyInstrument.Val() == "" {
			out = append(out, models.SarIssue{Path: pp + "/counterparty_id", Code: "REQ_COUNTERPARTY", Message: "Transaction bi-partie : contrepartie ou son support requis", Severity: "error", Layer: "pivot", Edit: pp + "/counterparty_id"})
		}
		if c := ix.Party(op.PartyID.Val()); c != nil && c.Kind.Val() == "person" && c.BirthDate.Val() == "" && isClient(c) {
			path := "pivot/parties/" + c.ID + "/birth_date"
			out = append(out, models.SarIssue{Path: path, Code: "REQ_CLIENT_BIRTHDATE", Message: "La date de naissance d'un client (my_client) est obligatoire", Severity: "error", Layer: "pivot", Edit: path})
		}
	}
	out = append(out, a.reportedFacts(in, ix)...)
	if a.p.Require != nil {
		out = append(out, a.p.Require(in)...)
	}
	return out
}

// reportedFacts checks the facts goAML needs on the parties and accounts that
// the projection reports, localized on the pivot so the analyst can fix them.
func (a *Adapter) reportedFacts(in sar.Input, ix *sar.Index) []models.SarIssue {
	var out []models.SarIssue
	seen := map[string]bool{}
	req := func(path, code, msg string) {
		if !seen[path+code] {
			seen[path+code] = true
			out = append(out, models.SarIssue{Path: path, Code: code, Message: msg, Severity: "error", Layer: "pivot", Edit: path})
		}
	}
	party := func(p *models.SarParty) {
		if p == nil {
			return
		}
		pre := "pivot/parties/" + p.ID
		for _, ad := range p.Addresses {
			ap := pre + "/addresses/" + ad.ID
			if ad.City.Val() == "" {
				req(ap+"/city", "REQ_ADDRESS_CITY", "goAML exige la ville de chaque adresse déclarée (t_address/city)")
			}
			if ad.Country.Val() == "" {
				req(ap+"/country", "REQ_ADDRESS_COUNTRY", "goAML exige le pays de chaque adresse déclarée (t_address/country_code)")
			}
		}
	}
	instrument := func(x *models.SarInstrument) {
		if x == nil || !accountLike(x) {
			return
		}
		pre := "pivot/instruments/" + x.ID
		if x.BIC.Val() == "" && (x.IBAN.Val() != "" || x.InstitutionCode.Val() == "") {
			req(pre+"/bic", "REQ_ACCOUNT_BIC", "BIC/SWIFT requis : goAML exige swift ou institution_code, et un IBAN exige le SWIFT (REJ-ACC02)")
		}
		for _, h := range holders(in, x) {
			party(h.Party)
		}
	}
	for _, op := range a.operations(in) {
		party(ix.Party(op.PartyID.Val()))
		party(ix.Party(op.CounterpartyID.Val()))
		instrument(ix.Instrument(op.InstrumentID.Val()))
		instrument(ix.Instrument(op.CounterpartyInstrument.Val()))
	}
	return out
}

func isClient(p *models.SarParty) bool {
	return p != nil && (p.IsClient.Val() == "true" || sar.HasRole(p, "client"))
}

func heldHere(x *models.SarInstrument) bool { return x != nil && x.HeldAtReporter.Val() == "true" }

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Project builds one report per chunk of MaxTx transactions.
func (a *Adapter) Project(in sar.Input) ([]*format.Node, []models.SarIssue) {
	ops := a.operations(in)
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].Timestamp.Val() < ops[j].Timestamp.Val() })
	rt := in.Value("report_type")
	withTx := rt == "STR" || rt == "TFTR" || rt == "STRe" || rt == "AIFT" || rt == "RIRT"
	var chunks [][]*models.SarOperation
	if withTx && len(ops) > 0 {
		size := a.p.MaxTx
		if size <= 0 {
			size = len(ops)
		}
		for i := 0; i < len(ops); i += size {
			chunks = append(chunks, ops[i:min(i+size, len(ops))])
		}
	} else {
		chunks = [][]*models.SarOperation{nil}
	}
	var docs []*format.Node
	var issues []models.SarIssue
	for i, chunk := range chunks {
		ctx := &Ctx{in: in, ix: sar.NewIndex(in.Pivot), key: a.p.Desc.Key, p: a.p, Ops: chunk}
		suffix := ""
		if len(chunks) > 1 {
			suffix = "-" + strconv.Itoa(i+1)
		}
		doc := a.report(ctx, rt, suffix, withTx)
		conform(a.p.Schema, doc, a.p.Schema.RootType)
		docs = append(docs, doc)
		for _, e := range ctx.Errs {
			if len(chunks) > 1 {
				e.Document = i + 1
			}
			issues = append(issues, e)
		}
	}
	return docs, issues
}

func (a *Adapter) report(ctx *Ctx, rt, suffix string, withTx bool) *format.Node {
	in := ctx.in
	key := a.p.Desc.Key
	r := format.NewNode("report")
	settings := func(field string) *format.Meta {
		return &format.Meta{Edit: "settings/" + key + "/" + field, Provenance: &models.SarProvenance{Kind: models.SarProvSettings}}
	}
	r.Leaf("rentity_id", in.Setting("rentity_id"), settings("rentity_id"))
	if b := in.Setting("rentity_branch"); b != "" {
		r.Leaf("rentity_branch", b, settings("rentity_branch"))
	}
	r.Leaf("submission_code", "E", sar.DerivedMeta("dépôt électronique (XML)", ""))
	rtMeta := a.extMeta(in, "report_type")
	rtMeta.Edit = "extension/" + key + "/report_type_override"
	rtMeta.Derived = "type choisi par le backend selon les faits ; modifiable par report_type_override"
	r.Leaf("report_code", rt, rtMeta)
	r.Leaf("entity_reference", in.Value("entity_reference")+suffix, a.extMeta(in, "entity_reference"))
	if v := in.Value("fiu_ref_number"); v != "" {
		r.Leaf("fiu_ref_number", v, a.extMeta(in, "fiu_ref_number"))
	}
	r.Leaf(a.p.DateElement, in.Value("submission_date"), a.extMeta(in, "submission_date"))
	r.Leaf("currency_code_local", a.p.Desc.Currency, sar.DerivedMeta("devise locale fixée par la CRF", ""))
	if code := in.Setting("reporting_user_code"); code != "" {
		r.Leaf("reporting_user_code", code, settings("reporting_user_code"))
	} else if fn := in.Setting("reporting_person_first_name"); fn != "" {
		rp := r.Add("reporting_person")
		rp.Leaf("first_name", fn, settings("reporting_person_first_name"))
		rp.Leaf("last_name", in.Setting("reporting_person_last_name"), settings("reporting_person_last_name"))
		if ph := in.Setting("reporting_person_phone"); ph != "" {
			phones := rp.Add("phones").Add("phone")
			phones.Leaf("tph_contact_type", a.p.PhoneType[0], nil)
			phones.Leaf("tph_communication_type", a.p.PhoneType[1], nil)
			phones.Leaf("tph_number", ph, settings("reporting_person_phone"))
		}
	}
	s := in.Pivot.Suspicion
	r.Leaf("reason", s.Analysis.Val(), pm("pivot/suspicion/analysis", s.Analysis))
	if s.ActionsTaken.Val() != "" {
		r.Leaf("action", s.ActionsTaken.Val(), pm("pivot/suspicion/actions_taken", s.ActionsTaken))
	}
	if withTx {
		for _, op := range ctx.Ops {
			a.transaction(ctx, r, op)
		}
	} else {
		a.activity(ctx, r)
	}
	inds := in.Values("indicators")
	if band := in.Value("amount_band"); band != "" {
		inds = append(append([]string{}, inds...), band)
	}
	if len(inds) > 0 {
		ri := r.Add("report_indicators")
		for i, code := range inds {
			m := a.extMeta(in, "indicators")
			m.Edit += "/" + strconv.Itoa(i)
			if i >= len(in.Values("indicators")) {
				m = a.extMeta(in, "amount_band")
				m.Derived = "tranche de montant calculée"
			}
			ri.Leaf("indicator", code, m)
		}
	}
	if a.p.ReportHook != nil {
		a.p.ReportHook(in, ctx, r)
	}
	return r
}

func (a *Adapter) transaction(ctx *Ctx, r *format.Node, op *models.SarOperation) {
	in := ctx.in
	pp := "pivot/operations/" + op.ID
	t := r.Add("transaction")
	number := op.Reference.Val()
	if number == "" {
		number = op.ID
	}
	t.Leaf("transactionnumber", number, pm(pp+"/reference", op.Reference))
	t.Leaf("internal_ref_number", op.ID, sar.DerivedMeta("identifiant Marble de l'opération", pp))
	if op.Location.Val() != "" {
		t.Leaf("transaction_location", op.Location.Val(), pm(pp+"/location", op.Location))
	}
	desc := op.Description.Val()
	if desc == "" && a.p.Desc.Country == "CH" {
		desc = sar.Vocabularies["operation_type"].Label(op.Type.Val())
	}
	t.Leaf("transaction_description", desc, pm(pp+"/description", op.Description))
	t.Leaf("date_transaction", sar.GoAMLDateTime(op.Timestamp.Val(), a.p.TZ), &format.Meta{Edit: pp + "/timestamp", Provenance: op.Timestamp.Prov(), Derived: "horodatage converti en heure locale " + a.p.TZ.String()})
	if op.ValueDate.Val() != "" {
		t.Leaf("value_date", sar.GoAMLDateTime(op.ValueDate.Val(), a.p.TZ), pm(pp+"/value_date", op.ValueDate))
	}
	if a.p.TxType != nil {
		t.Leaf("transaction_type_code", in.Value(sar.ExtKey("transaction_type_code", op.ID)), a.extMeta(in, sar.ExtKey("transaction_type_code", op.ID)))
	}
	t.Leaf("transmode_code", in.Value(sar.ExtKey("transmode_code", op.ID)), a.extMeta(in, sar.ExtKey("transmode_code", op.ID)))
	amount, why, ok := sar.Convert(sar.Abs(op.Amount.Val()), op.Currency.Val(), a.p.Desc.Currency, in.Rates)
	if ok {
		if why == "" {
			why = "montant en devise locale"
		}
		t.Leaf("amount_local", amount, &format.Meta{Edit: pp + "/amount", Provenance: sar.Calc(why), Derived: why})
	}
	client := ctx.ix.Party(op.PartyID.Val())
	clientInstr := ctx.ix.Instrument(op.InstrumentID.Val())
	cpty := ctx.ix.Party(op.CounterpartyID.Val())
	cptyInstr := ctx.ix.Instrument(op.CounterpartyInstrument.Val())
	type side struct {
		party *models.SarParty
		instr *models.SarInstrument
		my    bool
	}
	clientSide := side{client, clientInstr, isClient(client) && (clientInstr == nil || heldHere(clientInstr))}
	otherSide := side{cpty, cptyInstr, isClient(cpty) && heldHere(cptyInstr)}
	from, to := clientSide, otherSide
	if op.Direction.Val() == "incoming" {
		from, to = otherSide, clientSide
	}
	funds := in.Value(sar.ExtKey("funds_code", op.ID))
	fundsMeta := a.extMeta(in, sar.ExtKey("funds_code", op.ID))
	build := func(prefix string, sd side) {
		name := "t_" + prefix
		if sd.my {
			name += "_my_client"
		}
		n := t.Add(name)
		n.Leaf(prefix+"_funds_code", funds, fundsMeta)
		if op.Currency.Val() != a.p.Desc.Currency || (a.p.ForeignAlways && sd.my) {
			fc := n.Add(prefix + "_foreign_currency")
			fc.Leaf("foreign_currency_code", op.Currency.Val(), pm(pp+"/currency", op.Currency))
			fc.Leaf("foreign_amount", sar.Abs(op.Amount.Val()), pm(pp+"/amount", op.Amount))
			if rate, ok := rateFor(op.Currency.Val(), a.p.Desc.Currency, in.Rates); ok {
				fc.Leaf("foreign_exchange_rate", rate, sar.DerivedMeta("taux configuré", "configuration/rates"))
			}
		}
		if a.p.Suspected && sd.party != nil && sar.HasRole(sd.party, "suspect") {
			n.Leaf(prefix+"_is_suspected", "true", pm("pivot/parties/"+sd.party.ID+"/roles", nil))
		}
		switch {
		case sd.instr != nil && accountLike(sd.instr):
			a.account(ctx, n.Add(prefix+"_account"), sd.instr, sd.my)
		case sd.party != nil && sd.party.Kind.Val() == "entity":
			a.entity(ctx, n.Add(prefix+"_entity"), sd.party, sd.my)
		case sd.party != nil:
			a.person(ctx, n.Add(prefix+"_person"), sd.party, sd.my)
		}
		country := ""
		var meta *format.Meta
		switch {
		case sd.instr != nil && sd.instr.Country.Val() != "":
			country, meta = sd.instr.Country.Val(), pm("pivot/instruments/"+sd.instr.ID+"/country", sd.instr.Country)
		case sd.party == cpty && op.CounterpartyCountry.Val() != "":
			country, meta = op.CounterpartyCountry.Val(), pm(pp+"/counterparty_country", op.CounterpartyCountry)
		case sd.party != nil:
			country, meta = partyCountry(sd.party)
		}
		n.Leaf(prefix+"_country", country, meta)
	}
	build("from", from)
	build("to", to)
}

func rateFor(from, to string, rates map[string]string) (string, bool) {
	if r, ok := rates[from+"/"+to]; ok {
		return r, true
	}
	if r, ok := rates[to+"/"+from]; ok {
		x, ok2 := new(big.Rat).SetString(r)
		if ok2 && x.Sign() > 0 {
			// Inverse rate with 10 decimals: the XSD decimal has no fraction limit.
			return sar.Round(new(big.Rat).Inv(x), 10), true
		}
	}
	return "", false
}

func accountLike(x *models.SarInstrument) bool {
	switch x.Kind.Val() {
	case "cash", "cheque":
		return false
	}
	return x.IBAN.Val() != "" || x.AccountNumber.Val() != "" || x.CardNumber.Val() != "" || x.WalletAddress.Val() != "" || x.Reference.Val() != ""
}

func partyCountry(p *models.SarParty) (string, *format.Meta) {
	pre := "pivot/parties/" + p.ID
	for _, ad := range p.Addresses {
		if ad.Country.Val() != "" {
			return ad.Country.Val(), pm(pre+"/addresses/"+ad.ID+"/country", ad.Country)
		}
	}
	if p.Residence.Val() != "" {
		return p.Residence.Val(), pm(pre+"/residence", p.Residence)
	}
	if p.RegistrationCountry.Val() != "" {
		return p.RegistrationCountry.Val(), pm(pre+"/registration_country", p.RegistrationCountry)
	}
	if len(p.Nationalities) > 0 {
		return p.Nationalities[0].V, pm(pre+"/nationalities/0", &p.Nationalities[0])
	}
	return "", nil
}

func displayName(p *models.SarParty) string {
	if p == nil {
		return ""
	}
	if p.Kind.Val() == "entity" {
		return p.Name.Val()
	}
	return strings.TrimSpace(p.FirstName.Val() + " " + p.LastName.Val())
}

// holders returns parties linked to an instrument with their link kind.
func holders(in sar.Input, x *models.SarInstrument) []struct {
	Party *models.SarParty
	Link  *models.SarLink
} {
	ix := sar.NewIndex(in.Pivot)
	var out []struct {
		Party *models.SarParty
		Link  *models.SarLink
	}
	for i := range in.Pivot.Links {
		l := &in.Pivot.Links[i]
		if l.InstrumentID.Val() == x.ID {
			if p := ix.Party(l.PartyID.Val()); p != nil {
				out = append(out, struct {
					Party *models.SarParty
					Link  *models.SarLink
				}{p, l})
			}
		}
	}
	return out
}

func (a *Adapter) account(ctx *Ctx, n *format.Node, x *models.SarInstrument, my bool) {
	pre := "pivot/instruments/" + x.ID
	n.Meta = &format.Meta{Edit: pre}
	if x.Institution.Val() != "" && !(a.p.Desc.Country == "LU" && x.InstitutionCode.Val() != "" && x.BIC.Val() == "") {
		n.Leaf("institution_name", x.Institution.Val(), pm(pre+"/institution", x.Institution))
	}
	if x.BIC.Val() != "" {
		n.Leaf("swift", x.BIC.Val(), pm(pre+"/bic", x.BIC))
		if len(x.BIC.Val()) == 11 && a.p.Desc.Country == "LU" {
			n.Leaf("branch", x.BIC.Val(), sar.DerivedMeta("REJ-ACC03 : un SWIFT de 11 caractères est repris en branch", pre+"/bic"))
		}
	} else if x.InstitutionCode.Val() != "" {
		n.Leaf("institution_code", x.InstitutionCode.Val(), pm(pre+"/institution_code", x.InstitutionCode))
	}
	number, numberMeta := x.AccountNumber.Val(), pm(pre+"/account_number", x.AccountNumber)
	if x.IBAN.Val() != "" {
		number, numberMeta = x.IBAN.Val(), sar.DerivedMeta("REJ-ACC01 : le numéro de compte reprend l'IBAN", pre+"/iban")
	}
	for _, alt := range []*models.SarText{x.CardNumber, x.WalletAddress, x.Reference} {
		if number == "" && alt.Val() != "" {
			number = alt.Val()
		}
	}
	n.Leaf("account", number, numberMeta)
	if my && x.Currency.Val() != "" {
		n.Leaf("currency_code", x.Currency.Val(), pm(pre+"/currency", x.Currency))
	}
	hs := holders(ctx.in, x)
	name := ""
	for _, h := range hs {
		if h.Link.Kind.Val() == "holder" || h.Link.Kind.Val() == "co_holder" {
			if name != "" {
				name += " / "
			}
			name += displayName(h.Party)
		}
	}
	if name != "" {
		n.Leaf("account_name", name, sar.DerivedMeta("nom des titulaires liés au support", pre))
	}
	if x.IBAN.Val() != "" {
		n.Leaf("iban", x.IBAN.Val(), pm(pre+"/iban", x.IBAN))
	}
	if my {
		if code := a.p.AccountType[x.Kind.Val()]; code != "" {
			n.Leaf("account_type", code, sar.DerivedMeta("type de support "+x.Kind.Val(), pre+"/kind"))
		}
		var entityHolder *models.SarParty
		for _, h := range hs {
			if h.Party.Kind.Val() == "entity" && (h.Link.Kind.Val() == "holder" || h.Link.Kind.Val() == "co_holder") && entityHolder == nil {
				entityHolder = h.Party
				a.entity(ctx, n.Add("t_entity"), h.Party, false)
			}
		}
		for _, h := range hs {
			if h.Party.Kind.Val() != "person" {
				continue
			}
			role := a.p.AccountRole[h.Link.Kind.Val()]
			if role == "" {
				continue
			}
			sg := n.Add("signatory")
			a.person(ctx, sg.Add("t_person"), h.Party, false)
			sg.Leaf("role", role, sar.DerivedMeta("nature du lien "+h.Link.Kind.Val(), "pivot/links/"+h.Link.ID+"/kind"))
		}
		if x.Opened.Val() != "" {
			n.Leaf("opened", sar.GoAMLDateTime(x.Opened.Val(), a.p.TZ), pm(pre+"/opened", x.Opened))
		}
		if x.Closed.Val() != "" {
			n.Leaf("closed", sar.GoAMLDateTime(x.Closed.Val(), a.p.TZ), pm(pre+"/closed", x.Closed))
		}
		if x.Balance.Val() != "" {
			n.Leaf("balance", x.Balance.Val(), pm(pre+"/balance", x.Balance))
			n.Leaf("date_balance", sar.GoAMLDateTime(x.BalanceDate.Val(), a.p.TZ), pm(pre+"/balance_date", x.BalanceDate))
		}
		if code := a.p.AccountStatus[x.Status.Val()]; code != "" {
			n.Leaf("status_code", code, sar.DerivedMeta("statut "+x.Status.Val(), pre+"/status"))
		}
	}
	if a.p.AccountHook != nil {
		a.p.AccountHook(ctx.in, ctx, x, n, my)
	}
}

func (a *Adapter) phones(n *format.Node, p *models.SarParty) {
	if len(p.Phones) == 0 {
		return
	}
	ph := n.Add("phones")
	for i, t := range p.Phones {
		x := ph.Add("phone")
		x.Leaf("tph_contact_type", a.p.PhoneType[0], nil)
		x.Leaf("tph_communication_type", a.p.PhoneType[1], nil)
		x.Leaf("tph_number", t.V, pm(fmt.Sprintf("pivot/parties/%s/phones/%d", p.ID, i), &p.Phones[i]))
	}
}

func (a *Adapter) addresses(n *format.Node, p *models.SarParty) {
	if len(p.Addresses) == 0 {
		return
	}
	as := n.Add("addresses")
	for _, ad := range p.Addresses {
		pre := "pivot/parties/" + p.ID + "/addresses/" + ad.ID
		x := as.Add("address")
		typ := a.p.AddressType[ad.Type.Val()]
		if typ == "" {
			typ = a.p.AddressType["other"]
		}
		x.Leaf("address_type", typ, sar.DerivedMeta("type d'adresse "+ad.Type.Val(), pre+"/type"))
		street := strings.TrimSpace(ad.Number.Val() + " " + ad.Street.Val())
		x.Leaf("address", street, &format.Meta{Edit: pre + "/street", Provenance: ad.Street.Prov(), Derived: "numéro et voie"})
		x.Leaf("city", ad.City.Val(), pm(pre+"/city", ad.City))
		x.Leaf("zip", ad.PostalCode.Val(), pm(pre+"/postal_code", ad.PostalCode))
		x.Leaf("country_code", ad.Country.Val(), pm(pre+"/country", ad.Country))
	}
}

func (a *Adapter) person(ctx *Ctx, n *format.Node, p *models.SarParty, my bool) {
	pre := "pivot/parties/" + p.ID
	n.Meta = &format.Meta{Edit: pre}
	if g := a.p.Gender[p.Gender.Val()]; g != "" {
		n.Leaf("gender", g, sar.DerivedMeta("sexe "+p.Gender.Val(), pre+"/gender"))
	}
	n.Leaf("first_name", p.FirstName.Val(), pm(pre+"/first_name", p.FirstName))
	n.Leaf("last_name", p.LastName.Val(), pm(pre+"/last_name", p.LastName))
	if p.BirthDate.Val() != "" {
		n.Leaf("birthdate", sar.GoAMLDateTime(p.BirthDate.Val(), a.p.TZ), pm(pre+"/birth_date", p.BirthDate))
	}
	n.Leaf("birth_place", p.BirthPlace.Val(), pm(pre+"/birth_place", p.BirthPlace))
	n.Leaf("country_of_birth", p.BirthCountry.Val(), pm(pre+"/birth_country", p.BirthCountry))
	if len(p.Aliases) > 0 {
		n.Leaf("alias", p.Aliases[0].V, pm(pre+"/aliases/0", &p.Aliases[0]))
	}
	for i := 0; i < len(p.Nationalities) && i < 3; i++ {
		n.Leaf("nationality"+strconv.Itoa(i+1), p.Nationalities[i].V, pm(fmt.Sprintf("%s/nationalities/%d", pre, i), &p.Nationalities[i]))
	}
	if e, ok := ctx.p.Schema.Lookup("t_person", "residence"); ok && e.Max != 0 {
		n.Leaf("residence", p.Residence.Val(), pm(pre+"/residence", p.Residence))
	}
	a.phones(n, p)
	a.addresses(n, p)
	if len(p.Emails) > 0 {
		if e, ok := ctx.p.Schema.Lookup("t_person", "email"); ok && e.Max != 0 {
			n.Leaf("email", p.Emails[0].V, pm(pre+"/emails/0", &p.Emails[0]))
		} else {
			n.Add("emails").Leaf("email", p.Emails[0].V, pm(pre+"/emails/0", &p.Emails[0]))
		}
	}
	n.Leaf("occupation", p.Occupation.Val(), pm(pre+"/occupation", p.Occupation))
	n.Leaf("employer_name", p.Employer.Val(), pm(pre+"/employer", p.Employer))
	for _, id := range p.Identifications {
		ip := pre + "/identifications/" + id.ID
		code := a.p.IDType[id.Type.Val()]
		if code == "" || id.Number.Val() == "" {
			continue
		}
		x := n.Add("identification")
		x.Leaf("type", code, sar.DerivedMeta("type de pièce "+id.Type.Val(), ip+"/type"))
		x.Leaf("number", id.Number.Val(), pm(ip+"/number", id.Number))
		if id.IssueDate.Val() != "" {
			x.Leaf("issue_date", sar.GoAMLDateTime(id.IssueDate.Val(), a.p.TZ), pm(ip+"/issue_date", id.IssueDate))
		}
		if id.ExpiryDate.Val() != "" {
			x.Leaf("expiry_date", sar.GoAMLDateTime(id.ExpiryDate.Val(), a.p.TZ), pm(ip+"/expiry_date", id.ExpiryDate))
		}
		x.Leaf("issued_by", id.IssuedBy.Val(), pm(ip+"/issued_by", id.IssuedBy))
		x.Leaf("issue_country", id.IssueCountry.Val(), pm(ip+"/issue_country", id.IssueCountry))
	}
	if e, ok := ctx.p.Schema.Lookup("t_person", "tax_number"); ok && e.Max != 0 {
		n.Leaf("tax_number", p.TaxNumber.Val(), pm(pre+"/tax_number", p.TaxNumber))
	}
	if p.Description.Val() != "" {
		n.Leaf("comments", p.Description.Val(), pm(pre+"/description", p.Description))
	}
}

func (a *Adapter) entity(ctx *Ctx, n *format.Node, p *models.SarParty, my bool) {
	pre := "pivot/parties/" + p.ID
	n.Meta = &format.Meta{Edit: pre}
	n.Leaf("name", p.Name.Val(), pm(pre+"/name", p.Name))
	n.Leaf("commercial_name", p.TradeName.Val(), pm(pre+"/trade_name", p.TradeName))
	if lf := a.p.LegalForm[p.LegalForm.Val()]; lf != "" {
		n.Leaf("incorporation_legal_form", lf, sar.DerivedMeta("forme juridique "+p.LegalForm.Val(), pre+"/legal_form"))
	}
	n.Leaf("incorporation_number", p.RegistrationNumber.Val(), pm(pre+"/registration_number", p.RegistrationNumber))
	n.Leaf("business", p.BusinessActivity.Val(), pm(pre+"/business_activity", p.BusinessActivity))
	if e, ok := ctx.p.Schema.Lookup("t_entity", "phones"); ok && e.Max != 0 {
		a.phones(n, p)
	}
	a.addresses(n, p)
	n.Leaf("incorporation_country_code", p.RegistrationCountry.Val(), pm(pre+"/registration_country", p.RegistrationCountry))
	if a.p.EntityHook != nil {
		a.p.EntityHook(ctx.in, p, n, my)
	}
	if e, ok := ctx.p.Schema.Lookup("t_entity", "director_id"); ok && e.Max != 0 {
		for i := range ctx.in.Pivot.Links {
			l := &ctx.in.Pivot.Links[i]
			if l.ToPartyID.Val() != p.ID || (l.Kind.Val() != "director" && l.Kind.Val() != "beneficial_owner") {
				continue
			}
			d := ctx.ix.Party(l.PartyID.Val())
			if d == nil || d.Kind.Val() != "person" {
				continue
			}
			dn := n.Add("director_id")
			a.person(ctx, dn, d, false)
			if role := a.p.EntityRole[l.Kind.Val()]; role != "" {
				dn.Leaf("role", role, sar.DerivedMeta("nature du lien "+l.Kind.Val(), "pivot/links/"+l.ID+"/kind"))
			}
		}
	}
	if p.IncorporationDate.Val() != "" {
		n.Leaf("incorporation_date", sar.GoAMLDateTime(p.IncorporationDate.Val(), a.p.TZ), pm(pre+"/incorporation_date", p.IncorporationDate))
	}
}

// activity lists the subjects of a report without transactions.
func (a *Adapter) activity(ctx *Ctx, r *format.Node) {
	act := r.Add("activity")
	rps := act.Add("report_parties")
	for i := range ctx.in.Pivot.Parties {
		p := &ctx.in.Pivot.Parties[i]
		if !sar.HasRole(p, "suspect") && !isClient(p) {
			continue
		}
		rp := rps.Add("report_party")
		if a.p.PartyRole != nil {
			rp.Leaf("role", a.p.PartyRole(p), sar.DerivedMeta("rôle déduit des rôles neutres", "pivot/parties/"+p.ID+"/roles"))
		}
		my := isClient(p)
		var accountOf *models.SarInstrument
		if my && a.p.Desc.Country == "CH" {
			for _, h := range ctx.in.Pivot.Instruments {
				h := h
				for _, l := range ctx.in.Pivot.Links {
					if l.PartyID.Val() == p.ID && l.InstrumentID.Val() == h.ID && heldHere(&h) && accountOf == nil {
						accountOf = &h
					}
				}
			}
		}
		switch {
		case accountOf != nil:
			a.account(ctx, rp.Add("account_my_client"), accountOf, true)
		case p.Kind.Val() == "entity" && my:
			a.entity(ctx, rp.Add("entity_my_client"), p, true)
		case p.Kind.Val() == "entity":
			a.entity(ctx, rp.Add("entity"), p, false)
		case my:
			a.person(ctx, rp.Add("person_my_client"), p, true)
		default:
			a.person(ctx, rp.Add("person"), p, false)
		}
		if a.p.Suspected {
			rp.Leaf("is_suspected", boolStr(sar.HasRole(p, "suspect")), sar.DerivedMeta("rôle suspect", "pivot/parties/"+p.ID+"/roles"))
		}
	}
}

// Package returns the XML alone or a ZIP with the XML files and attachments
// outside the XML, one folder per report.
func (a *Adapter) Package(docs [][]byte, attachments []sar.AttachmentContent, in sar.Input) (sar.Artifact, error) {
	base := strings.ToLower(strings.ReplaceAll(a.p.Desc.Key, ":", "-")) + "-" + in.Value("entity_reference")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, d := range docs {
		folder := base
		if len(docs) > 1 {
			folder += "-" + strconv.Itoa(i+1)
		}
		w, err := zw.Create(path.Join(folder, folder+".xml"))
		if err != nil {
			return sar.Artifact{}, err
		}
		if _, err = w.Write(d); err != nil {
			return sar.Artifact{}, err
		}
		if i == 0 {
			for _, at := range attachments {
				w, err := zw.Create(path.Join(folder, sanitize(at.Name)))
				if err != nil {
					return sar.Artifact{}, err
				}
				if _, err = w.Write(at.Data); err != nil {
					return sar.Artifact{}, err
				}
			}
		}
	}
	if err := zw.Close(); err != nil {
		return sar.Artifact{}, err
	}
	return sar.Artifact{Name: base + ".zip", Mime: "application/zip", Data: buf.Bytes()}, nil
}

func sanitize(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return "attachment"
	}
	return name
}

// conform makes a projected document follow the country profile: elements
// the profile forbids (maxOccurs=0) are dropped, and single elements are
// wrapped in their list container when only the container is accepted
// (email -> emails/email, identification -> identifications/identification).
func conform(s *format.Schema, n *format.Node, typeName string) {
	wrap := map[string]string{"email": "emails", "identification": "identifications"}
	var kept []*format.Node
	containers := map[string]*format.Node{}
	for _, c := range n.Children {
		e, ok := s.Lookup(typeName, c.Name)
		if ok && e.Max != 0 {
			if _, complex := s.Complex[e.Type]; complex && !c.Attribute {
				conform(s, c, e.Type)
			}
			kept = append(kept, c)
			continue
		}
		if container, ok := wrap[c.Name]; ok {
			if ce, ok := s.Lookup(typeName, container); ok && ce.Max != 0 {
				box := containers[container]
				if box == nil {
					box = &format.Node{Name: container}
					containers[container] = box
					kept = append(kept, box)
				}
				box.Children = append(box.Children, c)
				conform(s, box, ce.Type)
			}
		}
	}
	n.Children = kept
}
