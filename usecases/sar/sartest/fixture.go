// Package sartest holds a shared synthetic case for SAR tests: one pivot
// producing TRACFIN, goAML LU and goAML CH reports.
package sartest

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

func t(v string) *models.SarText {
	return &models.SarText{V: v, P: &models.SarProvenance{Kind: models.SarProvDataModel, Sources: []models.SarSourceRef{{SourceID: "object:transactions:1", Table: "transactions"}}}}
}
func l(v ...string) []models.SarText {
	out := make([]models.SarText, 0, len(v))
	for _, s := range v {
		out = append(out, *t(s))
	}
	return out
}

const CaseID = "3f1c2b9a-6d1e-4c7a-9b2f-0a1b2c3d4e5f"

// Now is fixed so that balance dates are recent for the MROS 14-day assert.
func Now() time.Time { return time.Now().UTC().Truncate(time.Second) }

func Pivot() *models.SarPivot {
	now := Now()
	recent := now.AddDate(0, 0, -2).Format("2006-01-02")
	return &models.SarPivot{
		CaseID: CaseID, OrgID: "org-1", Revision: 3,
		Parties: []models.SarParty{
			{ID: "p1", Kind: t("person"), Roles: l("suspect", "client"), IsClient: t("true"), Relationship: t("business_relationship"), ClientSince: t("2021-03-02"),
				FirstName: t("Jean"), LastName: t("Martin"), BirthName: t("Martin"), Gender: t("male"), BirthDate: t("1980-05-17"), BirthPlace: t("Lyon"), BirthCountry: t("FR"),
				Nationalities: l("FR"), Residence: t("FR"), Occupation: t("Consultant"), PEP: t("no"), Sanctioned: t("no"), Phones: l("+33612345678"), Emails: l("jean.martin@example.com"),
				Addresses:       []models.SarAddress{{ID: "a1", Type: t("home"), Number: t("12"), Street: t("rue de la Paix"), PostalCode: t("75002"), City: t("Paris"), Country: t("FR")}},
				Identifications: []models.SarIdentification{{ID: "i1", Type: t("passport"), Number: t("19AB12345"), IssueCountry: t("FR"), IssueDate: t("2019-01-10"), ExpiryDate: t("2029-01-09"), IssuedBy: t("Préfecture de Paris")}},
				Description:     t("Client particulier, revenus déclarés incompatibles avec les flux.")},
			{ID: "p2", Kind: t("entity"), Roles: l("counterparty"), Name: t("Alpha Trading Ltd"), LegalForm: t("company"), RegistrationNumber: t("HE123456"), RegistrationCountry: t("CY"), RegistrationType: t("other"),
				BusinessActivity: t("Négoce"), Addresses: []models.SarAddress{{ID: "a2", Type: t("business"), Street: t("Makariou Avenue 5"), PostalCode: t("1065"), City: t("Nicosia"), Country: t("CY")}}},
			{ID: "p3", Kind: t("person"), Roles: l("counterparty"), FirstName: t("Anna"), LastName: t("Weber"), Gender: t("female"), Nationalities: l("DE"),
				Addresses: []models.SarAddress{{ID: "a3", Type: t("home"), Street: t("Hauptstrasse 3"), PostalCode: t("10115"), City: t("Berlin"), Country: t("DE")}}},
		},
		Instruments: []models.SarInstrument{
			{ID: "acc1", Kind: t("bank_account"), HeldAtReporter: t("true"), IBAN: t("FR7630006000011234567890189"), BIC: t("BNPAFRPPXXX"), Institution: t("Banque Exemple"), Country: t("FR"), Currency: t("EUR"),
				Category: t("personal"), Status: t("active"), Opened: t("2021-03-02"), Balance: t("1520.35"), BalanceDate: t(recent), Reference: t("CLI-0001")},
			{ID: "acc2", Kind: t("bank_account"), IBAN: t("CY17002001280000001200527600"), BIC: t("BCYPCY2NXXX"), Institution: t("Bank of Cyprus"), Country: t("CY")},
			{ID: "acc3", Kind: t("bank_account"), IBAN: t("DE89370400440532013000"), BIC: t("COBADEFFXXX"), Institution: t("Commerzbank"), Country: t("DE")},
		},
		Links: []models.SarLink{
			{ID: "l1", Kind: t("holder"), PartyID: t("p1"), InstrumentID: t("acc1")},
			{ID: "l2", Kind: t("holder"), PartyID: t("p2"), InstrumentID: t("acc2")},
			{ID: "l3", Kind: t("holder"), PartyID: t("p3"), InstrumentID: t("acc3")},
		},
		Operations: []models.SarOperation{
			{ID: "op1", Reference: t("TRX-1001"), Timestamp: t("2026-10-01T09:15:00+02:00"), Amount: t("9500.00"), Currency: t("EUR"), Direction: t("outgoing"), Type: t("transfer"), Channel: t("online"), Status: t("executed"),
				PartyID: t("p1"), InstrumentID: t("acc1"), CounterpartyID: t("p2"), CounterpartyInstrument: t("acc2"), CounterpartyCountry: t("CY"), Description: t("Facture 2026-114"), Suspicious: t("true")},
			{ID: "op2", Reference: t("TRX-1002"), Timestamp: t("2026-10-08T16:40:00+02:00"), Amount: t("9800.00"), Currency: t("EUR"), Direction: t("outgoing"), Type: t("transfer"), Channel: t("online"), Status: t("executed"),
				PartyID: t("p1"), InstrumentID: t("acc1"), CounterpartyID: t("p2"), CounterpartyInstrument: t("acc2"), CounterpartyCountry: t("CY"), Description: t("Facture 2026-121"), Suspicious: t("true")},
			{ID: "op3", Reference: t("TRX-1003"), Timestamp: t("2026-10-20T11:05:00+02:00"), Amount: t("12000.00"), Currency: t("USD"), Direction: t("incoming"), Type: t("transfer"), Channel: t("online"), Status: t("executed"),
				PartyID: t("p1"), InstrumentID: t("acc1"), CounterpartyID: t("p3"), CounterpartyInstrument: t("acc3"), CounterpartyCountry: t("DE"), Description: t("Prêt familial"), Suspicious: t("true")},
		},
		Attachments: []models.SarAttachment{
			{ID: "att1", FileID: "6b0f0c8e-1111-4a3b-8c9d-0e1f2a3b4c5d", Name: t("releve-octobre.pdf"), Mime: t("application/pdf"), SHA256: t("00"), Size: t("8"), DocumentType: t("statement"), Date: t("2026-10-31"), Description: t("Relevé d'octobre"), Selected: t("true")},
		},
		Suspicion: models.SarSuspicion{
			Analysis:     &models.SarText{V: "Le client a émis deux virements fractionnés juste sous 10 000 EUR vers une société chypriote sans lien apparent avec son activité, puis reçu 12 000 USD d'une tierce personne. Les justificatifs fournis (factures) sont incohérents.", P: &models.SarProvenance{Kind: models.SarProvManual, ActorID: "user-1"}},
			Themes:       l("money_laundering", "fraud"),
			ActionsTaken: t("Compte placé sous surveillance renforcée, justificatifs demandés le 22/10/2026."),
		},
	}
}

func Settings() map[string]map[string][]string {
	return map[string]map[string][]string{
		"tracfin:FR:1.4.0": {"organisme_profession": {"TPI1"}, "organisme_libelle": {"Banque Exemple"}, "organisme_num_id_pro": {"12345"}, "organisme_voie": {"1 place de la Bourse"}, "organisme_code_postal": {"75002"},
			"organisme_ville": {"Paris"}, "organisme_tel": {"0140000000"}, "declarant_nom": {"Durand"}, "declarant_prenom": {"Claire"}, "declarant_tel": {"0140000001"}, "declarant_courriel": {"conformite@banque-exemple.fr"}},
		"goaml:LU:5.0.2":  {"rentity_id": {"1234"}, "reporting_user_code": {"CDURAND"}, "entity_reference_prefix": {"BEX-"}},
		"goaml:CH:2025.3": {"rentity_id": {"5678"}, "reporting_user_code": {"CDURAND"}, "entity_reference_prefix": {"BEX-"}, "location_address": {"Bahnhofstrasse 1"}, "location_city": {"Zürich"}, "location_zip": {"8001"}, "account_branch_canton": {"ZH"}},
	}
}

func Rates() map[string]string {
	return map[string]string{"USD/EUR": "0.92", "EUR/CHF": "0.94", "USD/CHF": "0.86"}
}

// AIExtension is what a simulated model answers for each format.
func AIExtension() map[string]models.SarExtension {
	ai := func(v ...string) models.SarExtValue {
		return models.SarExtValue{Values: v, P: &models.SarProvenance{Kind: models.SarProvAI, Explanation: "simulé", Sources: []models.SarSourceRef{{SourceID: "case"}}}}
	}
	return map[string]models.SarExtension{
		"tracfin:FR:1.4.0": {"cadre_legal": ai("CL02"), "nature_soupcon_penale": ai("NSP07", "NSP10")},
		"goaml:LU:5.0.2":   {"indicators": ai("002TS-TxnPattern", "002TS-InconsFundsOrigin", "007SO-MoneyLaundering", "005PU-BankAcct")},
		"goaml:CH:2025.3":  {"indicators": ai("0002M", "1004V", "2018G", "3007B")},
	}
}
