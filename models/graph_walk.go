package models

// GraphRow is a single row of the client-schema `_graph` adjacency table.
type GraphRow struct {
	RecordType string
	RecordId   string
	FieldName  string
	FieldValue string
}

// GraphNode identifies a node by record type and id. It is comparable, so it
// can be used directly as a map key / set element.
type GraphNode struct {
	Type string
	Id   string
}

// GraphEdge is a directed edge between two nodes. Kind is "link" (a relationship
// derived from the data model) or "match" (records sharing a value on an opt-in
// field, e.g. same IBAN or same IP).
type GraphEdge struct {
	From  GraphNode
	To    GraphNode
	Kind  string
	Label string
	Field string
	Value string
}

// HyperconnectedRelation records a relationship (edge rule) that a node was not
// expanded through because it has more than the hyperconnected threshold of
// connections. ApproxCount is the number of omitted connections; it is exact
// when Exact is true (the count stayed within the cheap exact-count range) and a
// coarse estimate otherwise.
type HyperconnectedRelation struct {
	Label       string
	Kind        string // "link" or "match"
	Field       string // the node field the relationship pivots on
	ApproxCount int
	Exact       bool
}

// GraphResultNode is a node in the result, enriched with any hyperconnected
// relationships that were pruned during the walk. Nodes with no such
// relationships carry an empty Hyperconnected slice. Pivot marks a synthetic
// node standing for a shared value of a match relationship (its Type is the
// match label and its Id the shared value) rather than a real record.
type GraphResultNode struct {
	GraphNode
	Hyperconnected []HyperconnectedRelation
	Pivot          bool
}

// GraphResult is the subgraph reached from a starting node, as a flat set of
// nodes and edges (each node appears once; edges may converge on a shared node).
type GraphResult struct {
	Start GraphNode
	Nodes []GraphResultNode
	Edges []GraphEdge
}

// EdgeRule connects two `_graph` endpoints (type, field) whose equal values form
// an edge. Kind is "link" (a relationship derived from the data model, matched
// against a target's object_id identity) or "match" (a user-designated field
// whose shared value links otherwise-unrelated records).
//
// Rules are currently built in code (see the graph walk usecase), but are meant
// to eventually be loaded from the database — hence living here as a model.
type EdgeRule struct {
	LeftType, LeftField   string
	RightType, RightField string
	Kind, Label           string
	// CrossEntity marks edges that cross between entities and so count toward the
	// walk's degrees-of-separation limit. Match pivots are always bridges: they
	// link otherwise-unrelated records. A link is a bridge when it is "related"
	// (a reference to a different entity) rather than "belongs_to" (aggregation
	// of one entity's own records).
	CrossEntity bool
}

// OtherEndpoint returns the endpoint opposite to (recordType, fieldName) if the
// rule applies to that endpoint, so a walk can hop to the matching node.
func (r EdgeRule) OtherEndpoint(recordType, fieldName string) (string, string, bool) {
	switch {
	case r.LeftType == recordType && r.LeftField == fieldName:
		return r.RightType, r.RightField, true
	case r.RightType == recordType && r.RightField == fieldName:
		return r.LeftType, r.LeftField, true
	default:
		return "", "", false
	}
}
