package tracfin

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Bounds of the understanding note: the summary, any other string, and the
// length of each array.
const (
	maxNoteSummary = 4000
	maxNoteString  = 1000
	maxNoteItems   = 300
)

// Roles of an entity and statuses of a suspicion in the understanding note.
var (
	entityRoles = []string{
		"suspect", "probable_accomplice", "beneficiary", "originator", "counterparty",
		"possible_victim", "intermediary", "legal_representative", "other",
	}
	suspicionStatuses = []string{"asserted", "hypothesis"}
	basisKinds        = []string{"analysis", "data", "hypothesis"}
)

// understandingNote is the output of the understanding step: one shared,
// grounded reading of the case. It writes nothing in the document; the planner
// hands it to every later step.
type understandingNote struct {
	Summary  string `json:"summary"`
	Typology string `json:"typology"`
	Entities []struct {
		Object      core.ObjectRef `json:"object"`
		Role        string         `json:"role"`
		Description string         `json:"description"`
		Basis       []noteBasis    `json:"basis"`
	} `json:"entities"`
	Links []struct {
		From   core.ObjectRef `json:"from"`
		To     core.ObjectRef `json:"to"`
		Nature string         `json:"nature"`
		Basis  []noteBasis    `json:"basis"`
	} `json:"links"`
	Facts []struct {
		Statement string           `json:"statement"`
		Objects   []core.ObjectRef `json:"objects"`
		Basis     []noteBasis      `json:"basis"`
	} `json:"facts"`
	Suspicions []struct {
		Qualification string `json:"qualification"`
		Status        string `json:"status"`
		Quote         string `json:"quote"`
	} `json:"suspicions"`
	KeyOperations []struct {
		Objects     []core.ObjectRef `json:"objects"`
		Description string           `json:"description"`
	} `json:"key_operations"`
	Unknowns []struct {
		Topic   string           `json:"topic"`
		Objects []core.ObjectRef `json:"objects"`
		Why     string           `json:"why"`
	} `json:"unknowns"`
}

type noteBasis struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
}

func understandingSchema() jsonschema.Schema {
	enum := func(vals []string) string { return `["` + strings.Join(vals, `","`) + `"]` }
	const obj = `{"type":"object","additionalProperties":false,"required":["table","object_id"],"properties":{"table":{"type":"string"},"object_id":{"type":"string"}}}`
	basis := `{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","reference"],"properties":{` +
		`"kind":{"type":"string","enum":` + enum(basisKinds) + `,"description":"analysis = phrase copied verbatim from the analyst's text; data = table/object_id.field read; hypothesis = your reasoning, stated as a hypothesis"},` +
		`"reference":{"type":"string"}}}}`
	objs := `{"type":"array","items":` + obj + `}`
	raw := `{"type":"object","additionalProperties":false,"required":["summary","typology","entities","links","facts","suspicions","key_operations","unknowns"],"properties":{
 "summary":{"type":"string"},
 "typology":{"type":"string"},
 "entities":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["object","role","description","basis"],"properties":{
   "object":` + obj + `,"role":{"type":"string","enum":` + enum(entityRoles) + `},"description":{"type":"string"},"basis":` + basis + `}}},
 "links":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["from","to","nature","basis"],"properties":{
   "from":` + obj + `,"to":` + obj + `,"nature":{"type":"string"},"basis":` + basis + `}}},
 "facts":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["statement","objects","basis"],"properties":{
   "statement":{"type":"string"},"objects":` + objs + `,"basis":` + basis + `}}},
 "suspicions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["qualification","status","quote"],"properties":{
   "qualification":{"type":"string"},"status":{"type":"string","enum":` + enum(suspicionStatuses) + `},"quote":{"type":"string"}}}},
 "key_operations":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["objects","description"],"properties":{
   "objects":` + objs + `,"description":{"type":"string"}}}},
 "unknowns":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["topic","objects","why"],"properties":{
   "topic":{"type":"string"},"objects":` + objs + `,"why":{"type":"string"}}}}}}`
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		panic("understanding schema: " + err.Error()) // constant above: a programming error
	}
	return s
}

// minQuoteWords is the least number of words of a phrase quoted from the
// analyst's text: a shorter one ("de", "le client") proves nothing.
const minQuoteWords = 3

// cleanUnderstanding decodes an understanding note and checks it against the
// selection and the analyst's text. The note is a reading aid, so the check is
// best-effort: an entity, link, fact, suspicion, key operation or unknown whose
// references, quote or bounds fail is dropped (and listed in dropped), the rest
// is kept. Only a note that does not parse, or whose summary, typology or array
// sizes exceed the hard bounds, is refused.
func cleanUnderstanding(raw json.RawMessage, sc core.StructuredContext) (cleaned json.RawMessage, dropped []string, err error) {
	var n understandingNote
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&n); err != nil {
		return nil, nil, errors.Wrap(err, "unreadable understanding note")
	}
	v := noteValidator{sc: sc, text: foldAnalystText(sc.AnalystText)}
	v.bounded("summary", n.Summary, maxNoteSummary)
	v.bounded("typology", n.Typology, maxNoteString)
	v.size("entities", len(n.Entities))
	v.size("links", len(n.Links))
	v.size("facts", len(n.Facts))
	v.size("suspicions", len(n.Suspicions))
	v.size("key_operations", len(n.KeyOperations))
	v.size("unknowns", len(n.Unknowns))
	if len(v.problems) > 0 {
		return nil, nil, errors.New("invalid understanding note: " + strings.Join(v.problems, "; "))
	}

	// keep runs check on one item: the item is kept when it adds no problem.
	keep := func(at string, check func()) bool {
		before := len(v.problems)
		check()
		if len(v.problems) == before {
			return true
		}
		dropped = append(dropped, "dropped "+at+": "+strings.Join(v.problems[before:], "; "))
		v.problems = v.problems[:before]
		return false
	}

	out := n
	out.Entities = out.Entities[:0:0]
	out.Links = out.Links[:0:0]
	out.Facts = out.Facts[:0:0]
	out.Suspicions = out.Suspicions[:0:0]
	out.KeyOperations = out.KeyOperations[:0:0]
	out.Unknowns = out.Unknowns[:0:0]
	for i, e := range n.Entities {
		if keep("entities["+itoa(i)+"]", func() {
			v.object("object", e.Object)
			v.enum("role", e.Role, entityRoles)
			v.bounded("description", e.Description, maxNoteString)
			v.basis("", e.Basis)
		}) {
			out.Entities = append(out.Entities, e)
		}
	}
	for i, l := range n.Links {
		if keep("links["+itoa(i)+"]", func() {
			v.object("from", l.From)
			v.object("to", l.To)
			v.bounded("nature", l.Nature, maxNoteString)
			v.basis("", l.Basis)
		}) {
			out.Links = append(out.Links, l)
		}
	}
	for i, f := range n.Facts {
		if keep("facts["+itoa(i)+"]", func() {
			v.bounded("statement", f.Statement, maxNoteString)
			v.objects("objects", f.Objects)
			v.basis("", f.Basis)
		}) {
			out.Facts = append(out.Facts, f)
		}
	}
	for i, s := range n.Suspicions {
		if keep("suspicions["+itoa(i)+"]", func() {
			v.bounded("qualification", s.Qualification, maxNoteString)
			v.bounded("quote", s.Quote, maxNoteString)
			v.enum("status", s.Status, suspicionStatuses)
			if s.Status == "asserted" && !v.inText(s.Quote) {
				v.fail("quote: an asserted suspicion needs a phrase of at least " + itoa(minQuoteWords) + " words copied verbatim from the analyst's text (analysis or extra_info)")
			}
		}) {
			out.Suspicions = append(out.Suspicions, s)
		}
	}
	for i, k := range n.KeyOperations {
		if keep("key_operations["+itoa(i)+"]", func() {
			v.objects("objects", k.Objects)
			v.bounded("description", k.Description, maxNoteString)
		}) {
			out.KeyOperations = append(out.KeyOperations, k)
		}
	}
	for i, u := range n.Unknowns {
		if keep("unknowns["+itoa(i)+"]", func() {
			v.bounded("topic", u.Topic, maxNoteString)
			v.objects("objects", u.Objects)
			v.bounded("why", u.Why, maxNoteString)
		}) {
			out.Unknowns = append(out.Unknowns, u)
		}
	}
	if len(dropped) == 0 {
		return raw, nil, nil
	}
	cleaned, err = json.Marshal(out)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode understanding note")
	}
	return cleaned, dropped, nil
}

// CleanStructured implements core.StructuredCleaner: the understanding note
// keeps its valid items. Other kinds are returned as they are.
func (f *format) CleanStructured(kind string, raw json.RawMessage, sc core.StructuredContext) (json.RawMessage, []string, error) {
	if kind != StepUnderstanding {
		return raw, nil, nil
	}
	return cleanUnderstanding(raw, sc)
}

// noteValidator collects the problems of the item being checked.
type noteValidator struct {
	sc       core.StructuredContext
	text     string
	problems []string
}

func (v *noteValidator) fail(msg string) {
	v.problems = append(v.problems, msg)
}

func (v *noteValidator) size(name string, n int) {
	if n > maxNoteItems {
		v.fail(name + ": at most " + itoa(maxNoteItems) + " items")
	}
}

func (v *noteValidator) bounded(name, s string, limit int) {
	if utf8.RuneCountInString(s) > limit {
		v.fail(name + ": longer than " + itoa(limit) + " characters")
	}
}

func (v *noteValidator) enum(name, got string, allowed []string) {
	for _, a := range allowed {
		if a == got {
			return
		}
	}
	v.fail(name + ": unknown value " + quote(got) + " (" + strings.Join(allowed, ", ") + ")")
}

func (v *noteValidator) object(name string, ref core.ObjectRef) {
	for _, s := range v.sc.Selection {
		if s == ref {
			return
		}
	}
	v.fail(name + ": " + ref.Table + "/" + ref.ObjectID + " is not a selected object")
}

func (v *noteValidator) objects(name string, refs []core.ObjectRef) {
	if len(refs) > maxNoteItems {
		v.fail(name + ": at most " + itoa(maxNoteItems) + " items")
		return
	}
	for i, r := range refs {
		v.object(name+"["+itoa(i)+"]", r)
	}
}

func (v *noteValidator) basis(owner string, basis []noteBasis) {
	if len(basis) > maxNoteItems {
		v.fail("basis: at most " + itoa(maxNoteItems) + " items")
		return
	}
	for i, b := range basis {
		at := "basis[" + itoa(i) + "]"
		v.bounded(at+".reference", b.Reference, maxNoteString)
		switch b.Kind {
		case "analysis":
			if !v.inText(b.Reference) {
				v.fail(at + ".reference: an analysis basis must be a phrase of at least " + itoa(minQuoteWords) + " words copied verbatim from the analyst's text (analysis or extra_info): " + quote(b.Reference))
			}
		case "data":
			if !v.dataReference(b.Reference) {
				v.fail(at + ".reference: a data basis is table/object_id.field of a selected object, the field being one of its fields: " + quote(b.Reference))
			}
		case "hypothesis":
			if strings.TrimSpace(b.Reference) == "" {
				v.fail(at + ".reference: a hypothesis basis states the reasoning")
			}
		default:
			v.enum(at+".kind", b.Kind, basisKinds)
		}
	}
}

// dataReference tells whether ref is table/object_id.field of a selected object
// (the object id may itself hold dots: the field is what follows the object
// prefix) and, when the frozen snapshots are known, the field exists in the
// snapshot of that object.
func (v *noteValidator) dataReference(ref string) bool {
	for _, s := range v.sc.Selection {
		field, ok := strings.CutPrefix(ref, s.Table+"/"+s.ObjectID+".")
		if !ok || strings.TrimSpace(field) == "" {
			continue
		}
		if v.sc.Snapshots == nil || field == "object_id" {
			return true
		}
		if _, exists := v.sc.Snapshots[s][field]; exists {
			return true
		}
	}
	return false
}

// inText tells whether phrase (at least minQuoteWords words) appears in the
// analyst's text, compared case-, accent- and whitespace-insensitively.
func (v *noteValidator) inText(phrase string) bool {
	p := foldAnalystText(phrase)
	return len(strings.Fields(p)) >= minQuoteWords && strings.Contains(v.text, p)
}

var ligatureFold = strings.NewReplacer("œ", "oe", "æ", "ae", "ß", "ss", "’", "'", "‘", "'", "“", `"`, "”", `"`)

// foldAnalystText lowercases, strips diacritics and collapses whitespace.
func foldAnalystText(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(ligatureFold.Replace(strings.ToLower(out))), " ")
}

func itoa(n int) string { return strconv.Itoa(n) }
func quote(s string) string {
	if utf8.RuneCountInString(s) > 80 {
		s = string([]rune(s)[:80]) + "..."
	}
	return `"` + s + `"`
}
