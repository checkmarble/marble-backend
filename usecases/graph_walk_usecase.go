package usecases

import (
	"context"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

const (
	graphMaxCrossEntityHops      = 1
	graphMaxNodes                = 5000
	graphHyperconnectedThreshold = 70
	graphExactCountLimit         = 100
)

// TODO: should be retrieved from database.
// matchRules are the value-match edges. These cannot be derived from the schema
// (nothing says "IBAN/IP is an identity worth pivoting on"), so they stay
// hardcoded for the prototype. A match pivot is always a bridge: it links
// otherwise-unrelated records. The link rules are computed from the data model.
var matchRules = []models.EdgeRule{
	{LeftType: "account", LeftField: "iban", RightType: "account", RightField: "iban", Kind: "match", Label: "same_iban", CrossEntity: true},
	{LeftType: "devices", LeftField: "ip_address", RightType: "devices", RightField: "ip_address", Kind: "match", Label: "same_ip", CrossEntity: true},
}

type GraphWalkUsecase struct {
	executorFactory     executor_factory.ExecutorFactory
	dataModelRepository repositories.DataModelRepository
	graphRepository     repositories.GraphRepository
}

// WalkGraph does a bounded breadth-first walk over the org's client-schema
// `_graph` table from the given start node, returning the reached subgraph as a
// flat set of nodes (each once) and edges (which may converge on a shared node,
// so a shared IP linking two users shows up as edges into the same device).
func (uc GraphWalkUsecase) WalkGraph(
	ctx context.Context, organizationId uuid.UUID, startType, startId string,
) (models.GraphResult, error) {
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, uc.executorFactory.NewExecutor(), organizationId, false, true)
	if err != nil {
		return models.GraphResult{}, err
	}

	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return models.GraphResult{}, err
	}

	start := models.GraphNode{Type: startType, Id: startId}
	w := &graphWalker{
		ctx:             ctx,
		exec:            exec,
		repo:            uc.graphRepository,
		rules:           append(linkRules(dataModel), matchRules...),
		maxHops:         graphMaxCrossEntityHops,
		maxNodes:        graphMaxNodes,
		hyperThreshold:  graphHyperconnectedThreshold,
		exactCountLimit: graphExactCountLimit,
		visited:         map[models.GraphNode]bool{start: true},
		edgeSeen:        map[edgeKey]bool{},
		hyper:           map[models.GraphNode][]models.HyperconnectedRelation{},
		pivots:          map[models.GraphNode]bool{},
		nodes:           []models.GraphNode{start},
		queue:           []frame{{node: start, hops: 0}},
	}
	return w.run(start)
}

// frame is a queued node together with the number of bridge hops taken to reach
// it, so the walker can stop crossing entities once maxHops is reached.
type frame struct {
	node models.GraphNode
	hops int
}

// linkRules derives one edge rule per data-model link: a child FK field value
// equals the parent's object_id identity value. Whether the link is a bridge
// comes from the data model itself (LinkType): "related" links reference a
// different entity and cost a hop; "belongs_to" links aggregate one entity's own
// records and are free.
func linkRules(dataModel models.DataModel) []models.EdgeRule {
	links := dataModel.AllLinksAsMap()
	rules := make([]models.EdgeRule, 0, len(links))

	for _, link := range links {
		rules = append(rules, models.EdgeRule{
			LeftType:    link.ChildTableName,
			LeftField:   link.ChildFieldName,
			RightType:   link.ParentTableName,
			RightField:  link.ParentFieldName,
			Kind:        "link",
			Label:       link.Name,
			CrossEntity: link.LinkType != models.LinkTypeBelongsTo,
		})
	}
	return rules
}

// edgeKey deduplicates edges undirected-ly: the node pair is stored in a
// canonical order so A->B and B->A (same label) collapse to one key. GraphNode
// is comparable, so the key needs no string building.
type edgeKey struct {
	lo, hi models.GraphNode
	label  string
}

func newEdgeKey(a, b models.GraphNode, label string) edgeKey {
	if nodeLess(b, a) {
		a, b = b, a
	}
	return edgeKey{lo: a, hi: b, label: label}
}

// nodeLess is a total order on nodes, used only to canonicalize edge keys.
func nodeLess(a, b models.GraphNode) bool {
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	return a.Id < b.Id
}

// graphWalker carries the mutable state of a single BFS so the traversal reads
// as a short driver (run) plus named steps (expand / visit / recordEdge).
type graphWalker struct {
	ctx             context.Context
	exec            repositories.Executor
	repo            repositories.GraphRepository
	rules           []models.EdgeRule
	maxHops         int
	maxNodes        int
	hyperThreshold  int
	exactCountLimit int

	visited  map[models.GraphNode]bool
	edgeSeen map[edgeKey]bool
	hyper    map[models.GraphNode][]models.HyperconnectedRelation
	pivots   map[models.GraphNode]bool
	nodes    []models.GraphNode
	edges    []models.GraphEdge
	queue    []frame
}

// run drains the queue, expanding one node at a time, until the reachable
// subgraph (within the hop and node bounds) is exhausted, then assembles the
// result (joining each discovered node with any hyperconnected relations found
// on it).
func (w *graphWalker) run(start models.GraphNode) (models.GraphResult, error) {
	for len(w.queue) > 0 {
		cur := w.queue[0]
		w.queue = w.queue[1:]
		if err := w.expand(cur); err != nil {
			return models.GraphResult{}, err
		}
	}

	nodes := make([]models.GraphResultNode, len(w.nodes))
	for i, n := range w.nodes {
		nodes[i] = models.GraphResultNode{GraphNode: n, Hyperconnected: w.hyper[n], Pivot: w.pivots[n]}
	}
	return models.GraphResult{Start: start, Nodes: nodes, Edges: w.edges}, nil
}

// expand looks at every `_graph` row of the current node and, for each edge rule
// that applies, follows it to the matching neighbors — recording edges and
// discovering new nodes.
func (w *graphWalker) expand(cur frame) error {
	rows, err := w.repo.GetNodeRows(w.ctx, w.exec, cur.node.Type, cur.node.Id)
	if err != nil {
		return err
	}

	for _, row := range rows {
		for _, rule := range w.rules {
			nType, nField, ok := rule.OtherEndpoint(row.RecordType, row.FieldName)
			if !ok {
				continue
			}

			// Fetch one past the exact-count limit: this both reveals a
			// hyperconnected relationship (by overflowing the threshold) and lets us
			// report an exact count as long as the fan-out stays within the still-
			// cheap exact range, without pulling the full fan-out.
			neighbors, err := w.repo.FindMatchingRows(w.ctx, w.exec, nType, nField, row.FieldValue, w.exactCountLimit+1)
			if err != nil {
				return err
			}

			// A hyperconnected relationship is never crossed (its many neighbors
			// are irrelevant); we record it on the node with a connection count
			// instead. This is checked before the hop limit so a supernode reached
			// at the hop boundary is still flagged as hyperconnected.
			if len(neighbors) > w.hyperThreshold {
				count := len(neighbors)
				exact := true

				// Beyond the exact range: fall back to the cheap coarse estimate.
				if count > w.exactCountLimit {
					exact = false
					count, err = w.repo.EstimateMatchingRows(w.ctx, w.exec, nType, nField, row.FieldValue)
					if err != nil {
						return err
					}
				}

				w.markHyperconnected(cur.node, rule, row.FieldName, count, exact)
				continue
			}

			if rule.CrossEntity && cur.hops >= w.maxHops {
				continue
			}

			// A match relationship connects a whole shared-value group; it is
			// materialized as a star through a synthetic pivot node for the value
			// (see recordMatch), which is type-agnostic and avoids the O(n^2)
			// clique a pairwise expansion would produce. Link relationships are
			// genuine pairwise references, emitted directly.
			if rule.Kind == "match" {
				w.recordMatch(cur, rule, row, neighbors)
				continue
			}

			for _, nb := range neighbors {
				neighbor := models.GraphNode{Type: nb.RecordType, Id: nb.RecordId}
				if neighbor == cur.node {
					continue
				}

				nextHops := cur.hops
				if rule.CrossEntity {
					nextHops++
				}
				if !w.visit(neighbor, nextHops) {
					continue // over the node cap; skip the dangling edge too
				}

				w.recordEdge(cur.node, neighbor, rule, row)
			}
		}
	}

	return nil
}

// visit reports whether an edge to node may be recorded. An already-seen node is
// usable as-is; a new node is recorded and enqueued (at the given hop count); a
// new node past maxNodes is rejected so no dangling edge is kept.
func (w *graphWalker) visit(node models.GraphNode, hops int) bool {
	if w.visited[node] {
		return true
	}
	if len(w.nodes) >= w.maxNodes {
		return false
	}
	w.visited[node] = true
	w.nodes = append(w.nodes, node)
	w.queue = append(w.queue, frame{node: node, hops: hops})
	return true
}

// recordEdge appends an edge from `from` to `to`, deduplicated undirected-ly by
// (node pair, label).
func (w *graphWalker) recordEdge(from, to models.GraphNode, rule models.EdgeRule, row models.GraphRow) {
	key := newEdgeKey(from, to, rule.Label)
	if w.edgeSeen[key] {
		return
	}
	w.edgeSeen[key] = true
	w.edges = append(w.edges, models.GraphEdge{
		From:  from,
		To:    to,
		Kind:  rule.Kind,
		Label: rule.Label,
		Field: row.FieldName,
		Value: row.FieldValue,
	})
}

// markHyperconnected records that `node` was not expanded through `rule` because
// the relationship exceeds the hyperconnected threshold, keeping a count of the
// omitted connections (exact when within the exact-count range). A given
// (node, label) is recorded once.
func (w *graphWalker) markHyperconnected(node models.GraphNode, rule models.EdgeRule, field string, count int, exact bool) {
	for _, existing := range w.hyper[node] {
		if existing.Label == rule.Label {
			return
		}
	}
	w.hyper[node] = append(w.hyper[node], models.HyperconnectedRelation{
		Label:       rule.Label,
		Kind:        rule.Kind,
		Field:       field,
		ApproxCount: count,
		Exact:       exact,
	})
}

// recordMatch materializes a shared-value group as a star through a synthetic
// pivot node (one per rule+value): the current node and every matching member
// are linked to the pivot rather than to each other. This is type-agnostic — a
// match rule may connect different entity types (e.g. a device and an account
// sharing an IP) — and turns an O(n^2) clique into n edges. Members are also
// discovered/enqueued so the walk continues through them; each member is linked
// here (not only when it is itself expanded) so members that won't be expanded
// — e.g. reached at the hop limit — are still attached to the pivot.
func (w *graphWalker) recordMatch(cur frame, rule models.EdgeRule, row models.GraphRow, neighbors []models.GraphRow) {
	// A match group with only cur in it is a value nobody else shares — no pivot.
	hasOther := false
	for _, nb := range neighbors {
		if nb.RecordType != cur.node.Type || nb.RecordId != cur.node.Id {
			hasOther = true
			break
		}
	}
	if !hasOther {
		return
	}

	pivot := pivotNode(rule, row.FieldValue)
	if !w.addPivot(pivot) {
		return // over the node cap; don't record dangling edges
	}
	w.recordEdge(cur.node, pivot, rule, row)

	for _, nb := range neighbors {
		member := models.GraphNode{Type: nb.RecordType, Id: nb.RecordId}
		if member == cur.node {
			continue
		}
		if !w.visit(member, cur.hops+1) { // match is always a bridge
			continue // over the node cap; skip the dangling edge too
		}
		w.recordEdge(member, pivot, rule, nb)
	}
}

// pivotNode is the synthetic node standing for a shared value of a match rule.
// Its type is the rule label and its id is the value, so members of the same
// group (whatever their entity type) all resolve to the same pivot.
func pivotNode(rule models.EdgeRule, value string) models.GraphNode {
	return models.GraphNode{Type: rule.Label, Id: value}
}

// addPivot records a pivot node (once), respecting the node cap. Pivots are not
// enqueued: they are synthetic and have no `_graph` rows to expand — the walk
// reaches other members directly, not through the pivot.
func (w *graphWalker) addPivot(pivot models.GraphNode) bool {
	if w.visited[pivot] {
		return true
	}
	if len(w.nodes) >= w.maxNodes {
		return false
	}
	w.visited[pivot] = true
	w.pivots[pivot] = true
	w.nodes = append(w.nodes, pivot)
	return true
}
