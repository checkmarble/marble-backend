package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/assert"
)

// fakeGraphRepository is an in-memory `_graph` table: a flat list of rows that
// the query methods filter over, so a walk can be driven without a database. The
// exec argument is ignored.
type fakeGraphRepository struct {
	rows []models.GraphRow
	// estimateOverride, when non-zero, is what EstimateMatchingRows returns
	// instead of the exact count — so a test can tell whether the walk used the
	// exact fetched count or fell back to the (approximate) estimate.
	estimateOverride int
}

func (r fakeGraphRepository) GetNodeRows(
	_ context.Context, _ repositories.Executor, recordType, recordId string,
) ([]models.GraphRow, error) {
	var out []models.GraphRow
	for _, row := range r.rows {
		if row.RecordType == recordType && row.RecordId == recordId {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r fakeGraphRepository) FindMatchingRows(
	_ context.Context, _ repositories.Executor, recordType, fieldName, fieldValue string, limit int,
) ([]models.GraphRow, error) {
	var out []models.GraphRow
	for _, row := range r.rows {
		if row.RecordType == recordType && row.FieldName == fieldName && row.FieldValue == fieldValue {
			out = append(out, row)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// EstimateMatchingRows returns estimateOverride if set, else the exact match
// count — a fine stand-in for the planner estimate the real repository uses.
func (r fakeGraphRepository) EstimateMatchingRows(
	_ context.Context, _ repositories.Executor, recordType, fieldName, fieldValue string,
) (int, error) {
	if r.estimateOverride != 0 {
		return r.estimateOverride, nil
	}
	count := 0
	for _, row := range r.rows {
		if row.RecordType == recordType && row.FieldName == fieldName && row.FieldValue == fieldValue {
			count++
		}
	}
	return count, nil
}

// walkConfig holds the tunable bounds a test injects, so behavior can be
// exercised without huge fixtures. Zero fields fall back to the real defaults.
type walkConfig struct {
	maxHops         int
	maxNodes        int
	hyperThreshold  int
	exactCountLimit int
}

// runWalk drives a graphWalker over the given repo/rules from start, mirroring
// how WalkGraph wires one up.
func runWalk(t *testing.T, repo fakeGraphRepository, rules []models.EdgeRule, start models.GraphNode, cfg walkConfig) models.GraphResult {
	t.Helper()
	if cfg.maxHops == 0 {
		cfg.maxHops = graphMaxCrossEntityHops
	}
	if cfg.maxNodes == 0 {
		cfg.maxNodes = graphMaxNodes
	}
	if cfg.hyperThreshold == 0 {
		cfg.hyperThreshold = graphHyperconnectedThreshold
	}
	if cfg.exactCountLimit == 0 {
		cfg.exactCountLimit = graphExactCountLimit
	}
	w := &graphWalker{
		ctx:             context.Background(),
		repo:            repo,
		rules:           rules,
		maxHops:         cfg.maxHops,
		maxNodes:        cfg.maxNodes,
		hyperThreshold:  cfg.hyperThreshold,
		exactCountLimit: cfg.exactCountLimit,
		visited:         map[models.GraphNode]bool{start: true},
		edgeSeen:        map[edgeKey]bool{},
		hyper:           map[models.GraphNode][]models.HyperconnectedRelation{},
		pivots:          map[models.GraphNode]bool{},
		nodes:           []models.GraphNode{start},
		queue:           []frame{{node: start, hops: 0}},
	}
	result, err := w.run(start)
	assert.NoError(t, err)
	return result
}

// nodeSet collapses the result nodes into a set for order-independent asserts.
func nodeSet(result models.GraphResult) map[models.GraphNode]bool {
	set := make(map[models.GraphNode]bool, len(result.Nodes))
	for _, n := range result.Nodes {
		set[n.GraphNode] = true
	}
	return set
}

// hasEdge reports whether the result has an (undirected) edge between a and b
// with the given label.
func hasEdge(result models.GraphResult, a, b models.GraphNode, label string) bool {
	for _, e := range result.Edges {
		if e.Label != label {
			continue
		}
		if (e.From == a && e.To == b) || (e.From == b && e.To == a) {
			return true
		}
	}
	return false
}

// hyperOf returns the hyperconnected relations recorded on a node in the result.
func hyperOf(result models.GraphResult, node models.GraphNode) []models.HyperconnectedRelation {
	for _, n := range result.Nodes {
		if n.GraphNode == node {
			return n.Hyperconnected
		}
	}
	return nil
}

// isPivot reports whether the given node is present in the result as a pivot.
func isPivot(result models.GraphResult, node models.GraphNode) bool {
	for _, n := range result.Nodes {
		if n.GraphNode == node {
			return n.Pivot
		}
	}
	return false
}

func TestGraphWalk_LinkChain(t *testing.T) {
	// transactions -belongs_to-> account -belongs_to-> user. Aggregation links
	// are free (not bridges), so the whole chain is reached even at maxHops=1.
	rules := []models.EdgeRule{
		{LeftType: "transactions", LeftField: "account_id", RightType: "account", RightField: "object_id", Kind: "link", Label: "tx_account"},
		{LeftType: "account", LeftField: "user_id", RightType: "user", RightField: "object_id", Kind: "link", Label: "acct_user"},
	}
	rows := []models.GraphRow{
		{RecordType: "transactions", RecordId: "T1", FieldName: "account_id", FieldValue: "A1"},
		{RecordType: "account", RecordId: "A1", FieldName: "object_id", FieldValue: "A1"},
		{RecordType: "account", RecordId: "A1", FieldName: "user_id", FieldValue: "U1"},
		{RecordType: "user", RecordId: "U1", FieldName: "object_id", FieldValue: "U1"},
	}
	start := models.GraphNode{Type: "transactions", Id: "T1"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, start, walkConfig{maxHops: 1})

	nodes := nodeSet(result)
	assert.Len(t, result.Nodes, 3)
	assert.True(t, nodes[models.GraphNode{Type: "account", Id: "A1"}])
	assert.True(t, nodes[models.GraphNode{Type: "user", Id: "U1"}])
	assert.Len(t, result.Edges, 2)
	assert.True(t, hasEdge(result, start, models.GraphNode{Type: "account", Id: "A1"}, "tx_account"))
	assert.True(t, hasEdge(result,
		models.GraphNode{Type: "account", Id: "A1"},
		models.GraphNode{Type: "user", Id: "U1"}, "acct_user"))
}

func TestGraphWalk_MatchPivot(t *testing.T) {
	// Three accounts share an IBAN: they attach to a single synthetic pivot node
	// for the value (a star), never directly to each other (no clique).
	rules := []models.EdgeRule{
		{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "account", RecordId: "A1", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A2", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A3", FieldName: "iban", FieldValue: "IB"},
	}
	a1 := models.GraphNode{Type: "account", Id: "A1"}
	a2 := models.GraphNode{Type: "account", Id: "A2"}
	a3 := models.GraphNode{Type: "account", Id: "A3"}
	pivot := models.GraphNode{Type: "same_iban", Id: "IB"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, a1, walkConfig{maxHops: 1})

	assert.Len(t, result.Nodes, 4) // 3 accounts + 1 pivot
	assert.True(t, isPivot(result, pivot))
	assert.Len(t, result.Edges, 3)
	assert.True(t, hasEdge(result, a1, pivot, "same_iban"))
	assert.True(t, hasEdge(result, a2, pivot, "same_iban"))
	assert.True(t, hasEdge(result, a3, pivot, "same_iban"))
	assert.False(t, hasEdge(result, a1, a2, "same_iban"), "members attach to the pivot, not to each other")
	assert.False(t, hasEdge(result, a2, a3, "same_iban"))
}

func TestGraphWalk_MatchNoPivotWhenUnshared(t *testing.T) {
	// An account whose IBAN no other record shares: the value isn't a
	// connection, so no pivot node (and no edge) is produced.
	rules := []models.EdgeRule{
		{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "account", RecordId: "A1", FieldName: "iban", FieldValue: "IB"},
	}
	start := models.GraphNode{Type: "account", Id: "A1"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, start, walkConfig{maxHops: 1})

	assert.Len(t, result.Nodes, 1, "unshared value must not create a pivot")
	assert.Empty(t, result.Edges)
}

func TestGraphWalk_MatchPivotCrossType(t *testing.T) {
	// A cross-type match: devices and accounts sharing an IP. Both entity types
	// attach to the same pivot — one star of M+N edges, not an M×N bipartite mesh.
	rules := []models.EdgeRule{
		{LeftType: "devices", LeftField: "ip", RightType: "account", RightField: "signup_ip", Kind: "match", Label: "same_ip", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "devices", RecordId: "D1", FieldName: "ip", FieldValue: "IP"},
		{RecordType: "devices", RecordId: "D2", FieldName: "ip", FieldValue: "IP"},
		{RecordType: "account", RecordId: "A1", FieldName: "signup_ip", FieldValue: "IP"},
		{RecordType: "account", RecordId: "A2", FieldName: "signup_ip", FieldValue: "IP"},
	}
	d1 := models.GraphNode{Type: "devices", Id: "D1"}
	members := []models.GraphNode{
		d1,
		{Type: "devices", Id: "D2"},
		{Type: "account", Id: "A1"},
		{Type: "account", Id: "A2"},
	}
	pivot := models.GraphNode{Type: "same_ip", Id: "IP"}

	// maxHops=2 so the walk crosses back to the far-side device via the accounts.
	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, d1, walkConfig{maxHops: 2})

	assert.Len(t, result.Nodes, 5) // 2 devices + 2 accounts + 1 pivot
	assert.True(t, isPivot(result, pivot))
	assert.Len(t, result.Edges, 4) // one per member -> pivot, no M×N mesh
	for _, m := range members {
		assert.True(t, hasEdge(result, m, pivot, "same_ip"), "%s should attach to the pivot", m.Id)
	}
	assert.False(t, hasEdge(result, members[0], members[2], "same_ip"), "no direct device-account edge")
}

func TestGraphWalk_BridgeHopLimit(t *testing.T) {
	// U1 -related-> U2 -related-> U3. "related" links are bridges, so at
	// maxHops=1 the walk reaches U2 but stops before U3.
	rules := []models.EdgeRule{
		{LeftType: "user", LeftField: "peer_id", RightType: "user", RightField: "object_id", Kind: "link", Label: "peer", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "user", RecordId: "U1", FieldName: "peer_id", FieldValue: "U2"},
		{RecordType: "user", RecordId: "U1", FieldName: "object_id", FieldValue: "U1"},
		{RecordType: "user", RecordId: "U2", FieldName: "peer_id", FieldValue: "U3"},
		{RecordType: "user", RecordId: "U2", FieldName: "object_id", FieldValue: "U2"},
		{RecordType: "user", RecordId: "U3", FieldName: "object_id", FieldValue: "U3"},
	}
	u1 := models.GraphNode{Type: "user", Id: "U1"}
	u2 := models.GraphNode{Type: "user", Id: "U2"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, u1, walkConfig{maxHops: 1})

	nodes := nodeSet(result)
	assert.Len(t, result.Nodes, 2)
	assert.True(t, nodes[u2])
	assert.False(t, nodes[models.GraphNode{Type: "user", Id: "U3"}], "U3 is 2 bridge hops away and must not be reached")
	assert.Len(t, result.Edges, 1)
	assert.True(t, hasEdge(result, u1, u2, "peer"))
}

func TestGraphWalk_NodeCap(t *testing.T) {
	// A star of accounts (below the hyperconnected threshold) emitted from the
	// smallest id, walked with a tiny maxNodes: the result is truncated to
	// exactly maxNodes nodes and never records an edge to a node it dropped.
	rules := []models.EdgeRule{
		{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "account", RecordId: "A1", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A2", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A3", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A4", FieldName: "iban", FieldValue: "IB"},
		{RecordType: "account", RecordId: "A5", FieldName: "iban", FieldValue: "IB"},
	}
	start := models.GraphNode{Type: "account", Id: "A1"}

	const maxNodes = 3
	// hyperThreshold above the 5-member group so it is crossed, not pruned.
	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, start, walkConfig{maxHops: 1, maxNodes: maxNodes, hyperThreshold: 100})

	assert.Len(t, result.Nodes, maxNodes, "node count is capped at maxNodes")
	// The start plus one edge per other retained node; no edges to dropped nodes.
	assert.Len(t, result.Edges, maxNodes-1)
	retained := nodeSet(result)
	for _, e := range result.Edges {
		assert.True(t, retained[e.From] && retained[e.To], "edges must only reference retained nodes")
	}
}

func TestGraphWalk_HyperconnectedRelationPruned(t *testing.T) {
	// Five accounts share an IBAN, with a threshold of 3: the relationship is
	// hyperconnected, so none of the peers are pulled in and the start node is
	// annotated with an approximate count instead.
	rules := []models.EdgeRule{
		{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	}
	rows := make([]models.GraphRow, 0, 5)
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5"} {
		rows = append(rows, models.GraphRow{RecordType: "account", RecordId: id, FieldName: "iban", FieldValue: "IB"})
	}
	start := models.GraphNode{Type: "account", Id: "A1"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, start, walkConfig{maxHops: 1, hyperThreshold: 3})

	assert.Len(t, result.Nodes, 1, "hyperconnected peers must not be pulled in")
	assert.Empty(t, result.Edges)

	hyper := hyperOf(result, start)
	assert.Len(t, hyper, 1)
	assert.Equal(t, "same_iban", hyper[0].Label)
	assert.Equal(t, "match", hyper[0].Kind)
	assert.Equal(t, "iban", hyper[0].Field)
	assert.Equal(t, 5, hyper[0].ApproxCount)
}

func TestGraphWalk_PerRelationshipPruning(t *testing.T) {
	// U1 belongs to org O1 (a single cheap link, traversed) and shares an IP with
	// six users (hyperconnected at threshold 3, pruned). Only the cheap link's
	// neighbor is pulled in; the IP relationship is recorded on U1.
	rules := []models.EdgeRule{
		{LeftType: "user", LeftField: "org_id", RightType: "organization", RightField: "object_id", Kind: "link", Label: "belongs_org"},
		{LeftType: "user", LeftField: "ip", RightType: "user", RightField: "ip", Kind: "match", Label: "same_ip", CrossEntity: true},
	}
	rows := []models.GraphRow{
		{RecordType: "user", RecordId: "U1", FieldName: "org_id", FieldValue: "O1"},
		{RecordType: "user", RecordId: "U1", FieldName: "ip", FieldValue: "IP"},
		{RecordType: "organization", RecordId: "O1", FieldName: "object_id", FieldValue: "O1"},
	}
	for _, id := range []string{"U2", "U3", "U4", "U5", "U6"} {
		rows = append(rows, models.GraphRow{RecordType: "user", RecordId: id, FieldName: "ip", FieldValue: "IP"})
	}
	u1 := models.GraphNode{Type: "user", Id: "U1"}
	o1 := models.GraphNode{Type: "organization", Id: "O1"}

	result := runWalk(t, fakeGraphRepository{rows: rows}, rules, u1, walkConfig{maxHops: 1, hyperThreshold: 3})

	nodes := nodeSet(result)
	assert.Len(t, result.Nodes, 2, "only the cheap link's neighbor is pulled in")
	assert.True(t, nodes[o1])
	assert.False(t, nodes[models.GraphNode{Type: "user", Id: "U2"}], "hyperconnected IP peers must not be pulled in")
	assert.True(t, hasEdge(result, u1, o1, "belongs_org"))

	hyper := hyperOf(result, u1)
	assert.Len(t, hyper, 1)
	assert.Equal(t, "same_ip", hyper[0].Label)
	assert.Equal(t, 6, hyper[0].ApproxCount)
	assert.Empty(t, hyperOf(result, o1), "the cheap neighbor is not hyperconnected")
}

// ibanGroup builds n accounts sharing one IBAN, plus a same_iban match rule.
func ibanGroup(n int) ([]models.GraphRow, []models.EdgeRule) {
	rows := make([]models.GraphRow, 0, n)
	for i := 1; i <= n; i++ {
		rows = append(rows, models.GraphRow{
			RecordType: "account", RecordId: fmt.Sprintf("A%03d", i), FieldName: "iban", FieldValue: "IB",
		})
	}
	rules := []models.EdgeRule{
		{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	}
	return rows, rules
}

func TestGraphWalk_ExactCountWithinRange(t *testing.T) {
	// 4 connections, hyperconnected above 2 but within the exact-count range of
	// 10: the count is the exact fetched value, not the (overridden) estimate.
	rows, rules := ibanGroup(4)
	repo := fakeGraphRepository{rows: rows, estimateOverride: 9999}
	start := models.GraphNode{Type: "account", Id: "A001"}

	result := runWalk(t, repo, rules, start, walkConfig{maxHops: 1, hyperThreshold: 2, exactCountLimit: 10})

	hyper := hyperOf(result, start)
	assert.Len(t, hyper, 1)
	assert.Equal(t, 4, hyper[0].ApproxCount, "within the exact range the count is precise, not estimated")
	assert.True(t, hyper[0].Exact)
}

func TestGraphWalk_EstimateBeyondRange(t *testing.T) {
	// 6 connections, past the exact-count range of 3: the count comes from the
	// (overridden) estimate rather than the capped fetch.
	rows, rules := ibanGroup(6)
	repo := fakeGraphRepository{rows: rows, estimateOverride: 9999}
	start := models.GraphNode{Type: "account", Id: "A001"}

	result := runWalk(t, repo, rules, start, walkConfig{maxHops: 1, hyperThreshold: 2, exactCountLimit: 3})

	hyper := hyperOf(result, start)
	assert.Len(t, hyper, 1)
	assert.Equal(t, 9999, hyper[0].ApproxCount, "beyond the exact range the count falls back to the estimate")
	assert.False(t, hyper[0].Exact)
}
