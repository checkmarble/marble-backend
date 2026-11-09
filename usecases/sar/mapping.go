package sar

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// Deterministic mapping of the client data model into the pivot. The table
// mappings are configured once per organization (models.SarTableMapping).
// They extend the TRACFIN mapping mechanics: same collected sources (ingested
// objects, decision snapshots, case files), exact values, explicit units and
// timezones, code translation, provenance down to the source field.

// object is one data model record found in the collected context.
type object struct {
	sourceID string
	table    string
	id       string
	data     map[string]any
	locator  string // JSON pointer prefix of the data in the source content
}

func objects(sources []models.TracfinAISource) []object {
	var out []object
	seen := map[string]bool{}
	add := func(o object) {
		k := o.table + ":" + o.id
		if o.id == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, o)
	}
	for _, s := range sources {
		switch s.Kind {
		case "ingested_object":
			var c struct {
				Table  string `json:"table"`
				ID     string `json:"id"`
				Object struct {
					Data map[string]any `json:"Data"`
				} `json:"object"`
			}
			if json.Unmarshal(s.Content, &c) == nil {
				add(object{sourceID: s.ID, table: c.Table, id: c.ID, data: c.Object.Data, locator: "/object/Data"})
			}
		case "decision_snapshot":
			var c struct {
				Type     string          `json:"historical_type"`
				Snapshot json.RawMessage `json:"snapshot"`
			}
			if json.Unmarshal(s.Content, &c) != nil {
				continue
			}
			var data map[string]any
			dec := json.NewDecoder(strings.NewReader(string(c.Snapshot)))
			dec.UseNumber()
			if dec.Decode(&data) != nil {
				continue
			}
			id, _ := data["object_id"].(string)
			add(object{sourceID: s.ID, table: c.Type, id: id, data: data, locator: "/snapshot"})
		}
	}
	return out
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return new(big.Float).SetFloat64(x).Text('f', -1)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

func timestamp(raw string, loc *time.Location) (string, bool) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.Format(time.RFC3339), true
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t.Format(time.RFC3339), true
		}
	}
	return "", false
}

func minorToMajor(v string) (string, bool) {
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return "", false
	}
	return Round(new(big.Rat).Quo(r, big.NewRat(100, 1)), 2), true
}

// MapResult is the prefill produced by the mapping, with its diagnostics.
type MapResult struct {
	Pivot       models.SarPivot   `json:"pivot"`
	Diagnostics []models.SarIssue `json:"diagnostics"`
}

// MapSources builds a pivot from the collected sources and the mappings.
func MapSources(cfg models.SarConfiguration, caseID, orgID string, sources []models.TracfinAISource) MapResult {
	res := MapResult{Pivot: models.SarPivot{CaseID: caseID, OrgID: orgID}}
	objs := objects(sources)
	byTable := map[string][]models.SarTableMapping{}
	for _, m := range cfg.Mappings {
		byTable[m.Table] = append(byTable[m.Table], m)
	}
	ref := map[string]string{} // table:objectID -> pivot id
	prefix := map[string]string{"operation": "op", "person": "p", "entity": "p", "instrument": "acc"}
	type pending struct {
		m   models.SarTableMapping
		o   object
		pid string
	}
	var work []pending
	for _, o := range objs {
		for _, m := range byTable[o.table] {
			pid := NewID(prefix[m.Entity], o.table+":"+o.id)
			ref[o.table+":"+o.id] = pid
			work = append(work, pending{m, o, pid})
		}
	}
	// Parties and instruments first so that operations can reference them.
	sort.SliceStable(work, func(i, j int) bool { return work[i].m.Entity != "operation" && work[j].m.Entity == "operation" })
	for _, w := range work {
		m, o := w.m, w.o
		loc := Paris
		if m.Timezone != "" {
			if l, err := time.LoadLocation(m.Timezone); err == nil {
				loc = l
			}
		}
		values := map[string]*models.SarText{}
		lists := map[string][]models.SarText{}
		fields := make([]string, 0, len(m.Fields))
		for k := range m.Fields {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		for _, target := range fields {
			src := m.Fields[target]
			var val string
			prov := &models.SarProvenance{Kind: models.SarProvDataModel, MappingID: m.ID}
			switch {
			case strings.HasPrefix(src, "="):
				val = strings.TrimPrefix(src, "=")
				prov.Explanation = "constante de la configuration"
			case strings.HasPrefix(src, "@"):
				// @table:field references another mapped object.
				table, field, _ := strings.Cut(strings.TrimPrefix(src, "@"), ":")
				id := text(o.data[field])
				val = ref[table+":"+id]
				prov.Sources = []models.SarSourceRef{{SourceID: o.sourceID, Locator: o.locator + "/" + field, Table: o.table, ObjectID: o.id, Field: field}}
				prov.Explanation = "référence vers " + table + " " + id
				if val == "" && id != "" {
					res.Diagnostics = append(res.Diagnostics, models.SarIssue{Path: "mapping/" + m.ID + "/" + target, Code: "MAP_REFERENCE_UNRESOLVED", Message: fmt.Sprintf("%s %s n'est pas dans le contexte collecté", table, id), Severity: "warning", Layer: "pivot"})
				}
			default:
				raw, ok := o.data[src]
				if !ok || raw == nil {
					continue
				}
				val = text(raw)
				prov.Sources = []models.SarSourceRef{{SourceID: o.sourceID, Locator: o.locator + "/" + src, Table: o.table, ObjectID: o.id, Field: src}}
			}
			if val == "" {
				continue
			}
			if codes, ok := m.Codes[target]; ok {
				if c, ok := codes[val]; ok {
					prov.Explanation = strings.TrimSpace(prov.Explanation + " code " + val + " traduit en " + c)
					val = c
				}
			}
			switch target {
			case "amount", "balance":
				if m.AmountUnit == "minor" {
					v, ok := minorToMajor(val)
					if !ok {
						continue
					}
					prov.Explanation = strings.TrimSpace(prov.Explanation + " montant en unités mineures divisé par 100")
					val = v
				}
				if strings.HasPrefix(val, "-") && target == "amount" {
					val = strings.TrimPrefix(val, "-")
					if _, has := m.Fields["direction"]; !has {
						values["direction"] = &models.SarText{V: "outgoing", P: &models.SarProvenance{Kind: models.SarProvCalculated, Explanation: "montant négatif : opération sortante"}}
					}
				}
			case "timestamp":
				v, ok := timestamp(val, loc)
				if !ok {
					res.Diagnostics = append(res.Diagnostics, models.SarIssue{Path: "mapping/" + m.ID + "/timestamp", Code: "MAP_TIMESTAMP", Message: "horodatage illisible : " + val, Severity: "warning", Layer: "pivot"})
					continue
				}
				if v != val {
					prov.Explanation = strings.TrimSpace(prov.Explanation + " horodatage normalisé (fuseau " + loc.String() + ")")
				}
				val = v
			}
			if f, ok := FieldByPath(entityCollection(m.Entity) + "/*/" + target); ok && f.Kind == "date" {
				if ts, ok := timestamp(val, loc); ok {
					t, _ := time.Parse(time.RFC3339, ts)
					if d := t.In(loc).Format("2006-01-02"); d != val {
						prov.Explanation = strings.TrimSpace(prov.Explanation + " date civile (fuseau " + loc.String() + ")")
						val = d
					}
				}
			}
			if f, ok := FieldByPath(entityCollection(m.Entity) + "/*/" + target); ok && f.List {
				lists[target] = append(lists[target], models.SarText{V: val, P: prov})
			} else {
				values[target] = &models.SarText{V: val, P: prov}
			}
		}
		assign(&res.Pivot, m.Entity, w.pid, values, lists)
	}
	// Case files become pivot attachments (selection is decided later).
	for _, s := range sources {
		if s.Kind != "attachment" {
			continue
		}
		var c struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
			Mime   string `json:"mime"`
		}
		if json.Unmarshal(s.Content, &c) != nil || c.ID == "" {
			continue
		}
		calc := func(v, why string) *models.SarText {
			if v == "" {
				return nil
			}
			return &models.SarText{V: v, P: &models.SarProvenance{Kind: models.SarProvCalculated, Explanation: why, Sources: []models.SarSourceRef{{SourceID: s.ID}}}}
		}
		size := ""
		if c.Size > 0 {
			size = fmt.Sprint(c.Size)
		}
		res.Pivot.Attachments = append(res.Pivot.Attachments, models.SarAttachment{ID: NewID("att", c.ID), FileID: c.ID, Name: calc(c.Name, "nom du fichier du case"), Mime: calc(c.Mime, "type détecté sur le contenu"), SHA256: calc(c.SHA256, "empreinte du contenu"), Size: calc(size, "taille du fichier")})
	}
	return res
}

func entityCollection(entity string) string {
	switch entity {
	case "operation":
		return "operations"
	case "instrument":
		return "instruments"
	}
	return "parties"
}

func assign(p *models.SarPivot, entity, id string, v map[string]*models.SarText, lists map[string][]models.SarText) {
	switch entity {
	case "operation":
		p.Operations = append(p.Operations, models.SarOperation{ID: id, Reference: v["reference"], Timestamp: v["timestamp"], ValueDate: v["value_date"], Amount: v["amount"], Currency: v["currency"], Direction: v["direction"], Type: v["type"], Channel: v["channel"], Status: v["status"], PartyID: v["party_id"], InstrumentID: v["instrument_id"], CounterpartyID: v["counterparty_id"], CounterpartyInstrument: v["counterparty_instrument_id"], CounterpartyCountry: v["counterparty_country"], Location: v["location"], Description: v["description"]})
	case "instrument":
		p.Instruments = append(p.Instruments, models.SarInstrument{ID: id, Kind: v["kind"], HeldAtReporter: v["held_at_reporter"], IBAN: v["iban"], AccountNumber: v["account_number"], BIC: v["bic"], Institution: v["institution"], Country: v["country"], Currency: v["currency"], Status: v["status"], Opened: v["opened"], Closed: v["closed"], Balance: v["balance"], BalanceDate: v["balance_date"], CardNumber: v["card_number"], WalletAddress: v["wallet_address"], Reference: v["reference"]})
		if owner := v["owner_id"]; owner != nil {
			p.Links = append(p.Links, models.SarLink{ID: NewID("l", id+owner.V), Kind: &models.SarText{V: "holder", P: owner.P}, PartyID: owner, InstrumentID: &models.SarText{V: id, P: owner.P}})
		}
	case "person", "entity":
		kind := &models.SarText{V: entity, P: &models.SarProvenance{Kind: models.SarProvDataModel, Explanation: "type de la table mappée"}}
		party := models.SarParty{ID: id, Kind: kind, IsClient: v["is_client"], ClientRef: v["client_ref"], ClientSince: v["client_since"], FirstName: v["first_name"], LastName: v["last_name"], BirthName: v["birth_name"], Gender: v["gender"], BirthDate: v["birth_date"], BirthPlace: v["birth_place"], BirthCountry: v["birth_country"], Residence: v["residence"], Occupation: v["occupation"], TaxNumber: v["tax_number"], PEP: v["pep"], Name: v["name"], TradeName: v["trade_name"], LegalForm: v["legal_form"], RegistrationNumber: v["registration_number"], RegistrationCountry: v["registration_country"], RegistrationType: v["registration_type"], IncorporationDate: v["incorporation_date"], BusinessActivity: v["business_activity"], Website: v["website"], Nationalities: lists["nationalities"], Phones: lists["phones"], Emails: lists["emails"]}
		if v["street"] != nil || v["city"] != nil {
			party.Addresses = []models.SarAddress{{ID: "a1", Street: v["street"], PostalCode: v["postal_code"], City: v["city"], Country: v["country"]}}
		}
		p.Parties = append(p.Parties, party)
	}
}
