package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// GraphRow is a single row of the client-schema `_graph` adjacency table, for one
// already-known record type. The table holds one row per (record, participating field,
// value) and is maintained out of band by a dedicated worker.
type GraphRow struct {
	RecordId   string
	FieldName  string
	FieldValue string
}

// GraphMatch is a record found by looking up a field value, paired with the requested value
// it came back for so the caller can group results without re-reading them.
type GraphMatch struct {
	Value    string
	RecordId string
}

// GraphNode identifies a node by record type and id. It is comparable, so it can be used
// directly as a map key / set element.
type GraphNode struct {
	Type string
	Id   string
}

// GraphEdge is an undirected edge between two nodes, oriented arbitrarily. Kind is "link" (a
// relationship derived from the data model) or "match" (records sharing a value on a
// configured field, e.g. same IBAN or same IP).
type GraphEdge struct {
	From GraphNode
	To   GraphNode
	Kind string

	// Label names the relationship: the chain of link names for a path contracted through
	// records that are not reported, the link's own name for a single hop, the relation's label
	// for a match. It is what tells two distinct relationships between the same pair of nodes
	// apart, and is what the result's edge deduplication keys on — which is why it stays even
	// though Through is what the caller is given.
	Label string

	// Through is the record types the relationship goes *between* its two ends, reading from From
	// to To. Neither end is in it: both are already named by the edge, and a connector end names
	// no record type at all. So it is empty on a single hop, and holds the records collapsed away
	// on a contracted path.
	//
	// On an edge reaching a match connector — which is always oriented record to connector — it
	// is the route from that record down to the one actually carrying the shared value, and Field
	// is the field carrying it. Empty means the record carries the value itself:
	//
	//	Through: []                    Field: "beneficiary_iban"  -> its own field
	//	Through: ["accounts"]          Field: "iban"              -> its account's field
	//	Through: ["sessions", "logins"] Field: "ip"               -> a login of one of its sessions
	Through []string

	Field string
	Value string
}

// GraphResultNode is a node in the result. Most are records, reported as they were found. Some
// are synthetic — see Connector — standing for something the graph needs to show but that is not
// itself a record.
type GraphResultNode struct {
	GraphNode

	// Connector marks a synthetic node rather than a record. There are two kinds, told apart by
	// ConnectorKind, and both are identified the same way: Type names the relationship and Id
	// the value it pivots on.
	//
	//   - "match": a value two or more records share on a field an organization declared as
	//     meaningful. Type is the relation's label, Id the shared value. Its edges are the
	//     records carrying it, so a value shared by n records costs n edges rather than n².
	//
	//   - "link": the records hanging off one record through a single data-model link, when
	//     there are too many of them to pull in. Type is the link's name, Id the value the
	//     children point at. Such a node exists only when the walk gave up on expanding it, so
	//     it always carries a HypernodeCount.
	Connector     bool
	ConnectorKind string // "link" or "match"

	// HypernodeCount is non-zero on a connector the walk did not expand through, because the
	// value it stands for is carried by more records than the cardinality cap allows — the tax
	// administration's IBAN appears on millions of unrelated transactions and says nothing
	// about who is related to whom. It is the approximate number of records concerned, and its
	// presence means this node's edges are a sample of what is out there rather than all of it.
	HypernodeCount int

	Metadata GraphResultNodeMetadata
}

type GraphResultNodeMetadata struct {
	Index int
	// Label is the record's caption: the value it carries on the field its table declares as
	// its caption field. It is empty on a connector, which is not a record, and on a record
	// whose table declares no caption field.
	Label     string
	RiskLevel int
	Tags      []uuid.UUID
}

// GraphResult is the subgraph reached from a starting node, as a flat set of nodes and edges
// (each node appears once; edges may converge on a shared node). Only end nodes — party
// records by default — and connector nodes are reported: the intermediate records the walk
// went through are collapsed away, so two parties related through a chain of accounts and
// transactions show up as directly connected.
type GraphResult struct {
	Start GraphNode
	Nodes []GraphResultNode
	Edges []GraphEdge
}

// GraphWalkOptions carries the caller-tunable parameters of a walk: the end-node types the
// result should be collapsed to (empty means default to the data model's party tables) and
// the number of degrees to explore from the start (zero means the default).
type GraphWalkOptions struct {
	EndTypes               []string
	Degrees                int
	SkipSameFieldRelations bool
	SameFieldRelations     []string
}

// GraphRelation declares that equal values of two (record type, field) endpoints connect the
// records carrying them, even though no link exists between those records. An organization
// defines its own relations against the tables and fields of its own data model.
//
// Relations are one-to-one: a group of three endpoints that should all count as sharing a
// value is expressed as three relations (A<->B, B<->C, C<->A). Relations sharing a Label
// converge on the same connector node, so such a group still renders as a single star.
type GraphRelation struct {
	Id         uuid.UUID
	OrgId      uuid.UUID
	Label      string
	LeftType   string
	LeftField  string
	RightType  string
	RightField string
	CreatedAt  time.Time
}

// OtherEndpoint returns the endpoint opposite to (recordType, fieldName) when the relation
// applies to that endpoint. A self-relation (both endpoints equal) returns that same endpoint,
// so callers must filter the origin record out of the results.
func (r GraphRelation) OtherEndpoint(recordType, fieldName string) (string, string, bool) {
	switch {
	case r.LeftType == recordType && r.LeftField == fieldName:
		return r.RightType, r.RightField, true
	case r.RightType == recordType && r.RightField == fieldName:
		return r.LeftType, r.LeftField, true
	default:
		return "", "", false
	}
}

// Endpoints returns the relation's two endpoints, deduplicated for a self-relation.
func (r GraphRelation) Endpoints() [][2]string {
	if r.LeftType == r.RightType && r.LeftField == r.RightField {
		return [][2]string{{r.LeftType, r.LeftField}}
	}
	return [][2]string{{r.LeftType, r.LeftField}, {r.RightType, r.RightField}}
}

// AppliesTo says whether both of the relation's endpoints still resolve to a field of a table in
// the data model. A relation is validated when it is created, so a false here means the two have
// drifted apart since — a field archived or renamed, a table dropped.
func (r GraphRelation) AppliesTo(dataModel DataModel) bool {
	for _, endpoint := range r.Endpoints() {
		if !GraphFieldExists(dataModel, endpoint[0], endpoint[1]) {
			return false
		}
	}
	return true
}

// GraphFieldExists says whether a (record type, field) pair names a field of a table in the data
// model.
func GraphFieldExists(dataModel DataModel, recordType, fieldName string) bool {
	table, ok := dataModel.Tables[recordType]
	if !ok {
		return false
	}
	_, ok = table.Fields[fieldName]
	return ok
}

// GraphTraversableFields returns, per record type, every field a walk can read on it: both ends of
// every link, and both endpoints of every relation that still matches the data model. A relation
// that no longer matches is skipped, since the walk cannot follow it either.
func GraphTraversableFields(dataModel DataModel, relations []GraphRelation) map[string][]string {
	fields := map[string][]string{}

	add := func(recordType, fieldName string) {
		if !slices.Contains(fields[recordType], fieldName) {
			fields[recordType] = append(fields[recordType], fieldName)
		}
	}

	for _, link := range dataModel.AllLinksAsMap() {
		add(link.ParentTableName, link.ParentFieldName)
		add(link.ChildTableName, link.ChildFieldName)
	}

	for _, relation := range relations {
		if !relation.AppliesTo(dataModel) {
			continue
		}
		for _, endpoint := range relation.Endpoints() {
			add(endpoint[0], endpoint[1])
		}
	}

	// The data model is held in maps, so without sorting the order fields are visited — and so
	// the order of the rows and of the resulting graph — would vary between two runs.
	for recordType := range fields {
		slices.Sort(fields[recordType])
	}

	return fields
}

// GraphIndexedFields returns, per record type, every field the adjacency table must carry: what a
// walk can read, plus object_id on every table whether or not anything links to it. Fields come
// back resolved against the data model, since how a value is rendered as text depends on its type.
//
// This is deliberately a superset of GraphTraversableFields and must stay one. A field a walk
// reads but the adjacency table does not carry does not fail — it silently finds nothing — which
// is why both are derived here rather than listed in two places.
func GraphIndexedFields(dataModel DataModel, relations []GraphRelation) map[string][]Field {
	names := GraphTraversableFields(dataModel, relations)

	indexed := make(map[string][]Field, len(dataModel.Tables))

	for recordType, table := range dataModel.Tables {
		wanted := names[recordType]

		if !slices.Contains(wanted, "object_id") {
			wanted = append(slices.Clone(wanted), "object_id")
		}

		slices.Sort(wanted)

		fields := make([]Field, 0, len(wanted))

		for _, name := range wanted {
			field, ok := table.Fields[name]
			if ok {
				fields = append(fields, field)
			}
		}

		indexed[recordType] = fields
	}

	return indexed
}

// CreateGraphRelation is the caller-supplied part of a relation: the rest is assigned by the
// database.
type CreateGraphRelation struct {
	OrgId      uuid.UUID
	Label      string
	LeftType   string
	LeftField  string
	RightType  string
	RightField string
}
