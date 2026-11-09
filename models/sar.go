package models

import (
	"encoding/json"
	"time"
)

// Suspicious activity reports (SAR) are built from one format-neutral pivot
// per case. Format adapters (TRACFIN, goAML LU, goAML CH...) add their own
// extension values and project pivot + extension deterministically into XML.

// Provenance kinds of a pivot or extension value.
const (
	SarProvDataModel  = "data_model" // mapped from an ingested object or decision snapshot
	SarProvDocument   = "document"   // read in a case file
	SarProvManual     = "manual"     // typed by the analyst
	SarProvAI         = "ai"         // proposed by the model from cited sources
	SarProvCalculated = "calculated" // derived by the backend
	SarProvSettings   = "settings"   // organization format settings
)

type SarSourceRef struct {
	SourceID string `json:"source_id"`           // ID of a collected context source (file:..., object:..., snapshot:...)
	Locator  string `json:"locator,omitempty"`   // JSON pointer, page, line or passage
	Table    string `json:"table,omitempty"`     // data model table
	ObjectID string `json:"object_id,omitempty"` // data model object ID
	Field    string `json:"field,omitempty"`     // data model field
}

type SarProvenance struct {
	Kind        string         `json:"kind"`
	Sources     []SarSourceRef `json:"sources,omitempty"`
	Explanation string         `json:"explanation,omitempty"`
	MappingID   string         `json:"mapping_id,omitempty"`
	RunID       string         `json:"run_id,omitempty"`
	ActorID     string         `json:"actor_id,omitempty"`
	ChatID      string         `json:"chat_id,omitempty"`
	At          *time.Time     `json:"at,omitempty"`
}

// SarText is one scalar value with its provenance. Dates are ISO 8601
// (YYYY-MM-DD), timestamps RFC 3339 with offset, decimals exact lexemes,
// booleans "true"/"false", codes neutral pivot codes.
type SarText struct {
	V string         `json:"v"`
	P *SarProvenance `json:"p,omitempty"`
}

func (t *SarText) Set() bool { return t != nil && t.V != "" }
func (t *SarText) Val() string {
	if t == nil {
		return ""
	}
	return t.V
}

type SarAddress struct {
	ID         string   `json:"id"`
	Type       *SarText `json:"type,omitempty"` // home, business, other
	Street     *SarText `json:"street,omitempty"`
	Number     *SarText `json:"number,omitempty"`
	PostalCode *SarText `json:"postal_code,omitempty"`
	City       *SarText `json:"city,omitempty"`
	Region     *SarText `json:"region,omitempty"`
	Country    *SarText `json:"country,omitempty"` // ISO 3166 alpha-2
}

type SarIdentification struct {
	ID           string   `json:"id"`
	Type         *SarText `json:"type,omitempty"` // passport, id_card, driving_licence, residence_permit, other
	Number       *SarText `json:"number,omitempty"`
	IssueCountry *SarText `json:"issue_country,omitempty"`
	IssuedBy     *SarText `json:"issued_by,omitempty"`
	IssueDate    *SarText `json:"issue_date,omitempty"`
	ExpiryDate   *SarText `json:"expiry_date,omitempty"`
	AttachmentID *SarText `json:"attachment_id,omitempty"`
}

// SarParty is a natural person, a legal entity or an unknown party.
type SarParty struct {
	ID    string    `json:"id"`
	Kind  *SarText  `json:"kind,omitempty"`  // person, entity, unknown
	Roles []SarText `json:"roles,omitempty"` // suspect, victim, client, beneficial_owner, director, signatory, counterparty, conductor, other
	// IsClient marks parties with a relationship at the reporting entity (goAML my_client).
	IsClient     *SarText `json:"is_client,omitempty"`
	Relationship *SarText `json:"relationship,omitempty"` // business_relationship, occasional_customer, prospect, none
	ClientSince  *SarText `json:"client_since,omitempty"`
	ClientRef    *SarText `json:"client_ref,omitempty"`
	// Person
	FirstName     *SarText  `json:"first_name,omitempty"`
	LastName      *SarText  `json:"last_name,omitempty"`
	BirthName     *SarText  `json:"birth_name,omitempty"`
	Aliases       []SarText `json:"aliases,omitempty"`
	Gender        *SarText  `json:"gender,omitempty"` // male, female, other, unknown
	BirthDate     *SarText  `json:"birth_date,omitempty"`
	BirthPlace    *SarText  `json:"birth_place,omitempty"`
	BirthCountry  *SarText  `json:"birth_country,omitempty"`
	Nationalities []SarText `json:"nationalities,omitempty"`
	Residence     *SarText  `json:"residence,omitempty"`
	Occupation    *SarText  `json:"occupation,omitempty"`
	Employer      *SarText  `json:"employer,omitempty"`
	TaxNumber     *SarText  `json:"tax_number,omitempty"`
	PEP           *SarText  `json:"pep,omitempty"` // yes, no, unknown
	PEPFunction   *SarText  `json:"pep_function,omitempty"`
	Sanctioned    *SarText  `json:"sanctioned,omitempty"` // yes, no, unknown
	SanctionLists *SarText  `json:"sanction_lists,omitempty"`
	// Entity
	Name                *SarText `json:"name,omitempty"`
	TradeName           *SarText `json:"trade_name,omitempty"`
	LegalForm           *SarText `json:"legal_form,omitempty"` // neutral: company, partnership, foundation, association, trust, sole_trader, public_body, other
	RegistrationNumber  *SarText `json:"registration_number,omitempty"`
	RegistrationCountry *SarText `json:"registration_country,omitempty"`
	RegistrationType    *SarText `json:"registration_type,omitempty"` // siren, rcs, uid, other, unknown
	IncorporationDate   *SarText `json:"incorporation_date,omitempty"`
	BusinessActivity    *SarText `json:"business_activity,omitempty"`
	Website             *SarText `json:"website,omitempty"`
	// Both
	Addresses       []SarAddress        `json:"addresses,omitempty"`
	Phones          []SarText           `json:"phones,omitempty"`
	Emails          []SarText           `json:"emails,omitempty"`
	Identifications []SarIdentification `json:"identifications,omitempty"`
	Description     *SarText            `json:"description,omitempty"`
}

// SarInstrument is a financial support: bank account, card, wallet...
type SarInstrument struct {
	ID              string   `json:"id"`
	Kind            *SarText `json:"kind,omitempty"` // bank_account, card, prepaid_card, e_wallet, crypto_wallet, cheque, contract, cash, other
	HeldAtReporter  *SarText `json:"held_at_reporter,omitempty"`
	IBAN            *SarText `json:"iban,omitempty"`
	AccountNumber   *SarText `json:"account_number,omitempty"`
	BIC             *SarText `json:"bic,omitempty"`
	Institution     *SarText `json:"institution,omitempty"`
	InstitutionCode *SarText `json:"institution_code,omitempty"`
	Country         *SarText `json:"country,omitempty"`
	Currency        *SarText `json:"currency,omitempty"`
	Category        *SarText `json:"category,omitempty"` // personal, business
	Status          *SarText `json:"status,omitempty"`   // active, closed, blocked, dormant
	Opened          *SarText `json:"opened,omitempty"`
	Closed          *SarText `json:"closed,omitempty"`
	Balance         *SarText `json:"balance,omitempty"`
	BalanceDate     *SarText `json:"balance_date,omitempty"`
	CardNumber      *SarText `json:"card_number,omitempty"`
	WalletAddress   *SarText `json:"wallet_address,omitempty"`
	Reference       *SarText `json:"reference,omitempty"` // other identifier
	Description     *SarText `json:"description,omitempty"`
}

// SarLink relates two parties, or a party and an instrument.
type SarLink struct {
	ID           string   `json:"id"`
	Kind         *SarText `json:"kind,omitempty"` // holder, co_holder, signatory, director, beneficial_owner, shareholder, family, associate, employee, other
	PartyID      *SarText `json:"party_id,omitempty"`
	ToPartyID    *SarText `json:"to_party_id,omitempty"`
	InstrumentID *SarText `json:"instrument_id,omitempty"`
	Details      *SarText `json:"details,omitempty"`
	Since        *SarText `json:"since,omitempty"`
}

// SarOperation is one unit operation, in its original currency.
type SarOperation struct {
	ID                     string   `json:"id"`
	Reference              *SarText `json:"reference,omitempty"`
	Timestamp              *SarText `json:"timestamp,omitempty"` // RFC 3339 with offset
	ValueDate              *SarText `json:"value_date,omitempty"`
	Amount                 *SarText `json:"amount,omitempty"`
	Currency               *SarText `json:"currency,omitempty"`
	Direction              *SarText `json:"direction,omitempty"` // outgoing (debit of the client), incoming (credit), internal
	Type                   *SarText `json:"type,omitempty"`      // transfer, cash_deposit, cash_withdrawal, card_payment, cheque, crypto, direct_debit, other
	Channel                *SarText `json:"channel,omitempty"`   // branch, online, atm, mobile, other
	Status                 *SarText `json:"status,omitempty"`    // executed, attempted, refused, pending
	PartyID                *SarText `json:"party_id,omitempty"`  // client side party
	InstrumentID           *SarText `json:"instrument_id,omitempty"`
	CounterpartyID         *SarText `json:"counterparty_id,omitempty"`
	CounterpartyInstrument *SarText `json:"counterparty_instrument_id,omitempty"`
	CounterpartyCountry    *SarText `json:"counterparty_country,omitempty"`
	Location               *SarText `json:"location,omitempty"`
	Description            *SarText `json:"description,omitempty"`
	Suspicious             *SarText `json:"suspicious,omitempty"`
	Selected               *SarText `json:"selected,omitempty"` // false excludes the operation from the reports
}

type SarAttachment struct {
	ID           string   `json:"id"`
	FileID       string   `json:"file_id"`
	Name         *SarText `json:"name,omitempty"`
	Mime         *SarText `json:"mime,omitempty"`
	SHA256       *SarText `json:"sha256,omitempty"`
	Size         *SarText `json:"size,omitempty"`
	DocumentType *SarText `json:"document_type,omitempty"` // identity, statement, contract, correspondence, invoice, screening, other
	Date         *SarText `json:"date,omitempty"`
	Description  *SarText `json:"description,omitempty"`
	Selected     *SarText `json:"selected,omitempty"` // joins the export
	PartyID      *SarText `json:"party_id,omitempty"`
}

type SarSuspicion struct {
	Analysis       *SarText  `json:"analysis,omitempty"` // human analysis, protected
	Themes         []SarText `json:"themes,omitempty"`   // see SarThemes
	Summary        *SarText  `json:"summary,omitempty"`
	ActionsTaken   *SarText  `json:"actions_taken,omitempty"`
	DetectionDate  *SarText  `json:"detection_date,omitempty"`
	Urgent         *SarText  `json:"urgent,omitempty"`
	JudicialOrigin *SarText  `json:"judicial_origin,omitempty"`
}

type SarPriorReport struct {
	ID        string   `json:"id"`
	FIU       *SarText `json:"fiu,omitempty"` // TRACFIN, CRF-LU, MROS...
	Reference *SarText `json:"reference,omitempty"`
	Date      *SarText `json:"date,omitempty"`
	Relation  *SarText `json:"relation,omitempty"` // complement, related
}

type SarPivot struct {
	CaseID       string           `json:"case_id"`
	OrgID        string           `json:"org_id"`
	Revision     int64            `json:"revision"`
	Parties      []SarParty       `json:"parties"`
	Instruments  []SarInstrument  `json:"instruments"`
	Links        []SarLink        `json:"links"`
	Operations   []SarOperation   `json:"operations"`
	Attachments  []SarAttachment  `json:"attachments"`
	Suspicion    SarSuspicion     `json:"suspicion"`
	PriorReports []SarPriorReport `json:"prior_reports"`
	Validated    *SarValidation   `json:"validated,omitempty"` // analyst validation of the facts
	UpdatedAt    time.Time        `json:"updated_at"`
	UpdatedBy    string           `json:"updated_by"`
}

type SarValidation struct {
	Revision int64     `json:"revision"`
	ActorID  string    `json:"actor_id"`
	At       time.Time `json:"at"`
}

// SarExtValue holds the values of one extension field instance.
type SarExtValue struct {
	Values []string       `json:"values"`
	P      *SarProvenance `json:"p,omitempty"`
}

// SarExtension keys are "<field>" for report-level fields and
// "<field>#<pivot object id>" for per-object fields.
type SarExtension map[string]SarExtValue

type SarIssue struct {
	Path     string `json:"path"` // location in the format tree, or pivot/extension path
	Code     string `json:"code"` // stable code (REJ-xxx for LU asserts)
	Message  string `json:"message"`
	Severity string `json:"severity"`           // error, warning
	Layer    string `json:"layer"`              // pivot, extension, structure, rule
	Edit     string `json:"edit,omitempty"`     // pivot/... or extension/... path to fix
	Document int    `json:"document,omitempty"` // index of the split document (goAML LU > 500 transactions)
}

const (
	SarReportDraft      = "draft"
	SarReportGenerating = "generating"
	SarReportReady      = "ready"   // valid
	SarReportInvalid    = "invalid" // generated with blocking issues
	SarReportApproved   = "approved"
)

type SarReport struct {
	ID               string       `json:"id"`
	CaseID           string       `json:"case_id"`
	OrgID            string       `json:"org_id"`
	FormatKey        string       `json:"format_key"` // e.g. goaml:LU:5.0.2
	Revision         int64        `json:"revision"`
	PivotRevision    int64        `json:"pivot_revision"`
	Status           string       `json:"status"`
	ReportType       string       `json:"report_type"`
	Extension        SarExtension `json:"extension"`
	Issues           []SarIssue   `json:"issues"`
	ApprovedRevision int64        `json:"approved_revision,omitempty"`
	ApprovedBy       string       `json:"approved_by,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	UpdatedBy        string       `json:"updated_by"`
}

// SarRevision is the trace of every change to a pivot or a report.
type SarRevision struct {
	ID        string          `json:"id"`
	OrgID     string          `json:"org_id"`
	CaseID    string          `json:"case_id"`
	Object    string          `json:"object"` // pivot or report:<id>
	Revision  int64           `json:"revision"`
	Origin    string          `json:"origin"` // manual, agent, chat, mapping, settings
	ActorID   string          `json:"actor_id"`
	RunID     string          `json:"run_id,omitempty"`
	ChatID    string          `json:"chat_id,omitempty"`
	Summary   string          `json:"summary"`
	Patch     json.RawMessage `json:"patch,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// SarPatchOp edits the pivot or an extension by stable path.
// Paths: pivot/parties/<id>/first_name, pivot/parties/<id>/nationalities/0,
// pivot/operations/<id>, extension/<format_key>/<field>[#<object id>].
type SarPatchOp struct {
	Op    string          `json:"op"` // set, add, remove
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"` // string for scalars, []string for extension, object for add
}

type SarRunStep struct {
	Name        string     `json:"name"` // pivot:parties, pivot:review, ext:goaml:LU:5.0.2...
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	Issues      []SarIssue `json:"issues,omitempty"`
	Explanation string     `json:"explanation,omitempty"`
}

type SarRun struct {
	ID          string       `json:"id"`
	OrgID       string       `json:"org_id"`
	CaseID      string       `json:"case_id"`
	ActorID     string       `json:"actor_id"`
	Formats     []string     `json:"formats"`
	Mode        string       `json:"mode"`   // pivot, reports, all
	Status      string       `json:"status"` // queued, running, completed, failed
	Steps       []SarRunStep `json:"steps"`
	CallsUsed   int          `json:"calls_used"`
	CallsBudget int          `json:"calls_budget"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type SarRunArgs struct {
	OrgID   string `json:"org_id"`
	CaseID  string `json:"case_id"`
	RunID   string `json:"run_id"`
	ActorID string `json:"actor_id"`
}

func (SarRunArgs) Kind() string { return "sar_generation" }

// SarMemoryEntry is a deduplicated error seen and corrected for one format.
type SarMemoryEntry struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	FormatKey string    `json:"format_key"`
	Code      string    `json:"code"`
	Path      string    `json:"path"` // normalized, without indexes
	Cause     string    `json:"cause"`
	Fix       string    `json:"fix"`
	Frequency int       `json:"frequency"`
	Corrected int       `json:"corrected"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type SarChatMessage struct {
	ID             string       `json:"id"`
	OrgID          string       `json:"org_id"`
	CaseID         string       `json:"case_id"`
	ReportID       string       `json:"report_id"`
	Role           string       `json:"role"` // user, assistant
	Content        string       `json:"content"`
	Patch          []SarPatchOp `json:"patch,omitempty"`
	Status         string       `json:"status,omitempty"` // proposed, applied, rejected, refused, failed
	Issues         []SarIssue   `json:"issues,omitempty"`
	PivotRevision  int64        `json:"pivot_revision,omitempty"`
	ReportRevision int64        `json:"report_revision,omitempty"`
	ActorID        string       `json:"actor_id"`
	CreatedAt      time.Time    `json:"created_at"`
}

// SarTableMapping configures once per organization how a data model table
// feeds the pivot. Fields map pivot fields to source fields of the table.
type SarTableMapping struct {
	ID     string            `json:"id"`
	Table  string            `json:"table"`
	Entity string            `json:"entity"` // operation, person, entity, instrument
	Fields map[string]string `json:"fields"` // pivot field -> data model field (or "=constant")
	// Codes translate client values into neutral pivot codes, per pivot field.
	Codes map[string]map[string]string `json:"codes,omitempty"`
	// AmountUnit is "major" (default) or "minor" (cents) for operation.amount and balance.
	AmountUnit string `json:"amount_unit,omitempty"`
	// Timezone used when a source timestamp has no offset (default Europe/Paris).
	Timezone string `json:"timezone,omitempty"`
}

type SarConfiguration struct {
	OrgID    string                         `json:"org_id"`
	Mappings []SarTableMapping              `json:"mappings"`
	Settings map[string]map[string][]string `json:"settings"` // format key -> settings field -> values
	// Rates are explicit exchange rates to the reporting currencies (e.g. "USD/EUR": "0.92").
	Rates     map[string]string `json:"rates,omitempty"`
	Revision  int64             `json:"revision"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy string            `json:"updated_by"`
}

// Prov returns the provenance of a possibly absent value.
func (t *SarText) Prov() *SarProvenance {
	if t == nil {
		return nil
	}
	return t.P
}
