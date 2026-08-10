package models

import (
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
	From  GraphNode
	To    GraphNode
	Kind  string
	Label string
	Field string
	Value string
}

// HyperconnectedRelation records a relationship a node was not expanded through, because a
// single value of the relationship's field is carried by more records than the walk's
// cardinality cap. The tax administration's IBAN appears on millions of unrelated
// transactions and says nothing about who is related to whom, so such a relationship is
// reported but never walked. Count is the planner's estimate of how many records carry
// the value.
type HyperconnectedRelation struct {
	Label string
	Kind  string // "link" or "match"
	Field string // the field the relationship pivots on
	Count int
}

// GraphResultNode is a node in the result, enriched with the relationships that were pruned
// while walking it. Connector marks a synthetic node standing for a shared value rather than
// a real record: its Type is the configuration's label and its Id the shared value.
// ConnectorKind is "link" or "match" on such nodes.
type GraphResultNode struct {
	GraphNode
	Hyperconnected []HyperconnectedRelation
	Connector      bool
	ConnectorKind  string
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
	EndTypes []string
	Degrees  int
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
