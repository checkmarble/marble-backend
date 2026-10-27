package tracfin

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/models"
)

// AttachmentContent contains server-verified metadata and content. The caller
// owns Reader and must close it when appropriate; the serializer never closes it.
type AttachmentContent struct {
	BlockID  string
	Reader   io.Reader
	Metadata models.TracfinAttachment
}

// XMLIndex maps absolute, one-based XML occurrence paths to stable draft fields.
// Omitted values have no path and are addressed by Diagnostics' stable refs.
type XMLIndex struct {
	Fields      map[string]models.TracfinFieldRef
	Diagnostics []models.TracfinDiagnostic
}

type XMLFieldError struct {
	Field   models.TracfinFieldRef
	Message string
}

func (e *XMLFieldError) Error() string {
	return fmt.Sprintf("XML export %s block %s value %s: %s", e.Field.FieldID, e.Field.BlockID, e.Field.ValueID, e.Message)
}

func checkXMLText(text string, ref models.TracfinFieldRef) error {
	if !utf8.ValidString(text) {
		return &XMLFieldError{Field: ref, Message: "Invalid UTF-8"}
	}
	for _, r := range text {
		if r != 9 && r != 10 && r != 13 && (r < 0x20 || r > 0xD7FF && r < 0xE000 || r > 0xFFFD && r < 0x10000 || r > 0x10FFFF) {
			return &XMLFieldError{Field: ref, Message: fmt.Sprintf("Forbidden XML character U+%04X", r)}
		}
	}
	return nil
}

func WriteXML(w io.Writer, draft models.TracfinDraft, attachments []AttachmentContent, c Catalog) (XMLIndex, error) {
	index := XMLIndex{Fields: map[string]models.TracfinFieldRef{}, Diagnostics: ValidateDraft(draft, c).Diagnostics}
	ids := localXMLIDs(draft.Document.Root)
	lexeme := func(b models.TracfinBlock, f Field, val models.TracfinValue) (string, bool) {
		if val.Origin != nil && val.Origin.Resolution == "conflict" {
			return "", false
		}
		if f.Coverage == "calculated" {
			text, ok := ids[b.ID]
			return text, ok
		}
		if val.Kind == "reference" {
			target, reason := referenceTarget(draft.Document.Root, b, f, val)
			if reason != "" {
				return "", false
			}
			text, ok := ids[target.ID]
			return text, ok
		}
		if f.ValueKind == "reference" {
			return "", false
		}
		return valueText(val)
	}
	enc := xml.NewEncoder(w)
	var block func(models.TracfinBlock, string) error
	var scalar func(models.TracfinBlock, Field, models.TracfinValue, string) error
	attributes := func(b models.TracfinBlock, parent, owner, path string) ([]xml.Attr, error) {
		var attrs []xml.Attr
		for _, id := range c.OrderedFields {
			f := c.Fields[id]
			if !f.Attribute || f.Parent != parent || f.Coverage == "excluded" {
				continue
			}
			values := b.Fields[id]
			if f.Coverage == "calculated" {
				values = []models.TracfinValue{{}}
			}
			for _, val := range values {
				if val.ParentValueID != owner {
					continue
				}
				text, ok := lexeme(b, f, val)
				if !ok {
					continue
				}
				ref := fieldRef(b, f, val)
				if err := checkXMLText(text, ref); err != nil {
					return nil, err
				}
				attrs = append(attrs, xml.Attr{Name: xml.Name{Local: f.Name}, Value: text})
				index.Fields[path+"/@"+f.Name] = ref
			}
		}
		return attrs, nil
	}
	scalar = func(b models.TracfinBlock, f Field, val models.TracfinValue, path string) error {
		text, ok := lexeme(b, f, val)
		if !ok {
			return nil
		}
		ref := fieldRef(b, f, val)
		if err := checkXMLText(text, ref); err != nil {
			return err
		}
		attrs, err := attributes(b, f.ID, val.ID, path)
		if err != nil {
			return err
		}
		start := xml.StartElement{Name: xml.Name{Local: f.Name}, Attr: attrs}
		if err = enc.EncodeToken(start); err != nil {
			return err
		}
		if err = enc.EncodeToken(xml.CharData(text)); err != nil {
			return err
		}
		index.Fields[path] = ref
		return enc.EncodeToken(start.End())
	}
	block = func(b models.TracfinBlock, path string) error {
		f, ok := c.Fields[b.CatalogID]
		if !ok || f.Kind != "block" || f.Coverage == "excluded" {
			return fmt.Errorf("unknown or excluded document block %s", b.CatalogID)
		}
		attrs, err := attributes(b, f.ID, "", path)
		if err != nil {
			return err
		}
		start := xml.StartElement{Name: xml.Name{Local: f.Name}, Attr: attrs}
		if err = enc.EncodeToken(start); err != nil {
			return err
		}
		index.Fields[path] = models.TracfinFieldRef{BlockID: b.ID, FieldID: f.ID}
		for _, id := range c.OrderedFields {
			child := c.Fields[id]
			if child.Parent != f.ID || child.Attribute || child.Coverage == "excluded" || !activeField(b, child, c) {
				continue
			}
			n := 0
			if child.Kind == "block" {
				for _, instance := range b.Children {
					if instance.CatalogID != id {
						continue
					}
					n++
					if err := block(instance, path+"/"+child.Name+"["+strconv.Itoa(n)+"]"); err != nil {
						return err
					}
				}
			} else {
				values := b.Fields[id]
				if child.Coverage == "calculated" {
					values = []models.TracfinValue{{}}
				}
				for _, value := range values {
					if _, ok := lexeme(b, child, value); !ok {
						continue
					}
					n++
					if err := scalar(b, child, value, path+"/"+child.Name+"["+strconv.Itoa(n)+"]"); err != nil {
						return err
					}
				}
			}
		}
		return enc.EncodeToken(start.End())
	}
	if err := block(draft.Document.Root, "/ds"); err != nil {
		return index, err
	}
	return index, enc.Flush()
}

// Persons have declaration-local IDs. Support IDs are local to their person
// context; separate occurrences may refer back to the same selected source.
func localXMLIDs(root models.TracfinBlock) map[string]string {
	ids := map[string]string{}
	var people []models.TracfinBlock
	WalkBlocks(root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/personnes/pp" || b.CatalogID == "/ds/personnes/pm" || b.CatalogID == "/ds/personnes/pnc" {
			people = append(people, b)
		}
	})
	sort.Slice(people, func(i, j int) bool { return people[i].ID < people[j].ID })
	for n, person := range people {
		ids[person.ID] = strconv.Itoa(n + 1)
		var supports []models.TracfinBlock
		WalkBlocks(person, func(b models.TracfinBlock) {
			if strings.Contains(b.CatalogID, "/referencesFinancieres/referenceFinanciere") {
				supports = append(supports, b)
			}
		})
		sort.Slice(supports, func(i, j int) bool { return supports[i].ID < supports[j].ID })
		for k, support := range supports {
			ids[support.ID] = strconv.Itoa(k + 1)
		}
	}
	return ids
}

// ValidateXML reads the actual supplied XML independently of WriteXML. It does
// not fabricate draft IDs: diagnostics identify the fixed profile field only.
func ValidateXML(r io.Reader, c Catalog) models.TracfinValidation {
	report := newValidation(c)
	dec := xml.NewDecoder(r)
	type frame struct {
		path      string
		text      strings.Builder
		children  int
		lastOrder int
		counts    map[string]int
		fields    map[string][]models.TracfinValue
		parent    *frame
	}
	var stack []*frame
	var nodes []*frame
	roots := 0
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			addValidation(&report, validationDiagnostic("XML_SYNTAX_ERROR", "xml_syntax", err.Error(), models.TracfinFieldRef{}))
			return report
		}
		switch t := token.(type) {
		case xml.StartElement:
			path := "/" + t.Name.Local
			if len(stack) > 0 {
				path = stack[len(stack)-1].path + path
				stack[len(stack)-1].children++
				stack[len(stack)-1].counts[path]++
			} else {
				roots++
			}
			f, ok := c.Fields[path]
			if !ok || f.Coverage == "excluded" || t.Name.Space != "" {
				addValidation(&report, validationDiagnostic("UNKNOWN_XML_ELEMENT", "internal_structure", "Element is not part of this profile", models.TracfinFieldRef{FieldID: path}))
			}
			if len(stack) > 0 && ok {
				parent := stack[len(stack)-1]
				if f.Order < parent.lastOrder {
					addValidation(&report, validationDiagnostic("XML_ELEMENT_ORDER", "internal_structure", "Element is out of profile order", models.TracfinFieldRef{FieldID: path}))
				}
				parent.lastOrder = f.Order
			}
			fr := &frame{path: path, lastOrder: -1, counts: map[string]int{}, fields: map[string][]models.TracfinValue{}}
			if len(stack) > 0 {
				fr.parent = stack[len(stack)-1]
			}
			nodes = append(nodes, fr)
			for _, attr := range t.Attr {
				id := path + "/@" + attr.Name.Local
				af, ok := c.Fields[id]
				if !ok || !af.Attribute || attr.Name.Space != "" {
					addValidation(&report, validationDiagnostic("UNKNOWN_XML_ATTRIBUTE", "internal_structure", "Attribute is not part of this profile", models.TracfinFieldRef{FieldID: id}))
					continue
				}
				fr.counts[id]++
				text := attr.Value
				fr.fields[id] = append(fr.fields[id], models.TracfinValue{Kind: "text", Text: &text})
				validateScalar(&report, af, text, models.TracfinFieldRef{FieldID: id})
			}
			stack = append(stack, fr)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			} else if strings.TrimSpace(string(t)) != "" {
				addValidation(&report, validationDiagnostic("XML_SYNTAX_ERROR", "xml_syntax", "Text outside document element", models.TracfinFieldRef{}))
			}
		case xml.EndElement:
			fr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if f, ok := c.Fields[fr.path]; ok && f.Kind == "field" {
				text := fr.text.String()
				val := models.TracfinValue{Kind: "text", Text: &text}
				lexicalField := f
				if lexicalField.ValueKind == "reference" {
					lexicalField.ValueKind = "text"
				}
				validateValue(&report, models.TracfinBlock{Fields: fr.fields}, lexicalField, val, c)
				if fr.children > 0 {
					addValidation(&report, validationDiagnostic("XML_SCALAR_HAS_CHILDREN", "internal_structure", "Scalar element contains child elements", models.TracfinFieldRef{FieldID: fr.path}))
				}
				if len(stack) > 0 {
					parent := stack[len(stack)-1]
					parent.fields[fr.path] = append(parent.fields[fr.path], val)
				}
			} else if strings.TrimSpace(fr.text.String()) != "" {
				addValidation(&report, validationDiagnostic("XML_BLOCK_HAS_TEXT", "internal_structure", "Structural element contains text", models.TracfinFieldRef{FieldID: fr.path}))
			}
			for _, id := range c.OrderedFields {
				f := c.Fields[id]
				if f.Parent != fr.path || f.Coverage == "excluded" {
					continue
				}
				min, _ := strconv.Atoi(f.Min)
				max, err := strconv.Atoi(f.Max)
				count := fr.counts[id]
				if count < min {
					addValidation(&report, validationDiagnostic("MISSING_REQUIRED_VALUE", "internal_structure", "Required XML element or attribute is absent", models.TracfinFieldRef{FieldID: id}))
				}
				if err == nil && count > max {
					addValidation(&report, validationDiagnostic("CARDINALITY_EXCEEDED", "internal_structure", "Too many XML occurrences", models.TracfinFieldRef{FieldID: id}))
				}
			}
		}
	}
	if roots != 1 {
		addValidation(&report, validationDiagnostic("XML_SYNTAX_ERROR", "xml_syntax", "Expected exactly one document element", models.TracfinFieldRef{}))
	}
	textField := func(fr *frame, id string) string {
		values := fr.fields[id]
		if len(values) > 0 {
			text, _ := valueText(values[0])
			return text
		}
		return ""
	}
	people := map[string]bool{}
	supports := map[string]map[string]bool{}
	for _, node := range nodes {
		if node.path == "/ds/personnes/pp" || node.path == "/ds/personnes/pm" || node.path == "/ds/personnes/pnc" {
			id := textField(node, node.path+"/id")
			if id != "" {
				if people[id] {
					addValidation(&report, validationDiagnostic("DUPLICATE_PERSON_ID", "references", "Person ID occurs more than once", models.TracfinFieldRef{FieldID: node.path + "/id"}))
				}
				people[id] = true
			}
		}
	}
	for _, node := range nodes {
		if !strings.Contains(node.path, "/referencesFinancieres/referenceFinanciere") || c.Fields[node.path].Kind != "block" {
			continue
		}
		owner := node.parent
		for owner != nil && owner.path != "/ds/personnes/pp" && owner.path != "/ds/personnes/pm" && owner.path != "/ds/personnes/pnc" {
			owner = owner.parent
		}
		if owner == nil {
			continue
		}
		person := textField(owner, owner.path+"/id")
		id := textField(node, node.path+"/@id")
		if supports[person] == nil {
			supports[person] = map[string]bool{}
		}
		if id != "" {
			if supports[person][id] {
				addValidation(&report, validationDiagnostic("DUPLICATE_SUPPORT_ID", "references", "Support ID occurs more than once for this person", models.TracfinFieldRef{FieldID: node.path + "/@id"}))
			}
			supports[person][id] = true
		}
	}
	for _, node := range nodes {
		f, ok := c.Fields[node.path]
		if !ok || f.ValueKind != "reference" {
			continue
		}
		text := node.text.String()
		valid := people[text]
		if f.Name == "refFinanciere" {
			person := textField(node.parent, node.parent.path+"/identEmetteur")
			valid = people[person] && supports[person][text]
		}
		if !valid {
			addValidation(&report, validationDiagnostic("INVALID_XML_REFERENCE", "references", "Declared reference is absent or belongs to another person", models.TracfinFieldRef{FieldID: node.path}))
		}
	}
	return report
}
