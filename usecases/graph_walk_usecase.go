package usecases

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

// ================================================================================================
// How a graph walk works
// ================================================================================================
//
// A walk starts at one record (the "start node") and explores everything reachable from it
// through two different kinds of relationship:
//
//   - a LINK: a foreign key declared in the data model. A transaction's account_id names the
//     account it belongs to. Links are directional: "downward" from the parent (one account)
//     to its children (many transactions, potentially — this is where a walk fans out), and
//     "upward" from a child back to its single parent (this direction never fans out, since a
//     well-formed data model gives a child exactly one parent per link).
//
//   - a MATCH, from a "graph relation": two (table, field) pairs an organization has declared
//     as meaning the same thing when their values are equal, even though no link connects the
//     two tables — two accounts that happen to carry the same IBAN, say. Unlike a link, a match
//     has no natural direction and no natural cap on how many records can share a value.
//
// Most of the records a walk visits are not interesting on their own — nobody asks "show me
// this transaction", they ask "how is this party connected to that one" — so the walk works in
// two passes:
//
//  1. DISCOVERY (run and everything it calls: followDownward, followUpwardClosure,
//     followSameField, matchSameField) explores outward from the start node, one "degree" at a
//     time, and records every record and every relationship it finds — including all the
//     "boring" intermediate ones — into an adjacency list (graphWalker.adj). Nothing is
//     filtered out yet; the point of this pass is completeness.
//
//  2. CONTRACTION (result and everything it calls: contractFrom, contractedEdge,
//     attachConnector, ownersOf) then collapses that raw graph down to only the node types the
//     caller actually asked for (the "kept" or "end" types — party records by default), plus
//     the start node and any connector node (below). Every other record is walked through and
//     dropped, which is what makes two parties related through a chain of accounts and
//     transactions come back as directly connected.
//
// Worked example. A party owns an account, which sent a transaction to another account owned
// by a different party. Discovery finds every node on that chain, links and all:
//
//	Party A --owns--> Account A1 --sent--> Transaction T --received_by--> Account B1 <--owns-- Party B
//
// Contraction walks the chain looking for the next KEPT node (here, a party) and reports a
// single edge for the whole path, naming both the links it walked and the record types it
// collapsed away on the way (see contractedEdge):
//
//	Party A ==== "owns > sent > received_by > owns", through [account transaction account] ====> Party B
//
// A MATCH relationship never links two records directly. Instead, a synthetic "connector" node
// — identified by the relation's group id and the shared value, e.g. (a1b2..., "FR76...") — is
// wired to every record that carries the value, so that a value shared by n records costs n edges
// instead of the n² a fully-connected star would need. The connector itself is always part of the
// result: it is the visible "why" behind an otherwise unexplained edge between two end nodes.
// Identifying it by group id rather than by label is what lets the several relations spelling out
// one conceptual group converge on a single connector, and what keeps two groups an organization
// happened to label the same from collapsing into one. The label is carried alongside, for display
// only (see GraphResultNode.Label).
//
//	   Account A1
//	        \
//	         (connector: same_iban, "FR76...")
//	        /          \
//	   Account A2       Account A3
//
// Bounding the walk. Nothing here is free to spread without limit: every direction the walk can
// grow in has its own fan-out cap, the walk stops after a bounded number of degrees, and the
// total number of records pulled in is capped regardless of shape (see registerNode). The
// constants just below give the specific numbers and the reasoning behind each one.
//
// Something carried by more records than its cap allows is a "hypernode": it is reported, so the
// caller knows it is there, but never expanded through. Both kinds report it the same way — as a
// connector node carrying a HypernodeCount, which says "there are roughly this many of these, and
// the edges you can see are a sample rather than all of them":
//
//   - a MATCH already had a connector standing for the shared value, so it simply gains a count
//     (matchSameField).
//   - a LINK gets one synthesised for it, identified by the link's name and the value the
//     children point at (connectLinkHypernode).
//
// So every synthetic node in a result is a connector, and ConnectorKind says which of the two it
// is. Nothing is reported as a property hanging off the records that led there.

const (
	graphDefaultDegrees = 2
	graphMaxDegrees     = 5

	// graphMaxSameFieldFanout is the hypernode threshold. A value carried by more than this
	// many records says nothing about who is related to whom: the tax administration's IBAN
	// appears on millions of unrelated transactions. Such a value is reported but never
	// walked through.
	graphMaxSameFieldFanout = 50

	// graphMaxLinkFanout is the equivalent cap for a downward link. Links legitimately fan
	// out much wider than a shared attribute — a merchant owning many transactions is
	// normal, not suspicious — so they get their own, higher threshold.
	graphMaxLinkFanout = 500

	// graphMaxUpwardFanout guards an upward link. It resolves to a single parent in a
	// well-formed model, so this only bites when a parent field turns out not to be unique.
	graphMaxUpwardFanout = 10

	// graphMaxUpwardDepth bounds the upward closure in case the data model's links form a
	// cycle.
	graphMaxUpwardDepth = 10

	// graphMaxNodes bounds the total number of records one walk pulls in.
	graphMaxNodes = 5000

	// graphMaxEstimates bounds how many planner estimates one walk pays for. Sizing a pruned
	// relationship costs one round trip per distinct over-cardinality value, which is normally
	// a handful — such values are by definition few and widely shared. On a dataset where it
	// is not, the remainder report the threshold they crossed instead, which is a true lower
	// bound, rather than turning the walk back into one query per value.
	graphMaxEstimates = 50
)

// graphEdgeKindLink and graphEdgeKindMatch are the two kinds of edge a result can contain: a
// link edge comes from a data-model foreign key, a match edge from a graph relation. The same
// two strings label a connector node's ConnectorKind, saying which of the two the synthetic
// node stands for.
const (
	graphEdgeKindLink  = "link"
	graphEdgeKindMatch = "match"
)

type GraphWalkUsecase struct {
	enforceSecurity         security.EnforceSecurity
	executorFactory         executor_factory.ExecutorFactory
	featureAccessReader     OrganizationUsecaseFeatureAccessReader
	dataModelRepository     repositories.DataModelRepository
	graphRepository         repositories.GraphRepository
	graphRelationRepository repositories.GraphRelationRepository
}

// WalkGraph returns the graph of end nodes — "party" records unless the caller asks for other
// types — reachable from the start node. Intermediate records such as a party's accounts and
// transactions are walked through but not reported: two parties related through a chain of
// them show up as directly connected. Records related by a shared attribute instead of a
// link are joined through a synthetic connector node naming the value they share.
//
// The walk proceeds in degrees. Each degree follows every downward (one-to-many) link one
// level from its input, takes the full upward (many-to-one) closure of what it holds, matches
// shared attributes over all of that, and takes the upward closure of the records those
// matches reached. Everything newly found feeds the next degree.
func (uc GraphWalkUsecase) WalkGraph(
	ctx context.Context,
	startType, startId string,
	opts models.GraphWalkOptions,
) (models.GraphResult, error) {
	orgId := uc.enforceSecurity.OrgId()

	fa, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return models.GraphResult{}, err
	}

	if !fa.GraphExploration.IsAllowed() {
		return models.GraphResult{}, errors.Wrap(models.ForbiddenError,
			"organization not allowed to use the graph exploration feature")
	}
	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return models.GraphResult{}, err
	}

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, uc.executorFactory.NewExecutor(), orgId, false, true)
	if err != nil {
		return models.GraphResult{}, err
	}

	if _, ok := dataModel.Tables[startType]; !ok {
		return models.GraphResult{}, errors.Wrapf(models.BadParameterError,
			"unknown start node type %q", startType)
	}

	endTypes, err := resolveGraphEndTypes(dataModel, opts.EndTypes)
	if err != nil {
		return models.GraphResult{}, err
	}

	// An organization declares its own shared-attribute relations against its own data model. An
	// organization that has declared none still gets a walk: it just follows links only.
	relations, err := uc.graphRelationRepository.ListGraphRelations(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.GraphResult{}, err
	}

	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return models.GraphResult{}, err
	}

	degrees := opts.Degrees
	if degrees <= 0 {
		degrees = graphDefaultDegrees
	}
	degrees = min(degrees, graphMaxDegrees)

	w := &graphWalker{
		ctx:                    ctx,
		exec:                   exec,
		repo:                   uc.graphRepository,
		schema:                 buildGraphSchema(ctx, dataModel, endTypes, relations),
		skipSameFieldRelations: opts.SkipSameFieldRelations,
		sameFieldRelations:     opts.SameFieldRelations,
		maxNodes:               graphMaxNodes,
		maxLinkFanout:          graphMaxLinkFanout,
		maxUpwardFanout:        graphMaxUpwardFanout,
		maxSameFieldFanout:     graphMaxSameFieldFanout,
		maxUpwardDepth:         graphMaxUpwardDepth,
		maxEstimates:           graphMaxEstimates,
	}

	graph, err := w.run(models.GraphNode{Type: startType, Id: startId}, degrees)
	if err != nil {
		return models.GraphResult{}, err
	}

	if err := uc.enrichGraph(ctx, orgId, exec, dataModel, graph); err != nil {
		return models.GraphResult{}, err
	}

	return graph, nil
}

// enrichGraph fills in what the reported nodes carry beyond their identity. Each pass writes its
// own fields of a node's metadata rather than the whole struct, so they compose in any order.
func (uc GraphWalkUsecase) enrichGraph(
	ctx context.Context,
	orgId uuid.UUID,
	clientExec repositories.Executor,
	dataModel models.DataModel,
	graph models.GraphResult,
) error {
	if len(graph.Nodes) == 0 {
		return nil
	}

	if err := uc.addNodeScoring(ctx, orgId, graph); err != nil {
		return err
	}

	return uc.addNodeCaptions(ctx, clientExec, dataModel, graph)
}

// addNodeScoring attaches the risk level and tags held in the Marble database against each
// reported record.
func (uc GraphWalkUsecase) addNodeScoring(ctx context.Context, orgId uuid.UUID, graph models.GraphResult) error {
	// Only records have metadata to look up. A connector's Type is a relation group's id or a
	// link's name, neither of which is a record type, so asking about them is work that cannot
	// match: skip them, and keep where each surviving record sat so the answers can be put back.
	scoringRecords := make([]models.ScoringRecordRef, 0, len(graph.Nodes))
	nodeIndexes := make([]int, 0, len(graph.Nodes))

	for idx, node := range graph.Nodes {
		if node.Connector {
			continue
		}

		scoringRecords = append(scoringRecords, models.ScoringRecordRef{
			RecordType: node.GraphNode.Type,
			RecordId:   node.GraphNode.Id,
		})
		nodeIndexes = append(nodeIndexes, idx)
	}

	if len(scoringRecords) == 0 {
		return nil
	}

	scores, err := uc.graphRepository.GetNodeBatchMetadata(ctx, uc.executorFactory.NewExecutor(), orgId, scoringRecords)
	if err != nil {
		return err
	}

	// The query returns its rows tagged with the position of the record they were asked for, so
	// the rows are zipped back onto the nodes through nodeIndexes, which is where each of those
	// records sat: both must keep matching the order scoringRecords was built in above.
	for _, score := range scores {
		// Index is the one-based ordinality of the record within scoringRecords, not within the
		// result's nodes — a connector has no slot there. A position outside the range asked for
		// describes no node here, and indexing on it would take the whole walk down.
		if score.Index < 1 || score.Index > len(nodeIndexes) {
			continue
		}

		// Only the scored fields are assigned: the walk has already put what it knows in the same
		// struct, and replacing it wholesale would drop that.
		metadata := &graph.Nodes[nodeIndexes[score.Index-1]].Metadata

		metadata.RiskLevel = score.RiskLevel
		metadata.Tags = score.Tags
	}

	return nil
}

// addNodeCaptions labels each reported record with its caption: the field its table declares as
// its caption, by carrying the "caption" semantic sub-type. A table declaring no such field has
// nothing a record of it can be called, so those nodes are reported without a label rather than
// with an invented one.
//
// Which field that is comes out of the data model already in hand — the sub-type lives in the
// field's metadata, which GetDataModel reads — so resolving it costs nothing on top of the one
// query that reads the captions themselves.
func (uc GraphWalkUsecase) addNodeCaptions(
	ctx context.Context,
	clientExec repositories.Executor,
	dataModel models.DataModel,
	graph models.GraphResult,
) error {
	captionRecords := make([]models.ScoringRecordRef, len(graph.Nodes))
	captionFields := map[string]string{}

	for idx, node := range graph.Nodes {
		if node.Connector {
			// A connector is not a record: leave its slot in place so the positions still line up
			// with the nodes, but name no type for it, so nothing can be read for it. Its type is
			// a relation group's id or a link's name, which could coincide with a table's name.
			continue
		}

		captionRecords[idx] = models.ScoringRecordRef{
			RecordType: node.GraphNode.Type,
			RecordId:   node.GraphNode.Id,
		}

		captionField, ok := dataModel.Tables[node.Type].
			FieldWithSemanticSubType(models.FieldSemanticSubTypeCaption)
		if ok {
			captionFields[node.Type] = captionField.Name
		}
	}

	captions, err := uc.graphRepository.GetNodeBatchCaptions(ctx, clientExec, captionFields, captionRecords)
	if err != nil {
		return err
	}

	for _, caption := range captions {
		if caption.Index < 1 || caption.Index > len(graph.Nodes) {
			continue
		}

		graph.Nodes[caption.Index-1].Metadata.Label = caption.Label
	}

	return nil
}

// resolveGraphEndTypes turns the requested end types into a set of table names. An empty
// request defaults to the data model's party tables (person/company). A requested type that
// is not a table is a bad-parameter error.
func resolveGraphEndTypes(dataModel models.DataModel, requested []string) (map[string]bool, error) {
	endTypes := make(map[string]bool, len(requested))

	if len(requested) == 0 {
		for name, table := range dataModel.Tables {
			if table.SemanticType.IsParty() {
				endTypes[name] = true
			}
		}
		return endTypes, nil
	}

	for _, name := range requested {
		if _, ok := dataModel.Tables[name]; !ok {
			return nil, errors.Wrapf(models.BadParameterError, "unknown end type %q", name)
		}
		endTypes[name] = true
	}

	return endTypes, nil
}

// ------------------------------------------------------------------------------------------------
// Schema: precomputing what the walk is allowed to read and follow
// ------------------------------------------------------------------------------------------------

// graphSchema is everything the walk needs to know about the data model, arranged so no step
// has to search it.
type graphSchema struct {
	endTypes map[string]bool

	// downward holds the one-to-many direction of every link, keyed by the parent table it
	// is followed from: the children carry the value of their parent's field.
	downward map[string][]models.LinkToSingle
	// upward holds the many-to-one direction, keyed by the child table.
	upward map[string][]models.LinkToSingle

	sameField map[string][]models.GraphRelation

	// neededFields is, per record type, every field the walk can ever read on it. Records
	// are hydrated with exactly this set, once, so later steps read values from memory
	// instead of going back to the database per node.
	neededFields map[string][]string
}

func buildGraphSchema(
	ctx context.Context,
	dataModel models.DataModel,
	endTypes map[string]bool,
	relations []models.GraphRelation,
) graphSchema {
	schema := graphSchema{
		endTypes:     endTypes,
		downward:     map[string][]models.LinkToSingle{},
		upward:       map[string][]models.LinkToSingle{},
		sameField:    map[string][]models.GraphRelation{},
		neededFields: models.GraphTraversableFields(dataModel, relations),
	}

	for _, link := range dataModel.AllLinksAsMap() {
		schema.downward[link.ParentTableName] = append(schema.downward[link.ParentTableName], link)
		schema.upward[link.ChildTableName] = append(schema.upward[link.ChildTableName], link)
	}

	for _, relation := range relations {
		// A relation is validated against the data model when it is created, so an endpoint that
		// no longer resolves means the two have drifted apart since — a field archived or
		// renamed, a table dropped. Skip that relation rather than fail the whole walk, but say
		// so: it is a misconfiguration to fix, not something to expect.
		if !relation.AppliesTo(dataModel) {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "graph walk: relation does not match the data model, skipping it",
				"relation_id", relation.Id,
				"group_id", relation.GroupId,
				"label", relation.Label)
			continue
		}

		for _, endpoint := range relation.Endpoints() {
			recordType := endpoint[0]

			// A relation with both endpoints on the same table — sender and receiver IBAN of a
			// transaction, say — must be registered once, not once per endpoint: matching
			// already walks both of its fields.
			alreadyRegistered := slices.ContainsFunc(schema.sameField[recordType],
				func(existing models.GraphRelation) bool { return existing.Id == relation.Id })

			if !alreadyRegistered {
				schema.sameField[recordType] = append(schema.sameField[recordType], relation)
			}
		}
	}

	// The data model is held in maps, so without sorting the order links are visited — and
	// therefore the order of the resulting nodes and edges — would vary between two walks over
	// identical data.
	for recordType := range schema.downward {
		slices.SortFunc(schema.downward[recordType], graphLinkOrder)
	}
	for recordType := range schema.upward {
		slices.SortFunc(schema.upward[recordType], graphLinkOrder)
	}

	return schema
}

func graphLinkOrder(a, b models.LinkToSingle) int {
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	return strings.Compare(a.Id, b.Id)
}

// ------------------------------------------------------------------------------------------------
// Discovery state: the raw graph as it is found, before contraction collapses it
// ------------------------------------------------------------------------------------------------

// rawEdge is an adjacency entry of the graph as discovered, before intermediates are
// collapsed away. Every edge is inserted in both directions: contraction needs undirected
// reachability.
type rawEdge struct {
	to    models.GraphNode
	kind  string
	label string
	field string
	value string

	// toParent is set on the child-to-parent side of a link. Keeping the direction is what
	// separates "the party this record belongs to" — reachable by going up — from "another
	// record that merely hangs off the same parent", reachable by going up then back down.
	// Conflating the two makes a walk claim a party carries a value one of its neighbours'
	// records carries.
	toParent bool
}

// graphWalker holds everything one call to WalkGraph needs: it is created fresh per call and
// discarded once result() returns, so its maps never need to be cleared between requests.
type graphWalker struct {
	ctx                    context.Context
	exec                   repositories.Executor
	repo                   repositories.GraphRepository
	schema                 graphSchema
	skipSameFieldRelations bool
	sameFieldRelations     []uuid.UUID

	// --- fan-out and size limits for this walk, fixed for its whole lifetime (see the
	// constants near the top of this file for what each one means and why it has its value) ---
	maxNodes           int
	maxLinkFanout      int
	maxUpwardFanout    int
	maxSameFieldFanout int
	maxUpwardDepth     int
	maxEstimates       int

	// --- the raw discovery graph: filled in degree by degree during discovery, then read back
	// (never mutated) during contraction ---

	// values is the field cache: values[node][fieldName] is what hydrate last read for that
	// node. A node with no entry has simply not been hydrated yet.
	values map[models.GraphNode]map[string]string
	// adj is the undirected adjacency list of every edge found, keyed by either endpoint.
	adj map[models.GraphNode][]rawEdge
	// conns marks which nodes are connectors and records their kind (link/match). A connector's
	// own Type and Id are what identify it — its relation's group id and the shared value for a
	// match, its link's name and the parent value for a link — so its kind, plus the label
	// below, is all there is left to remember about one here.
	conns map[models.GraphNode]string

	// connLabels is what to call each connector: the group's label for a match connector, whose
	// Type is a group id and so unfit to show, and the link's name for a link one. result()
	// reports it as GraphResultNode.Label, next to the identity rather than in place of it.
	// Relations sharing a group id are guaranteed the same label by the creation path, so which
	// one first reaches connect() and sets this is not meant to matter.
	connLabels map[models.GraphNode]string

	// hyperConns are the connectors emitted for an over-cardinality value, mapped to the
	// estimated number of records carrying it. They have no members and so would be pruned as
	// unshared, but saying "this value exists and is shared far too widely to enumerate" is
	// the whole point of reporting them — which is also why the count lives here, on the one
	// node standing for the value, rather than being repeated on every record carrying it.
	// Membership, not the value, is what marks a connector as a hypernode: use comma-ok.
	hyperConns map[models.GraphNode]int

	// seen / order together are the node set: seen is for membership tests, order is the
	// (deterministic, discovery-order) sequence result() iterates to build its output.
	seen     map[models.GraphNode]bool
	order    []models.GraphNode
	edgeSeen map[graphEdgeKey]bool

	// upwardDone / sameFieldDone keep a node from being expanded twice. A node found in one
	// degree is also part of the next degree's input, where following its downward links a
	// second time is the intended one-level-per-degree spread, but re-running its upward
	// closure or its shared-attribute lookups would only repeat queries.
	upwardDone    map[models.GraphNode]bool
	sameFieldDone map[models.GraphNode]bool

	// --- counters surfaced only in the "graph walk completed" log line, for observability ---
	queries    int
	estimates  int
	pruned     int
	nodeCapHit bool
}

// ------------------------------------------------------------------------------------------------
// Per-degree discovery
// ------------------------------------------------------------------------------------------------

// run drives the whole discovery pass: it seeds the frontier with the start node alone, then
// repeats one "degree" until either the requested number of degrees has run or a degree finds
// nothing new to carry into the next one. One degree is this pipeline:
//
//	frontier                          ─┬─(followDownward)──────▶ children
//	frontier ∪ children               ─┴─(followUpwardClosure)─▶ parents
//	frontier ∪ children ∪ parents      ──(followSameField)─────▶ matched
//	matched                            ──(followUpwardClosure)─▶ matchedParents
//
//	next frontier = children ∪ parents ∪ matched ∪ matchedParents
//
// In words: follow every downward link one level from this degree's input; take the full
// upward closure of everything that gives (the input plus what it just found); match shared
// attributes over all of that; and take the upward closure of whatever the matching reached,
// without re-matching it before the next degree (see the comment on that call below for why).
//
// Note that the frontier this degree started with is deliberately not part of the next one:
// its downward links have already been followed here, and re-adding it would only make
// followDownward issue the exact same lookups again — followUpwardClosure and followSameField
// already guard themselves against repeat work via upwardDone / sameFieldDone, but
// followDownward has no such guard, so excluding the old frontier here is what keeps it honest.
func (w *graphWalker) run(start models.GraphNode, degrees int) (models.GraphResult, error) {
	w.values = map[models.GraphNode]map[string]string{}
	w.adj = map[models.GraphNode][]rawEdge{}
	w.conns = map[models.GraphNode]string{}
	w.connLabels = map[models.GraphNode]string{}
	w.hyperConns = map[models.GraphNode]int{}
	w.seen = map[models.GraphNode]bool{}
	w.edgeSeen = map[graphEdgeKey]bool{}
	w.upwardDone = map[models.GraphNode]bool{}
	w.sameFieldDone = map[models.GraphNode]bool{}

	w.registerNode(start)

	if err := w.hydrate([]models.GraphNode{start}); err != nil {
		return models.GraphResult{}, err
	}

	frontier := []models.GraphNode{start}
	ran := 0

	for degree := 1; degree <= degrees && len(frontier) > 0; degree++ {
		if err := w.ctx.Err(); err != nil {
			return models.GraphResult{}, errors.Wrap(err, "graph walk interrupted")
		}

		ran = degree

		// A downward link fans out, so it is followed a single level per degree, and only
		// from this degree's input.
		children, err := w.followDownward(frontier)
		if err != nil {
			return models.GraphResult{}, err
		}

		// An upward link converges on one parent, so its whole closure is affordable.
		parents, err := w.followUpwardClosure(graphUnion(frontier, children))
		if err != nil {
			return models.GraphResult{}, err
		}

		var (
			matched        []models.GraphNode
			matchedParents []models.GraphNode
		)

		// Shared attributes are matched over everything found so far this degree...
		if !w.skipSameFieldRelations {
			matched, err = w.followSameField(graphUnion(frontier, children, parents))
			if err != nil {
				return models.GraphResult{}, err
			}

			// ...and the records those matches reached get their parents too, but are not
			// themselves re-matched before the next degree.
			matchedParents, err = w.followUpwardClosure(matched)
			if err != nil {
				return models.GraphResult{}, err
			}

		}

		frontier = graphUnion(children, parents, matched, matchedParents)
	}

	result := w.result(start)

	utils.LoggerFromContext(w.ctx).InfoContext(w.ctx, "graph walk completed",
		"start_type", start.Type,
		"degrees_requested", degrees,
		"degrees_ran", ran,
		"records_seen", len(w.seen),
		"queries", w.queries,
		"relations_pruned", w.pruned,
		"node_cap_hit", w.nodeCapHit,
		"nodes_returned", len(result.Nodes),
		"edges_returned", len(result.Edges),
	)

	return result, nil
}

// followDownward follows every one-to-many link out of the frontier, one level (the "downward"
// direction described in the file-level comment above). Children hold the value of their
// parent's field, so this is a lookup of the child table by the child field, batched over
// every frontier record sharing that link. It is never run to closure like
// followUpwardClosure is: a downward link can fan out without bound (one merchant, many
// transactions), so it is capped at maxLinkFanout and only ever spreads one level per degree.
func (w *graphWalker) followDownward(frontier []models.GraphNode) ([]models.GraphNode, error) {
	var discovered []models.GraphNode

	byType := graphGroupByType(frontier)

	for _, recordType := range slices.Sorted(maps.Keys(byType)) {
		for _, link := range w.schema.downward[recordType] {
			found, err := w.followLink(byType[recordType], link,
				link.ParentFieldName, link.ChildTableName, link.ChildFieldName,
				w.maxLinkFanout, false)
			if err != nil {
				return nil, err
			}

			discovered = append(discovered, found...)
		}
	}

	return discovered, w.hydrate(discovered)
}

// followUpwardClosure follows many-to-one links from the seeds and keeps going until nothing
// new turns up, so a record reaches its parent, its parent's parent and so on within a single
// degree:
//
//	seed --(link)--> parent --(link)--> grandparent --(link)--> ...
//
// Running this to convergence is safe in a way followDownward is not: an upward link resolves
// to exactly one record in a well-formed data model (that is what "many-to-one" means), so
// walking all the way up from a handful of seeds only ever visits a handful of ancestors.
// maxUpwardFanout and maxUpwardDepth exist only to bound the damage on a data model where that
// assumption turns out to be false — a parent field that is not actually unique, or a cycle.
func (w *graphWalker) followUpwardClosure(seeds []models.GraphNode) ([]models.GraphNode, error) {
	level := make([]models.GraphNode, 0, len(seeds))

	for _, node := range seeds {
		if w.upwardDone[node] {
			continue
		}

		w.upwardDone[node] = true

		level = append(level, node)
	}

	var discovered []models.GraphNode

	for depth := 0; depth < w.maxUpwardDepth && len(level) > 0; depth++ {
		var next []models.GraphNode

		byType := graphGroupByType(level)

		for _, recordType := range slices.Sorted(maps.Keys(byType)) {
			for _, link := range w.schema.upward[recordType] {
				found, err := w.followLink(byType[recordType], link,
					link.ChildFieldName, link.ParentTableName, link.ParentFieldName,
					w.maxUpwardFanout, true)
				if err != nil {
					return nil, err
				}

				next = append(next, found...)
			}
		}

		next = graphUnion(next)

		for _, node := range next {
			w.upwardDone[node] = true
		}

		if err := w.hydrate(next); err != nil {
			return nil, err
		}

		discovered = append(discovered, next...)
		level = next
	}

	return discovered, nil
}

// followLink resolves one link in one direction for a whole set of records at once: it reads
// originField off each record, looks the collected values up in targetType.targetField, and
// wires an edge for every record it finds. targetIsParent says which way round the two ends are,
// since the same link is followed downward in one step (nodes are parents, target is the child)
// and upward in another (nodes are children, target is the parent). Whichever direction is
// running, the child end is always passed to addLinkEdge as the child — see the two branches
// below — so which end holds the foreign key (needed later by ownersOf, to know which direction
// is "up") is recorded the same way regardless of which direction discovered the edge. It
// reports the records it saw for the first time.
func (w *graphWalker) followLink(
	nodes []models.GraphNode,
	link models.LinkToSingle,
	originField, targetType, targetField string,
	limit int,
	targetIsParent bool,
) ([]models.GraphNode, error) {
	origins := w.originsByValue(nodes, originField)
	if len(origins) == 0 {
		return nil, nil
	}

	res, err := w.lookup(origins, targetType, targetField, limit)
	if err != nil {
		return nil, err
	}

	for _, value := range res.overCap {
		count, err := w.estimate(targetType, targetField, value, limit+1)
		if err != nil {
			return nil, err
		}

		w.connectLinkHypernode(origins[value], link, originField, value, count)
	}

	var discovered []models.GraphNode

	for _, value := range slices.Sorted(maps.Keys(res.members)) {
		for _, member := range res.members[value] {
			admitted, isNew := w.registerNode(member)
			if !admitted {
				continue
			}

			for _, origin := range origins[value] {
				if targetIsParent {
					w.addLinkEdge(origin, member, link.Name, link.ChildFieldName, value)
				} else {
					w.addLinkEdge(member, origin, link.Name, link.ChildFieldName, value)
				}
			}

			if isNew {
				discovered = append(discovered, member)
			}
		}
	}

	return discovered, nil
}

// ------------------------------------------------------------------------------------------------
// Shared-attribute matching
// ------------------------------------------------------------------------------------------------

// followSameField connects records carrying the same value on a field named by a graph
// relation. The records are not linked to each other directly: a synthetic connector node
// named after the shared value joins them instead — see the star diagram in the file-level
// comment above — which both says why they are related and keeps a value shared by n records
// as one star rather than the n² edges a direct connection would need.
func (w *graphWalker) followSameField(nodes []models.GraphNode) ([]models.GraphNode, error) {
	var discovered []models.GraphNode

	pending := make([]models.GraphNode, 0, len(nodes))

	for _, node := range nodes {
		if w.sameFieldDone[node] {
			continue
		}

		w.sameFieldDone[node] = true
		pending = append(pending, node)
	}

	byType := graphGroupByType(pending)

	for _, recordType := range slices.Sorted(maps.Keys(byType)) {
		for _, relation := range w.schema.sameField[recordType] {
			if len(w.sameFieldRelations) > 0 && !slices.Contains(w.sameFieldRelations, relation.GroupId) {
				continue
			}

			for _, endpoint := range relation.Endpoints() {
				if endpoint[0] != recordType {
					continue
				}

				found, err := w.matchSameField(byType[recordType], relation, recordType, endpoint[1])
				if err != nil {
					return nil, err
				}

				discovered = append(discovered, found...)
			}
		}
	}

	return discovered, w.hydrate(discovered)
}

// matchSameField applies one relation to one field of one batch of same-type nodes: it looks
// up whatever the relation's other endpoint carries the same value, creates a connector for
// every value that connects at least two records, and reports a hypernode instead for any
// value whose fan-out blew past maxSameFieldFanout.
func (w *graphWalker) matchSameField(
	nodes []models.GraphNode,
	relation models.GraphRelation,
	recordType, fieldName string,
) ([]models.GraphNode, error) {
	otherType, otherField, ok := relation.OtherEndpoint(recordType, fieldName)
	if !ok {
		return nil, nil
	}

	origins := w.originsByValue(nodes, fieldName)
	if len(origins) == 0 {
		return nil, nil
	}

	res, err := w.lookup(origins, otherType, otherField, w.maxSameFieldFanout)
	if err != nil {
		return nil, err
	}

	// An over-cardinality value is still worth surfacing — the caller wants to know the
	// shared IBAN is there — but it is a dead end: the connector is emitted with no members
	// and is never walked through. The count goes on the connector rather than on each record
	// carrying the value, since the connector is the one node that stands for the value.
	for _, value := range res.overCap {
		count, err := w.estimate(otherType, otherField, value, w.maxSameFieldFanout+1)
		if err != nil {
			return nil, err
		}

		connector := w.connect(relation.GroupId, relation.Label, value, w.participants(origins[value], fieldName, nil, ""))
		w.markHypernode(connector, count)
	}

	var discovered []models.GraphNode

	for _, value := range slices.Sorted(maps.Keys(res.members)) {
		participants := w.participants(origins[value], fieldName, res.members[value], otherField)

		if len(participants) < 2 {
			// Nobody else carries this value, so it does not connect anything.
			continue
		}

		w.connect(relation.GroupId, relation.Label, value, participants)

		for _, p := range participants {
			if p.isNew {
				discovered = append(discovered, p.node)
			}
		}
	}

	return discovered, nil
}

// graphParticipant is a record hanging off a connector node, together with the field it
// carries the shared value on — which differs between the two sides of a cross-type relation.
type graphParticipant struct {
	node  models.GraphNode
	field string
	isNew bool
}

// participants merges the records that led to a shared value with the records found carrying
// it, keeping each node once. A self-relation (both endpoints on the same table and field, e.g.
// "two accounts sharing an IBAN") returns the origins among the matches too, so the
// deduplication here is what stops a record connecting to itself.
func (w *graphWalker) participants(
	origins []models.GraphNode,
	originField string,
	members []models.GraphNode,
	memberField string,
) []graphParticipant {
	participants := make([]graphParticipant, 0, len(origins)+len(members))
	seen := make(map[models.GraphNode]bool, len(origins)+len(members))

	add := func(nodes []models.GraphNode, field string) {
		for _, node := range nodes {
			if seen[node] {
				continue
			}
			seen[node] = true
			participants = append(participants, graphParticipant{node: node, field: field})
		}
	}

	add(origins, originField)
	add(members, memberField)

	return participants
}

// connect wires every participant to the connector node standing for the value they share and
// returns that node.
func (w *graphWalker) connect(groupId uuid.UUID, label, value string, participants []graphParticipant) models.GraphNode {
	// A connector is identified by its relation's group id and the shared value. That is what
	// makes a group spelled out as several one-to-one relations (see the doc comment on
	// models.GraphRelation) converge on a single node regardless of what each relation is
	// labelled, and equally what keeps two groups that happen to share a label apart. The group
	// id is not meant to be displayed, so the label rides along in connLabels.
	connector := models.GraphNode{Type: groupId.String(), Id: value}

	admitted, isNew := w.registerNode(connector)

	if !admitted {
		return connector
	}
	if isNew {
		w.conns[connector] = graphEdgeKindMatch
		w.connLabels[connector] = label
	}

	for i := range participants {
		admitted, isNew := w.registerNode(participants[i].node)
		if !admitted {
			continue
		}

		participants[i].isNew = isNew

		w.addMatchEdge(participants[i].node, connector, label, participants[i].field, value)
	}

	return connector
}

// ------------------------------------------------------------------------------------------------
// Batched database access
// ------------------------------------------------------------------------------------------------

// graphLookupResult is the outcome of one batched value lookup: the records found per value,
// with the values whose fan-out blew past the cap held aside instead.
type graphLookupResult struct {
	members map[string][]models.GraphNode
	overCap []string
}

// lookup finds, for every value the origins carry, the records of recordType whose fieldName
// equals it — in one query for the whole batch. It asks for one row more than the cap so a
// value at the limit can be told from a value beyond it.
func (w *graphWalker) lookup(
	origins map[string][]models.GraphNode,
	recordType, fieldName string,
	limit int,
) (graphLookupResult, error) {
	matches, err := w.repo.FindByValues(w.ctx, w.exec, recordType, fieldName, slices.Sorted(maps.Keys(origins)), limit+1)
	w.queries++

	if err != nil {
		return graphLookupResult{}, err
	}

	idsByValue := map[string][]string{}

	for _, match := range matches {
		idsByValue[match.Value] = append(idsByValue[match.Value], match.RecordId)
	}

	res := graphLookupResult{members: map[string][]models.GraphNode{}}

	for value, ids := range idsByValue {
		if len(ids) > limit {
			res.overCap = append(res.overCap, value)
			continue
		}

		slices.Sort(ids)
		nodes := make([]models.GraphNode, 0, len(ids))

		for _, id := range ids {
			nodes = append(nodes, models.GraphNode{Type: recordType, Id: id})
		}

		res.members[value] = nodes
	}

	slices.Sort(res.overCap)

	return res, nil
}

// estimate sizes an over-cardinality relationship. lowerBound is the number of records the
// capped lookup actually proved, and is reported as-is once the walk's estimate budget is spent.
func (w *graphWalker) estimate(recordType, fieldName, value string, lowerBound int) (int, error) {
	if w.estimates >= w.maxEstimates {
		return lowerBound, nil
	}
	w.estimates++

	count, err := w.repo.EstimateValueCount(w.ctx, w.exec, recordType, fieldName, value)
	w.queries++

	if err != nil {
		return 0, err
	}

	// The planner works off sampled statistics and routinely lands under the truth — it will
	// happily estimate 21 rows for a value 48 records carry. Reporting a number below the
	// threshold that caused the pruning would contradict the pruning itself, so the count the
	// lookup proved wins.
	return max(count, lowerBound), nil
}

// hydrate reads, in one query per record type, every field the walk can need on the given
// records. Values are cached for the whole walk, so a record is read once no matter how many
// steps look at it.
func (w *graphWalker) hydrate(nodes []models.GraphNode) error {
	idsByType := map[string][]string{}
	for _, node := range nodes {
		if _, ok := w.values[node]; ok {
			continue
		}
		if _, ok := w.conns[node]; ok {
			// A connector stands for a value, not a record: there is nothing to read.
			continue
		}

		// Mark it hydrated up front, so a record with no rows in `_graph` is not asked for
		// again on every later step.
		w.values[node] = map[string]string{}
		idsByType[node.Type] = append(idsByType[node.Type], node.Id)
	}

	for _, recordType := range slices.Sorted(maps.Keys(idsByType)) {
		fields := w.schema.neededFields[recordType]

		if len(fields) == 0 {
			// Nothing on this record type is traversable.
			continue
		}

		rows, err := w.repo.FetchFields(w.ctx, w.exec, recordType, idsByType[recordType], fields)
		w.queries++
		if err != nil {
			return err
		}

		for _, row := range rows {
			node := models.GraphNode{Type: recordType, Id: row.RecordId}
			if values, ok := w.values[node]; ok {
				values[row.FieldName] = row.FieldValue
			}
		}
	}

	return nil
}

// originsByValue collects the distinct values the given records carry on a field, each mapped
// back to the records that carry it, so one lookup can serve the whole batch.
func (w *graphWalker) originsByValue(nodes []models.GraphNode, fieldName string) map[string][]models.GraphNode {
	origins := map[string][]models.GraphNode{}

	for _, node := range nodes {
		value := w.values[node][fieldName]
		if value == "" {
			// An empty value means the field was never set on this record (or is absent from
			// `_graph` entirely — hydrate leaves an unhydrated field as the zero value). It
			// cannot be shared with anything, so there is nothing to look up for it.
			continue
		}
		origins[value] = append(origins[value], node)
	}

	return origins
}

// ------------------------------------------------------------------------------------------------
// Bookkeeping: admitting nodes, recording edges, tracking hyperconnections
// ------------------------------------------------------------------------------------------------

// registerNode admits a node into the graph. It reports whether the node is usable at all —
// false once the node budget is spent, so callers drop the edge rather than leave it dangling
// — and whether this is the first time it has been seen, since only those need expanding.
func (w *graphWalker) registerNode(node models.GraphNode) (admitted, isNew bool) {
	if w.seen[node] {
		return true, false
	}
	if len(w.seen) >= w.maxNodes {
		w.nodeCapHit = true
		return false, false
	}

	w.seen[node] = true
	w.order = append(w.order, node)

	return true, true
}

// addLinkEdge records a link, keeping track of which end is the child holding the reference.
func (w *graphWalker) addLinkEdge(child, parent models.GraphNode, label, field, value string) {
	w.addEdge(child, parent, graphEdgeKindLink, label, field, value, true)
}

// addMatchEdge records a record hanging off the connector for a value it carries. There is no
// parent side to a shared attribute.
func (w *graphWalker) addMatchEdge(record, connector models.GraphNode, label, field, value string) {
	w.addEdge(record, connector, graphEdgeKindMatch, label, field, value, false)
}

func (w *graphWalker) addEdge(a, b models.GraphNode, kind, label, field, value string, bIsParent bool) {
	if a == b {
		return
	}

	key := newGraphEdgeKey(a, b, label, value)
	if w.edgeSeen[key] {
		return
	}
	w.edgeSeen[key] = true

	w.adj[a] = append(w.adj[a], rawEdge{
		to: b, kind: kind, label: label, field: field, value: value, toParent: bIsParent,
	})
	w.adj[b] = append(w.adj[b], rawEdge{
		to: a, kind: kind, label: label, field: field, value: value, toParent: false,
	})
}

// connectLinkHypernode stands in for the children the walk refused to pull in, when one value of
// a link's field turned out to be carried by too many records. Rather than record that fact on
// every record that led there, it is a node of its own — the link equivalent of the connector a
// shared value gets — wired to the records concerned:
//
//	Account A1 --transactions_account--> (link connector: transactions_account, "A1", ~5000)
//
// Contraction then treats it like any other connector: it is kept, it terminates a path, and the
// party A1 collapses into ends up holding the edge to it.
func (w *graphWalker) connectLinkHypernode(origins []models.GraphNode, link models.LinkToSingle, originField, value string, count int) {
	hypernode := models.GraphNode{Type: link.Name, Id: value}

	admitted, isNew := w.registerNode(hypernode)
	if !admitted {
		return
	}
	if isNew {
		w.conns[hypernode] = graphEdgeKindLink
		// A link connector's Type is already the link's name and so displayable as it is, but
		// reporting the label on every connector spares the caller having to know that.
		w.connLabels[hypernode] = link.Name
	}
	w.markHypernode(hypernode, count)

	for _, origin := range origins {
		// Not addLinkEdge: that marks the far end as the record's parent, which drives ownersOf.
		// A hypernode owns nothing — it is a leaf standing for what was left out.
		w.addEdge(origin, hypernode, graphEdgeKindLink, link.Name, originField, value, false)
	}
}

// markHypernode records that a connector stands for something too widely shared to expand
// through, together with the estimated number of records concerned.
//
// The same value can be reached again in a later degree, and through two relations of one group
// pointing at different endpoints, so the largest estimate seen wins: reporting a
// smaller one afterwards would understate a relationship already known to be bigger than that.
func (w *graphWalker) markHypernode(connector models.GraphNode, count int) {
	w.pruned++

	if existing, ok := w.hyperConns[connector]; !ok || count > existing {
		w.hyperConns[connector] = count
	}
}

// graphEdgeKey identifies an edge without regard to which of its two ends it was described
// from. newGraphEdgeKey always sorts the endpoints the same way (graphNodeLess) before storing
// them, so the same relationship hashes to the same key no matter which side asked for it.
// This is what lets result()'s own edge dedup collapse the two contractedEdge calls that a
// kept-to-kept relationship necessarily produces — contractFrom runs once per kept node, so
// such a relationship is always found and turned into an edge from both ends (see the doc
// comment on contractedEdge).
type graphEdgeKey struct {
	lo, hi models.GraphNode
	label  string
	value  string
}

func newGraphEdgeKey(a, b models.GraphNode, label, value string) graphEdgeKey {
	if graphNodeLess(b, a) {
		a, b = b, a
	}
	return graphEdgeKey{lo: a, hi: b, label: label, value: value}
}

func graphNodeLess(a, b models.GraphNode) bool {
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	return a.Id < b.Id
}

func graphGroupByType(nodes []models.GraphNode) map[string][]models.GraphNode {
	byType := map[string][]models.GraphNode{}
	seen := map[models.GraphNode]bool{}

	for _, node := range nodes {
		if seen[node] {
			continue
		}
		seen[node] = true
		byType[node.Type] = append(byType[node.Type], node)
	}

	return byType
}

// ------------------------------------------------------------------------------------------------
// Contraction: turning the raw discovery graph into the result the caller gets back
// ------------------------------------------------------------------------------------------------

// isKept says whether a node is reported as-is rather than walked through and dropped.
func (w *graphWalker) isKept(node, start models.GraphNode) bool {
	if node == start {
		// The start node anchors the result, so it is always reported — even when it is an
		// intermediate record such as a transaction.
		return true
	}
	if _, ok := w.conns[node]; ok {
		return true
	}
	return w.schema.endTypes[node.Type]
}

// result collapses the raw discovery graph down to what the caller asked for: end nodes,
// connector nodes and the start node. Every other record is walked through and dropped, so two
// end nodes joined by a chain of accounts and transactions come back directly connected.
//
// It runs in four passes:
//
//  1. Every kept node that is not itself a connector runs contractFrom, which walks outward
//     over LINK edges through records that are not kept, until it reaches another kept node
//     (or gives up on a dead end), turning each path it finds into one direct edge.
//
//  2. Every MATCH connector runs attachConnector instead: its neighbours are the records that
//     carry the shared value, not records it is linked to, so it is wired to the reported owner
//     of each of those records rather than contracted. A LINK connector needs neither pass —
//     step 1 already reached it, since it sits at the end of an ordinary chain of link edges.
//
//  3. A connector that ends up next to fewer than two reported nodes after step 2 is dropped —
//     see the comment on the `dropped` map below for why this can happen even though a
//     connector is only ever created for a value that, at match time, at least two records
//     carried. A hypernode is exempt: saying "there are more of these" is the point of it.
//
//  4. The surviving kept nodes and the edges built in steps 1-2 are filtered by the drop list
//     from step 3 and returned.
func (w *graphWalker) result(start models.GraphNode) models.GraphResult {
	kept := make([]models.GraphNode, 0, len(w.order))

	for _, node := range w.order {
		if w.isKept(node, start) {
			kept = append(kept, node)
		}
	}

	edges := make([]models.GraphEdge, 0)
	edgeSeen := map[graphEdgeKey]bool{}
	degrees := map[models.GraphNode]int{}

	add := func(edge models.GraphEdge) {
		key := newGraphEdgeKey(edge.From, edge.To, edge.Label, edge.Value)
		if edgeSeen[key] {
			return
		}

		edgeSeen[key] = true
		edges = append(edges, edge)
		degrees[edge.From]++
		degrees[edge.To]++
	}

	for _, from := range kept {
		if _, isConnector := w.conns[from]; isConnector {
			// A connector's edges come from its own members below, not from walking out of it:
			// its neighbours are related to the value, not to each other's records.
			continue
		}

		for _, edge := range w.contractFrom(from, start) {
			add(edge)
		}
	}

	for _, connector := range kept {
		if _, isConnector := w.conns[connector]; !isConnector {
			continue
		}
		for _, edge := range w.attachConnector(connector, start) {
			add(edge)
		}
	}

	dropped := map[models.GraphNode]bool{}

	for node := range w.conns {
		// connect only ever creates a connector once at least two records are wired to it (or,
		// for a hypernode, unconditionally — see matchSameField). But wiring at match time is
		// not the same as being reported here: attachConnector re-derives each wired record's
		// *owner* — the reported node it ultimately belongs to, via ownersOf — and a record
		// whose type has no upward path to any kept type contributes no owner, and so no edge,
		// even though it was genuinely wired to the connector. A connector can therefore end up
		// with fewer than two edges even though at least two records carried its value.
		//
		// Such a connector is not useful to the caller: a value only one reported node ends up
		// next to is not a connection. The one exception is a hypernode connector — reporting
		// that a value exists and is shared far too widely to enumerate is the whole point of
		// creating it, regardless of how many edges (if any) survive to describe it.
		if _, isHypernode := w.hyperConns[node]; degrees[node] < 2 && !isHypernode {
			dropped[node] = true
		}
	}

	nodes := make([]models.GraphResultNode, 0, len(kept))

	for _, node := range kept {
		if dropped[node] {
			continue
		}

		resultNode := models.GraphResultNode{GraphNode: node}

		if kind, ok := w.conns[node]; ok {
			resultNode.Connector = true
			resultNode.ConnectorKind = kind
			// A match connector's Type is its group id, which identifies it but says nothing to
			// a reader, so what to call it travels next to it rather than in place of it.
			resultNode.Metadata.Label = w.connLabels[node]
			// Non-zero only for a connector standing for a value too widely shared to expand
			// through, which is what tells the caller its edges are a sample and not the
			// whole set.
			resultNode.HypernodeCount = w.hyperConns[node]
		}

		nodes = append(nodes, resultNode)
	}

	retained := make([]models.GraphEdge, 0, len(edges))

	for _, edge := range edges {
		if dropped[edge.From] || dropped[edge.To] {
			continue
		}
		retained = append(retained, edge)
	}

	return models.GraphResult{Start: start, Nodes: nodes, Edges: retained}
}

// graphContractStep is a position in the contraction walk: the record reached, the link names
// followed to get there, and the edges at either end of that path — which are the same edge
// when the path turns out to be one hop long.
type graphContractStep struct {
	node   models.GraphNode
	labels []string
	// tables is the record types the path went through, `from` included, in the order they were
	// visited. It always has exactly one more entry than labels, since every hop adds the record
	// it landed on.
	tables []string
	first  rawEdge
	last   rawEdge
}

// contractFrom finds every kept node reachable from `from` by walking only LINK edges through
// records that are not themselves reported, and turns each such path into a single edge. A
// match edge is never walked through here: reaching a connector, or a record on the far side
// of one, is attachConnector's job below, not this one's.
//
// It is a breadth-first search seeded with `from`'s own link edges, so the first kept node
// found on any branch is necessarily the nearest one. That matters, because contraction stops
// as soon as it reaches one: whatever lies beyond a kept node is that node's own concern once
// its turn comes to run contractFrom, not `from`'s.
//
//	from --L1--> x1 --L2--> x2 --L3--> KEPT      (x1 and x2 are not reported, so they vanish)
//
//	contractFrom(from) reports one edge:  from ---"L1 > L2 > L3", through [x1 x2]---> KEPT
func (w *graphWalker) contractFrom(from, start models.GraphNode) []models.GraphEdge {
	out := make([]models.GraphEdge, 0)

	visited := map[models.GraphNode]bool{from: true}
	queue := make([]graphContractStep, 0, len(w.adj[from]))

	for _, edge := range w.adj[from] {
		if edge.kind != graphEdgeKindLink {
			continue
		}

		queue = append(queue, graphContractStep{
			node:   edge.to,
			labels: []string{edge.label},
			tables: []string{from.Type, edge.to.Type},
			first:  edge,
			last:   edge,
		})
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if visited[cur.node] {
			continue
		}

		visited[cur.node] = true

		if w.isKept(cur.node, start) {
			// A kept node ends the path. Walking past it would report its own neighbours as
			// connected to `from`, which they are not.
			out = append(out, w.contractedEdge(from, cur))
			continue
		}

		for _, edge := range w.adj[cur.node] {
			// Only link paths are contracted. A shared attribute is not something to route
			// through: attachConnector decides which records own the value.
			if edge.kind != graphEdgeKindLink || visited[edge.to] {
				continue
			}

			queue = append(queue, graphContractStep{
				node:   edge.to,
				labels: append(slices.Clone(cur.labels), edge.label),
				tables: append(slices.Clone(cur.tables), edge.to.Type),
				first:  cur.first,
				last:   edge,
			})
		}
	}

	return out
}

// attachConnector joins a connector to the reported nodes that own the records carrying its
// value. Ownership only ever goes up a link, from the record holding the reference to the
// record it references. Reaching the owners by undirected reachability instead would go up to a
// shared parent and back down into a sibling's records, and so claim a party carries a value
// only its neighbour's records carry.
//
// Concretely: two records can be linked to the same parent without sharing anything else.
//
//	   Device D
//	   /      \
//	Login L1   Login L2      (both belong to device D, but belong to different parties)
//	   |           |
//	Party P1     Party P2
//
// If L1 and L2 carry different values on the matched field, only L1 actually carries the value
// this connector stands for. ownersOf(L1) climbs the link to P1 and stops there — it never
// crosses back down through D to L2 — so the connector is wired to P1 alone. Undirected
// reachability from the connector would instead cross through D and wire it to P2 as well,
// falsely claiming P2 carries a value that only L1, on the other side of the device, actually
// has.
func (w *graphWalker) attachConnector(connector, start models.GraphNode) []models.GraphEdge {
	out := make([]models.GraphEdge, 0)

	for _, member := range w.adj[connector] {
		if member.kind != graphEdgeKindMatch {
			continue
		}

		for _, owner := range w.ownersOf(member.to, start) {
			// The route from the reported node down to the record carrying the value, which is
			// what lets the edge explain itself: together with Field it says the value sits on
			// this node's own field, or on that of a record so many links beneath it. ownersOf
			// climbed the other way, and ends on the node the edge already names.
			through := make([]string, 0, len(owner.tables)-1)
			through = append(through, owner.tables[:len(owner.tables)-1]...)
			slices.Reverse(through)

			// Always oriented record to connector. Unlike a link between two records, this edge
			// is only ever described from one end, and what it says about the route reads from
			// that end: there is nothing to normalise here, and flipping it would leave the
			// route pointing away from the record it belongs to.
			out = append(out, models.GraphEdge{
				From:    owner.node,
				To:      connector,
				Kind:    graphEdgeKindMatch,
				Label:   member.label,
				Through: through,
				Field:   member.field,
				Value:   member.value,
			})
		}
	}

	return out
}

// graphOwner is a reported node a record belongs to, together with the record types climbed to
// get from that record up to it — the record's own type first, the owner's last. A record that
// owns itself has a one-entry path.
type graphOwner struct {
	node   models.GraphNode
	tables []string
}

// ownersOf returns the reported nodes a record belongs to, following links upward only and
// stopping at the first one found on each branch. A record that is itself reported owns itself.
//
// The result is a slice, not a single node, because a record can have more than one upward
// link — a transaction might reference both the account it was sent from and the party it was
// sent to — and each such link is its own branch, independently climbed to its own owner.
//
// Each owner carries the path climbed to reach it, so a caller can say not only which reported
// node a value belongs to but how far beneath it the record carrying that value sits.
func (w *graphWalker) ownersOf(record, start models.GraphNode) []graphOwner {
	if w.isKept(record, start) {
		return []graphOwner{{node: record, tables: []string{record.Type}}}
	}

	var owners []graphOwner

	visited := map[models.GraphNode]bool{record: true}
	queue := []graphOwner{{node: record, tables: []string{record.Type}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range w.adj[cur.node] {
			if edge.kind != graphEdgeKindLink || !edge.toParent || visited[edge.to] {
				continue
			}

			visited[edge.to] = true

			owner := graphOwner{node: edge.to, tables: append(slices.Clone(cur.tables), edge.to.Type)}

			if w.isKept(edge.to, start) {
				owners = append(owners, owner)
				continue
			}

			queue = append(queue, owner)
		}
	}

	return owners
}

// contractedEdge turns one BFS step from contractFrom into the single edge that step
// represents, orienting it — and its label path — the same way regardless of which end started
// the search (see the comment on graphEdgeKey for why that matters). Two shapes come out of it:
//
//   - a direct hop (len(labels) == 1), or a path that ends on a connector: the edge keeps that
//     last hop's own kind/label/field/value, since a shared value is the "why" of the
//     relationship no matter how many links it took to reach the connector holding it.
//   - a genuine multi-hop path through un-reported records: no single field or value names
//     what connects the two ends any more, so the edge becomes a link edge whose label is the
//     chain of the actual link names walked, joined by " > " (the "L1 > L2 > L3" example on
//     contractFrom above).
//
// Either way the edge reports what the path went *between*: the record types it collapsed away,
// with neither of its own ends among them — they are already named by the edge, and a connector
// end names no record type at all.
func (w *graphWalker) contractedEdge(from models.GraphNode, step graphContractStep) models.GraphEdge {
	to := step.node
	labels := step.labels
	_, toConnector := w.conns[to]

	// What the edge reports of its route is what lies between its two ends: both of them are
	// already named by the edge itself, and a connector end names no record type at all.
	through := make([]string, 0, len(step.tables)-2)
	through = append(through, step.tables[1:len(step.tables)-1]...)

	// Both ends of a relationship run their own contraction, so it is found twice. Orient the
	// edge and its path the same way each time, or it would be reported as two edges whose
	// labels merely read backwards from one another. An edge reaching a connector is found once,
	// from the record end, and is left that way round: what it says about the route only reads
	// one way, away from that record.
	if !toConnector && graphNodeLess(to, from) {
		from, to = to, from
		labels = slices.Clone(labels)
		slices.Reverse(labels)
		slices.Reverse(through)
	}

	// A path ending on a connector is a shared-attribute relationship, however many links were
	// walked to get to it: the value being shared is the "why", not the route there.
	if len(labels) == 1 || toConnector {
		return models.GraphEdge{
			From:    from,
			To:      to,
			Kind:    step.last.kind,
			Label:   step.last.label,
			Through: through,
			Field:   step.last.field,
			Value:   step.last.value,
		}
	}

	// The path went through records that are not reported, so no single field or value names
	// the relationship: the chain of link names is what is left of it.
	return models.GraphEdge{
		From:    from,
		To:      to,
		Kind:    graphEdgeKindLink,
		Label:   strings.Join(labels, " > "),
		Through: through,
	}
}

// graphUnion returns the deduplicated union of any number of node lists, keeping each node's
// first-seen order. It is the small utility run() uses to stitch a degree's separately
// discovered node lists (children, parents, matched, matchedParents) back into one frontier for
// the next degree.
func graphUnion(lists ...[]models.GraphNode) []models.GraphNode {
	out := make([]models.GraphNode, 0)
	seen := map[models.GraphNode]bool{}

	for _, list := range lists {
		for _, node := range list {
			if seen[node] {
				continue
			}
			seen[node] = true
			out = append(out, node)
		}
	}

	return out
}
