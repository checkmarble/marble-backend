package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type GraphEdgeNode struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

type GraphEdge struct {
	From    GraphEdgeNode `json:"from"`
	To      GraphEdgeNode `json:"to"`
	Kind    string        `json:"kind"`
	Through []string      `json:"through,omitempty"`
	Field   string        `json:"field"`
	Value   string        `json:"value"`
}

type GraphNode struct {
	Type string `json:"type"`
	Id   string `json:"id"`
	// Connector marks a synthetic node rather than a record: either a value records share
	// ("match") or an un-expanded set of children through one link ("link").
	Connector     bool   `json:"connector,omitempty"`
	ConnectorKind string `json:"connector_kind,omitempty"`
	// HypernodeCount is set only on a connector the walk did not expand through, because what
	// it stands for is shared too widely: it is the approximate number of records concerned,
	// and its presence means this node's edges are a sample rather than the whole set.
	HypernodeCount int `json:"hypernode_count,omitempty"`

	Metadata GraphNodeMetadata `json:"metadata,omitzero"`
}

type GraphNodeMetadata struct {
	Label     string      `json:"label,omitempty"`
	RiskLevel int         `json:"risk_level,omitempty"`
	Tags      []uuid.UUID `json:"tags,omitempty"`
}

type Graph struct {
	Start GraphEdgeNode `json:"start"`
	Nodes []GraphNode   `json:"nodes"`
	Edges []GraphEdge   `json:"edges"`
}

func adaptGraphNode(n models.GraphNode) GraphEdgeNode {
	return GraphEdgeNode{Type: n.Type, Id: n.Id}
}

func adaptGraphResultNode(n models.GraphResultNode) GraphNode {
	node := GraphNode{
		Type:           n.Type,
		Id:             n.Id,
		Connector:      n.Connector,
		ConnectorKind:  n.ConnectorKind,
		HypernodeCount: n.HypernodeCount,
		Metadata: GraphNodeMetadata{
			Label:     n.Metadata.Label,
			RiskLevel: n.Metadata.RiskLevel,
			Tags:      n.Metadata.Tags,
		},
	}

	return node
}

func adaptGraphEdge(e models.GraphEdge) GraphEdge {
	return GraphEdge{
		From:    adaptGraphNode(e.From),
		To:      adaptGraphNode(e.To),
		Kind:    e.Kind,
		Through: e.Through,
		Field:   e.Field,
		Value:   e.Value,
	}
}

func AdaptGraphResultDto(r models.GraphResult) Graph {
	return Graph{
		Start: adaptGraphNode(r.Start),
		Nodes: pure_utils.Map(r.Nodes, adaptGraphResultNode),
		Edges: pure_utils.Map(r.Edges, adaptGraphEdge),
	}
}
