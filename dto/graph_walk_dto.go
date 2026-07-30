package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type GraphNode struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

type GraphEdge struct {
	From  GraphNode `json:"from"`
	To    GraphNode `json:"to"`
	Kind  string    `json:"kind"`
	Label string    `json:"label"`
	Field string    `json:"field"`
	Value string    `json:"value"`
}

type HyperconnectedRelation struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Field string `json:"field"`
	Count int    `json:"count"`
}

type GraphResultNode struct {
	Type           string                   `json:"type"`
	Id             string                   `json:"id"`
	Connector      bool                     `json:"connector,omitempty"`
	ConnectorKind  string                   `json:"connector_kind,omitempty"`
	Hyperconnected []HyperconnectedRelation `json:"hyperconnected,omitempty"`
}

type GraphResult struct {
	Start GraphNode         `json:"start"`
	Nodes []GraphResultNode `json:"nodes"`
	Edges []GraphEdge       `json:"edges"`
}

func adaptGraphNode(n models.GraphNode) GraphNode {
	return GraphNode{Type: n.Type, Id: n.Id}
}

func adaptHyperconnectedRelation(r models.HyperconnectedRelation) HyperconnectedRelation {
	return HyperconnectedRelation{
		Label: r.Label,
		Kind:  r.Kind,
		Field: r.Field,
		Count: r.Count,
	}
}

func adaptGraphResultNode(n models.GraphResultNode) GraphResultNode {
	return GraphResultNode{
		Type:           n.Type,
		Id:             n.Id,
		Connector:      n.Connector,
		ConnectorKind:  n.ConnectorKind,
		Hyperconnected: pure_utils.Map(n.Hyperconnected, adaptHyperconnectedRelation),
	}
}

func adaptGraphEdge(e models.GraphEdge) GraphEdge {
	return GraphEdge{
		From:  adaptGraphNode(e.From),
		To:    adaptGraphNode(e.To),
		Kind:  e.Kind,
		Label: e.Label,
		Field: e.Field,
		Value: e.Value,
	}
}

func AdaptGraphResultDto(r models.GraphResult) GraphResult {
	return GraphResult{
		Start: adaptGraphNode(r.Start),
		Nodes: pure_utils.Map(r.Nodes, adaptGraphResultNode),
		Edges: pure_utils.Map(r.Edges, adaptGraphEdge),
	}
}
