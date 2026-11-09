// Package format is the shared description language of declaration formats.
// Each format (TRACFIN, goAML LU, goAML CH) is described once in Go: tree,
// cardinalities, simple types, facets, code lists with labels and Go rules
// for conditional constraints and XSD 1.1 asserts. No XSD is read at runtime;
// the official XSD files are only used as test oracles.
package format

import (
	"regexp"
	"sync"
)

const Unbounded = -1

type Base string

const (
	BaseString   Base = "string"
	BaseInt      Base = "int"
	BaseDecimal  Base = "decimal"
	BaseBoolean  Base = "boolean"
	BaseDate     Base = "date"     // TRACFIN dd/mm/yyyy is expressed as a pattern on string
	BaseDateTime Base = "dateTime" // xs:dateTime lexical form
)

// SimpleType holds XSD-like facets. Negative integer facets mean "absent".
type SimpleType struct {
	Name           string `json:"name"`
	Base           Base   `json:"base"`
	MinLength      int    `json:"min_length"`
	MaxLength      int    `json:"max_length"`
	Pattern        string `json:"pattern,omitempty"` // RE2, matched against the whole value
	MinInclusive   string `json:"min_inclusive,omitempty"`
	MaxInclusive   string `json:"max_inclusive,omitempty"`
	TotalDigits    int    `json:"total_digits"`
	FractionDigits int    `json:"fraction_digits"`
	CodeList       string `json:"code_list,omitempty"`
}

type Element struct {
	Name      string `json:"name"`
	Label     string `json:"label,omitempty"`
	Type      string `json:"type"` // simple or complex type name
	Min       int    `json:"min"`
	Max       int    `json:"max"` // Unbounded or 0 when the profile forbids the element
	Attribute bool   `json:"attribute,omitempty"`
	Choice    string `json:"choice,omitempty"` // choice group ID inside the owner type
	Fixed     string `json:"fixed,omitempty"`
	Doc       string `json:"doc,omitempty"`
}

// Choice lists the leading element of each alternative. An alternative is
// selected when its leading element is present.
type Choice struct {
	ID    string   `json:"id"`
	Min   int      `json:"min"`
	Leads []string `json:"leads"`
}

type ComplexType struct {
	Name     string    `json:"name"`
	Label    string    `json:"label,omitempty"`
	Text     string    `json:"text,omitempty"` // simple content type for elements with attributes
	Elements []Element `json:"elements"`
	Choices  []Choice  `json:"choices,omitempty"`
}

type Code struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CodeList struct {
	Name   string `json:"name"`
	Label  string `json:"label,omitempty"`
	Source string `json:"source,omitempty"`
	Codes  []Code `json:"codes"`
}

type Schema struct {
	Key         string                  `json:"key"`
	Label       string                  `json:"label"`
	RootElement string                  `json:"root_element"`
	RootType    string                  `json:"root_type"`
	Namespace   string                  `json:"namespace,omitempty"`
	Complex     map[string]*ComplexType `json:"complex"`
	Simple      map[string]*SimpleType  `json:"simple"`
	CodeLists   map[string]*CodeList    `json:"code_lists"`
	Rules       []Rule                  `json:"-"`

	once     sync.Once
	patterns map[string]*regexp.Regexp
	codes    map[string]map[string]string
}

func (s *Schema) init() {
	s.once.Do(func() {
		s.patterns = map[string]*regexp.Regexp{}
		for name, t := range s.Simple {
			if t.Pattern != "" {
				s.patterns[name] = regexp.MustCompile("^(?:" + t.Pattern + ")$")
			}
		}
		s.codes = map[string]map[string]string{}
		for name, l := range s.CodeLists {
			m := make(map[string]string, len(l.Codes))
			for _, c := range l.Codes {
				m[c.Value] = c.Label
			}
			s.codes[name] = m
		}
	})
}

// HasCode reports whether value belongs to the named code list.
func (s *Schema) HasCode(list, value string) bool {
	s.init()
	_, ok := s.codes[list][value]
	return ok
}

// CodeLabel returns the label of a code, or "" when unknown.
func (s *Schema) CodeLabel(list, value string) string {
	s.init()
	return s.codes[list][value]
}

// Lookup returns the element description of child in the complex type.
func (s *Schema) Lookup(typeName, child string) (Element, bool) {
	t := s.Complex[typeName]
	if t == nil {
		return Element{}, false
	}
	for _, e := range t.Elements {
		if e.Name == child {
			return e, true
		}
	}
	return Element{}, false
}

// TypeAt resolves the complex type name at an element path such as
// "report/transaction/t_from_my_client" (indexes are ignored).
func (s *Schema) TypeAt(path ...string) (string, bool) {
	current := s.RootType
	for _, name := range path {
		e, ok := s.Lookup(current, name)
		if !ok {
			return "", false
		}
		current = e.Type
	}
	return current, true
}
