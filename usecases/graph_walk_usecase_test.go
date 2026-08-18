package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

///////////////////////////////
// Fake `_graph` table
///////////////////////////////

type fakeGraphRow struct {
	recordType string
	recordId   string
	fieldName  string
	fieldValue string
}

// fakeGraphRepository is an in-memory `_graph` table. It honours perValueLimit per value, so
// the fan-out caps are exercised through the same code path production uses.
//
// This is a deliberate behavioral fixture, not a stand-in for mocks.GraphRepository: the tests
// below drive the real graphWalker algorithm over several degrees and assert on the shape of
// its output graph, rather than on which calls were made — pinning that to argument-by-argument
// mock expectations would just pin the tests to the algorithm's internal call sequence.
type fakeGraphRepository struct {
	rows []fakeGraphRow

	// estimateOverride stands in for the planner's estimate, which is deliberately unrelated
	// to what the capped lookup managed to see.
	estimateOverride int

	// captions stands in for the records themselves, which is where a caption is read from:
	// captions[recordType][recordId].
	captions map[string]map[string]string

	// riskLevels stands in for the scoring table in the Marble database, keyed by record id.
	riskLevels map[string]int
}

func (repo *fakeGraphRepository) FetchFields(
	_ context.Context, _ repositories.Executor, recordType string, recordIds, fieldNames []string,
) ([]models.GraphRow, error) {
	out := make([]models.GraphRow, 0)
	for _, row := range repo.rows {
		if row.recordType != recordType {
			continue
		}
		if !slices.Contains(recordIds, row.recordId) || !slices.Contains(fieldNames, row.fieldName) {
			continue
		}
		out = append(out, models.GraphRow{
			RecordId:   row.recordId,
			FieldName:  row.fieldName,
			FieldValue: row.fieldValue,
		})
	}
	return out, nil
}

func (repo *fakeGraphRepository) FindByValues(
	_ context.Context, _ repositories.Executor, recordType, fieldName string,
	values []string, perValueLimit int,
) ([]models.GraphMatch, error) {
	out := make([]models.GraphMatch, 0)
	taken := map[string]int{}

	for _, row := range repo.rows {
		if row.recordType != recordType || row.fieldName != fieldName {
			continue
		}
		if !slices.Contains(values, row.fieldValue) {
			continue
		}
		if taken[row.fieldValue] >= perValueLimit {
			continue
		}
		taken[row.fieldValue]++
		out = append(out, models.GraphMatch{Value: row.fieldValue, RecordId: row.recordId})
	}

	return out, nil
}

func (repo *fakeGraphRepository) EstimateValueCount(
	_ context.Context, _ repositories.Executor, recordType, fieldName, value string,
) (int, error) {
	if repo.estimateOverride != 0 {
		return repo.estimateOverride, nil
	}

	count := 0
	for _, row := range repo.rows {
		if row.recordType == recordType && row.fieldName == fieldName && row.fieldValue == value {
			count++
		}
	}
	return count, nil
}

// GetNodeBatchMetadata answers the way the real query does: one row per requested record,
// tagged with that record's 1-based position in the request.
func (repo *fakeGraphRepository) GetNodeBatchMetadata(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	records []models.ScoringRecordRef,
) ([]models.GraphResultNodeMetadata, error) {
	if repo.riskLevels == nil {
		return []models.GraphResultNodeMetadata{}, nil
	}

	out := make([]models.GraphResultNodeMetadata, 0, len(records))

	for idx, record := range records {
		out = append(out, models.GraphResultNodeMetadata{
			Index:     idx + 1,
			RiskLevel: repo.riskLevels[record.RecordId],
		})
	}

	return out, nil
}

// GetNodeBatchCaptions answers positionally like the real query, and only for the record types it
// was asked to read a caption field for — so a caller passing no caption field for a type, or a
// blank slot for a connector, gets nothing back for it.
func (repo *fakeGraphRepository) GetNodeBatchCaptions(
	_ context.Context,
	_ repositories.Executor,
	captionFields map[string]string,
	records []models.ScoringRecordRef,
) ([]models.GraphResultNodeMetadata, error) {
	out := make([]models.GraphResultNodeMetadata, 0, len(records))

	for idx, record := range records {
		if _, ok := captionFields[record.RecordType]; !ok {
			continue
		}

		if caption := repo.captions[record.RecordType][record.RecordId]; caption != "" {
			out = append(out, models.GraphResultNodeMetadata{Index: idx + 1, Label: caption})
		}
	}

	return out, nil
}

///////////////////////////////
// Data model fixtures
///////////////////////////////

func graphTestTable(name string, semantic models.SemanticType, fields ...string) models.Table {
	table := models.Table{
		Name:          name,
		SemanticType:  semantic,
		Fields:        map[string]models.Field{},
		LinksToSingle: map[string]models.LinkToSingle{},
	}
	for _, field := range append([]string{"object_id"}, fields...) {
		table.Fields[field] = models.Field{Name: field, DataType: models.String}
	}
	return table
}

// graphTestLink declares the many-to-one direction: childTable.childField holds the value of
// parentTable.parentField.
func graphTestLink(name, childTable, childField, parentTable, parentField string) models.LinkToSingle {
	return models.LinkToSingle{
		Id:              name,
		Name:            name,
		ChildTableName:  childTable,
		ChildFieldName:  childField,
		ParentTableName: parentTable,
		ParentFieldName: parentField,
	}
}

func graphTestDataModel(tables []models.Table, links ...models.LinkToSingle) models.DataModel {
	dataModel := models.DataModel{Tables: map[string]models.Table{}}
	for _, table := range tables {
		dataModel.Tables[table.Name] = table
	}
	// Links live on the child table, which is where GetDataModel puts them.
	for _, link := range links {
		dataModel.Tables[link.ChildTableName].LinksToSingle[link.Name] = link
	}
	return dataModel
}

// amlDataModel is the shape the walk is designed for: parties owning accounts and devices,
// accounts owning transactions, and transactions optionally naming a counterparty party.
func amlDataModel() models.DataModel {
	return graphTestDataModel(
		[]models.Table{
			graphTestTable("users", models.SemanticTypePerson, "email"),
			graphTestTable("accounts", models.SemanticTypeAccount, "user_id", "iban", "swift"),
			graphTestTable("transactions", models.SemanticTypeTransaction,
				"account_id", "counterparty_user_id", "sender_iban", "receiver_iban"),
			graphTestTable("devices", models.SemanticTypeOther, "user_id", "ip_address"),
		},
		graphTestLink("accounts_user", "accounts", "user_id", "users", "object_id"),
		graphTestLink("transactions_account", "transactions", "account_id", "accounts", "object_id"),
		graphTestLink("transactions_counterparty", "transactions", "counterparty_user_id", "users", "object_id"),
		graphTestLink("devices_user", "devices", "user_id", "users", "object_id"),
	)
}

// graphTestRelation builds a relation the way the creation path does, with a generated id and,
// absent an existing group to join, a group of its own — same as CreateGraphRelation defaults a
// relation created without an explicit group_id.
func graphTestRelation(label, leftType, leftField, rightType, rightField string) models.GraphRelation {
	return models.GraphRelation{
		Id:         pure_utils.NewId(),
		GroupId:    pure_utils.NewId(),
		Label:      label,
		LeftType:   leftType,
		LeftField:  leftField,
		RightType:  rightType,
		RightField: rightField,
	}
}

func sameIbanConfig() models.GraphRelation {
	return graphTestRelation("same_iban", "accounts", "iban", "accounts", "iban")
}

func sameSwiftConfig() models.GraphRelation {
	return graphTestRelation("same_swift", "accounts", "swift", "accounts", "swift")
}

///////////////////////////////
// Harness
///////////////////////////////

// graphCaps overrides the walk's bounds. A zero field falls back to the production constant.
type graphCaps struct {
	nodes           int
	linkFanout      int
	upwardFanout    int
	sameFieldFanout int
	upwardDepth     int
	estimates       int
}

type graphWalkCase struct {
	dataModel models.DataModel
	rows      []fakeGraphRow
	endTypes  []string
	configs   []models.GraphRelation
	start     models.GraphNode
	degrees   int
	caps      graphCaps
	estimate  int
}

func runGraphWalk(t *testing.T, tc graphWalkCase) (models.GraphResult, *graphWalker) {
	t.Helper()

	orDefault := func(value, fallback int) int {
		if value == 0 {
			return fallback
		}
		return value
	}

	ctx := utils.StoreLoggerInContext(context.Background(),
		slog.New(slog.DiscardHandler))

	endTypes, err := resolveGraphEndTypes(tc.dataModel, tc.endTypes)
	require.NoError(t, err)

	repo := &fakeGraphRepository{rows: tc.rows, estimateOverride: tc.estimate}

	w := &graphWalker{
		ctx:                ctx,
		repo:               repo,
		schema:             buildGraphSchema(ctx, tc.dataModel, endTypes, tc.configs),
		maxNodes:           orDefault(tc.caps.nodes, graphMaxNodes),
		maxLinkFanout:      orDefault(tc.caps.linkFanout, graphMaxLinkFanout),
		maxUpwardFanout:    orDefault(tc.caps.upwardFanout, graphMaxUpwardFanout),
		maxSameFieldFanout: orDefault(tc.caps.sameFieldFanout, graphMaxSameFieldFanout),
		maxUpwardDepth:     orDefault(tc.caps.upwardDepth, graphMaxUpwardDepth),
		maxEstimates:       orDefault(tc.caps.estimates, graphMaxEstimates),
	}

	degrees := orDefault(tc.degrees, graphDefaultDegrees)
	result, err := w.run(tc.start, degrees)
	require.NoError(t, err)

	return result, w
}

func node(recordType, id string) models.GraphNode {
	return models.GraphNode{Type: recordType, Id: id}
}

func resultNodes(result models.GraphResult) map[models.GraphNode]models.GraphResultNode {
	out := map[models.GraphNode]models.GraphResultNode{}
	for _, n := range result.Nodes {
		out[n.GraphNode] = n
	}
	return out
}

func findEdge(result models.GraphResult, a, b models.GraphNode) (models.GraphEdge, bool) {
	for _, edge := range result.Edges {
		if (edge.From == a && edge.To == b) || (edge.From == b && edge.To == a) {
			return edge, true
		}
	}
	return models.GraphEdge{}, false
}

// throughFrom reads an edge's route starting from the given endpoint, so a test can state the
// route it expects without depending on which way round the result happened to orient the edge
// (see graphNodeLess). An edge reaching a connector is always oriented record-first, so those are
// asserted on directly instead.
func throughFrom(edge models.GraphEdge, from models.GraphNode) []string {
	through := slices.Clone(edge.Through)
	if edge.To == from {
		slices.Reverse(through)
	}
	return through
}

// matchConnector is the node the connector for a value shared through a relation is identified
// by: the relation's group id and that value — never its label, which two independent groups may
// share (see connect).
func matchConnector(relation models.GraphRelation, value string) models.GraphNode {
	return node(relation.GroupId.String(), value)
}

func connectorNodes(result models.GraphResult) []models.GraphResultNode {
	var out []models.GraphResultNode
	for _, n := range result.Nodes {
		if n.Connector {
			out = append(out, n)
		}
	}
	return out
}

// userWithAccount is the minimal party -> account shape, repeated across most fixtures.
func userWithAccount(user, account, iban string) []fakeGraphRow {
	rows := []fakeGraphRow{
		{"users", user, "object_id", user},
		{"accounts", account, "object_id", account},
		{"accounts", account, "user_id", user},
	}
	if iban != "" {
		rows = append(rows, fakeGraphRow{"accounts", account, "iban", iban})
	}
	return rows
}

///////////////////////////////
// Traversal shape
///////////////////////////////

func TestGraphWalk_DownwardFollowedOneLevelPerDegree(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", ""),
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A1"},
	)

	_, one := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 1,
	})
	assert.True(t, one.seen[node("accounts", "A1")], "the first degree reaches the account")
	assert.False(t, one.seen[node("transactions", "T1")],
		"a downward link is followed one level only, so the account's transactions wait for the next degree")

	_, two := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 2,
	})
	assert.True(t, two.seen[node("transactions", "T1")])
}

func TestGraphWalk_EveryDownwardLinkFollowed(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", ""),
		fakeGraphRow{"devices", "D1", "object_id", "D1"},
		fakeGraphRow{"devices", "D1", "user_id", "U1"},
	)

	_, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 1,
	})

	assert.True(t, w.seen[node("accounts", "A1")])
	assert.True(t, w.seen[node("devices", "D1")])
}

func TestGraphWalk_UpwardClosureIsUnbounded(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", ""),
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A1"},
	)

	result, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("transactions", "T1"), degrees: 1,
	})

	// Two upward hops in a single degree: the transaction's account, then that account's owner.
	assert.True(t, w.seen[node("accounts", "A1")])
	assert.True(t, w.seen[node("users", "U1")])

	edge, ok := findEdge(result, node("transactions", "T1"), node("users", "U1"))
	require.True(t, ok, "the transaction and the party it belongs to are directly connected")
	assert.Equal(t, graphEdgeKindLink, edge.Kind)
	assert.Equal(t, "transactions_account > accounts_user", edge.Label)
	assert.Equal(t, []string{"accounts"}, throughFrom(edge, node("transactions", "T1")),
		"the account walked through is reported even though it is not itself a node")
}

func TestGraphWalk_UpwardLinkCycleTerminates(t *testing.T) {
	dataModel := graphTestDataModel(
		[]models.Table{
			graphTestTable("left", models.SemanticTypePerson, "right_id"),
			graphTestTable("right", models.SemanticTypeOther, "left_id"),
		},
		graphTestLink("left_right", "left", "right_id", "right", "object_id"),
		graphTestLink("right_left", "right", "left_id", "left", "object_id"),
	)

	rows := []fakeGraphRow{
		{"left", "L1", "object_id", "L1"},
		{"left", "L1", "right_id", "R1"},
		{"right", "R1", "object_id", "R1"},
		{"right", "R1", "left_id", "L1"},
	}

	_, w := runGraphWalk(t, graphWalkCase{
		dataModel: dataModel, rows: rows, start: node("left", "L1"), degrees: 3,
	})

	assert.True(t, w.seen[node("right", "R1")])
	assert.Len(t, w.seen, 2, "a cycle must not keep rediscovering the same two records")
}

func TestGraphWalk_UpwardClosureIsBoundedByDepth(t *testing.T) {
	// A four-deep chain of many-to-one links, which a well-formed model would not have, but a
	// cyclic or pathological one might.
	dataModel := graphTestDataModel(
		[]models.Table{
			graphTestTable("l0", models.SemanticTypeOther, "parent"),
			graphTestTable("l1", models.SemanticTypeOther, "parent"),
			graphTestTable("l2", models.SemanticTypeOther, "parent"),
			graphTestTable("l3", models.SemanticTypePerson),
		},
		graphTestLink("l0_l1", "l0", "parent", "l1", "object_id"),
		graphTestLink("l1_l2", "l1", "parent", "l2", "object_id"),
		graphTestLink("l2_l3", "l2", "parent", "l3", "object_id"),
	)

	rows := []fakeGraphRow{
		{"l0", "N0", "parent", "N1"},
		{"l1", "N1", "object_id", "N1"},
		{"l1", "N1", "parent", "N2"},
		{"l2", "N2", "object_id", "N2"},
		{"l2", "N2", "parent", "N3"},
		{"l3", "N3", "object_id", "N3"},
	}

	_, w := runGraphWalk(t, graphWalkCase{
		dataModel: dataModel, rows: rows, start: node("l0", "N0"), degrees: 1,
		caps: graphCaps{upwardDepth: 2},
	})

	assert.True(t, w.seen[node("l1", "N1")])
	assert.True(t, w.seen[node("l2", "N2")])
	assert.False(t, w.seen[node("l3", "N3")], "the closure stops at the depth bound")
}

///////////////////////////////
// Shared attributes
///////////////////////////////

func TestGraphWalk_SameFieldConnectorJoinsTwoParties(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "IB1")...)

	config := sameIbanConfig()

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{config},
		start:   node("users", "U1"), degrees: 1,
	})

	connector := matchConnector(config, "IB1")
	nodes := resultNodes(result)

	require.Contains(t, nodes, connector)
	assert.True(t, nodes[connector].Connector)
	assert.Equal(t, graphEdgeKindMatch, nodes[connector].ConnectorKind)
	assert.Equal(t, "same_iban", nodes[connector].Metadata.Label,
		"the connector is identified by its group but reported under its label")

	// The other party is reached in the same degree: the shared attribute finds its account,
	// and the upward closure that follows finds its owner.
	assert.Contains(t, nodes, node("users", "U2"))

	// Parties sharing an attribute are joined *through* the connector, never directly.
	_, direct := findEdge(result, node("users", "U1"), node("users", "U2"))
	assert.False(t, direct)

	edge, ok := findEdge(result, node("users", "U1"), connector)
	require.True(t, ok)
	assert.Equal(t, graphEdgeKindMatch, edge.Kind)
	assert.Equal(t, "same_iban", edge.Label)

	// The edge explains itself: from the party, down to one of its accounts, whose iban carries
	// the value. It reads that way round because a connector edge is always oriented from the
	// record, never towards it.
	assert.Equal(t, node("users", "U1"), edge.From)
	assert.Equal(t, connector, edge.To)
	assert.Equal(t, []string{"accounts"}, edge.Through)
	assert.Equal(t, "iban", edge.Field)
	assert.Equal(t, "IB1", edge.Value)

	_, ok = findEdge(result, node("users", "U2"), connector)
	assert.True(t, ok)

	// The accounts themselves are intermediates and are not reported.
	assert.NotContains(t, nodes, node("accounts", "A1"))
}

func TestGraphWalk_PartiesCanShareAnAttributeDirectly(t *testing.T) {
	// The shared field is on the party itself, so there is no intermediate to contract and the
	// connector edge is a single hop.
	config := graphTestRelation("same_email", "users", "email", "users", "email")

	rows := []fakeGraphRow{
		{"users", "U1", "object_id", "U1"},
		{"users", "U1", "email", "a@b.c"},
		{"users", "U2", "object_id", "U2"},
		{"users", "U2", "email", "a@b.c"},
	}

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, configs: []models.GraphRelation{config},
		start: node("users", "U1"), degrees: 1,
	})

	connector := matchConnector(config, "a@b.c")
	assert.Contains(t, resultNodes(result), connector)
	assert.Contains(t, resultNodes(result), node("users", "U2"))

	edge, ok := findEdge(result, node("users", "U2"), connector)
	require.True(t, ok)
	assert.Equal(t, graphEdgeKindMatch, edge.Kind)
	assert.Equal(t, "email", edge.Field)
	assert.Equal(t, "a@b.c", edge.Value)

	// Nothing lies between the party and the value: it is carried on the party's own field, which
	// is what an empty route on a match edge says.
	assert.Equal(t, node("users", "U2"), edge.From)
	assert.Empty(t, edge.Through)
}

func TestGraphWalk_UnsharedValueYieldsNoConnector(t *testing.T) {
	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: userWithAccount("U1", "A1", "IB1"),
		configs: []models.GraphRelation{sameIbanConfig()},
		start:   node("users", "U1"), degrees: 2,
	})

	// A self-config returns the origin among its own matches, so the deduplication of
	// participants is what keeps a record from connecting to itself.
	assert.Empty(t, connectorNodes(result))
	assert.Len(t, result.Nodes, 1)
	assert.Empty(t, result.Edges)
}

func TestGraphWalk_SameFieldResultsAreNotRematchedInTheSameDegree(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "IB1")...)
	rows = append(rows, userWithAccount("U3", "A3", "IB9")...)
	// A2 and A3 share a SWIFT code, so U3 is only reachable by matching a second time.
	rows = append(rows,
		fakeGraphRow{"accounts", "A1", "swift", "SW1"},
		fakeGraphRow{"accounts", "A2", "swift", "SW2"},
		fakeGraphRow{"accounts", "A3", "swift", "SW2"},
	)

	configs := []models.GraphRelation{sameIbanConfig(), sameSwiftConfig()}

	one, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, configs: configs,
		start: node("users", "U1"), degrees: 1,
	})
	assert.Contains(t, resultNodes(one), node("users", "U2"),
		"the upward closure of a shared-attribute match runs in the same degree")
	assert.NotContains(t, resultNodes(one), node("users", "U3"),
		"a record found by matching is not itself re-matched before the next degree")

	two, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, configs: configs,
		start: node("users", "U1"), degrees: 2,
	})
	assert.Contains(t, resultNodes(two), node("users", "U3"))
}

func TestGraphWalk_ConfigPairSetConvergesOnOneConnector(t *testing.T) {
	// One conceptual group of three endpoints, spelled out as the three one-to-one relations it
	// takes to express it. They share a group id, so they must collapse onto a single connector.
	label := "shared_iban"
	groupId := pure_utils.NewId()
	inGroup := func(r models.GraphRelation) models.GraphRelation {
		r.GroupId = groupId
		return r
	}
	configs := []models.GraphRelation{
		inGroup(graphTestRelation(label, "accounts", "iban", "transactions", "sender_iban")),
		inGroup(graphTestRelation(label, "transactions", "sender_iban", "transactions", "receiver_iban")),
		inGroup(graphTestRelation(label, "accounts", "iban", "transactions", "receiver_iban")),
	}

	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "")...)
	rows = append(rows, userWithAccount("U3", "A3", "")...)
	rows = append(rows,
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A2"},
		fakeGraphRow{"transactions", "T1", "sender_iban", "IB1"},
		fakeGraphRow{"transactions", "T2", "object_id", "T2"},
		fakeGraphRow{"transactions", "T2", "account_id", "A3"},
		fakeGraphRow{"transactions", "T2", "receiver_iban", "IB1"},
	)

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, configs: configs,
		start: node("users", "U1"), degrees: 1,
	})

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	assert.Equal(t, matchConnector(configs[0], "IB1"), connectors[0].GraphNode)
	assert.Equal(t, label, connectors[0].Metadata.Label)

	nodes := resultNodes(result)
	for _, user := range []string{"U1", "U2", "U3"} {
		assert.Contains(t, nodes, node("users", user))
		_, ok := findEdge(result, node("users", user), connectors[0].GraphNode)
		assert.True(t, ok, "%s hangs off the shared connector", user)
	}
}

func TestGraphWalk_DifferentLabelsSameGroupConvergeOnOneConnector(t *testing.T) {
	// Two relations describing the same shared attribute, given different labels (say, one was
	// renamed after the other was created against its group), but sharing a group id. Grouping
	// must go by GroupId, not by Label matching, so they still collapse onto a single connector.
	// Which of the two labels ends up reported on it is deterministic (whichever relation is
	// processed first — here, configA) but incidental: the creation path keeps every relation in
	// a group on the same label, so a real group never actually exercises this choice.
	groupId := pure_utils.NewId()
	configA := graphTestRelation("iban_match", "accounts", "iban", "accounts", "iban")
	configA.GroupId = groupId
	configB := graphTestRelation("shared_iban_legacy", "accounts", "iban", "accounts", "iban")
	configB.GroupId = groupId

	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "IB1")...)

	result, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{configA, configB},
		start:   node("users", "U1"), degrees: 1,
	})

	// Internally, both relations resolve to the very same identity: the group id, not the label.
	assert.True(t, w.seen[node(groupId.String(), "IB1")])

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1, "a shared group id converges onto one connector regardless of label")
	assert.Equal(t, matchConnector(configA, "IB1"), connectors[0].GraphNode)
	assert.Equal(t, "iban_match", connectors[0].Metadata.Label,
		"the one connector is reported under one of its group's labels")
}

func TestGraphWalk_SameLabelDifferentGroupsProduceSeparateConnectors(t *testing.T) {
	// Two groups an organization chose to label the same — say two sets of IBANs it wants
	// followed independently under one name. Nothing about them may be merged on the strength of
	// that shared label: they are distinct groups, so they are distinct connectors, all the way
	// out to the result. This is the case that requires the label to be reported *beside* the
	// identity rather than as it.
	configA := graphTestRelation("same_iban", "accounts", "iban", "accounts", "iban")
	configB := graphTestRelation("same_iban", "accounts", "iban", "accounts", "iban")
	require.NotEqual(t, configA.GroupId, configB.GroupId)

	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "IB1")...)

	result, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{configA, configB},
		start:   node("users", "U1"), degrees: 1,
	})

	// The two groups never share a node: distinct group ids mean distinct identities, even though
	// they matched on the exact same value.
	assert.True(t, w.seen[matchConnector(configA, "IB1")])
	assert.True(t, w.seen[matchConnector(configB, "IB1")])

	// Both surface in the result as their own connector, each identified by its own group and
	// each reported under the label they happen to share.
	connectors := connectorNodes(result)
	require.Len(t, connectors, 2, "identical labels never converge across distinct group ids")

	assert.ElementsMatch(t,
		[]models.GraphNode{matchConnector(configA, "IB1"), matchConnector(configB, "IB1")},
		[]models.GraphNode{connectors[0].GraphNode, connectors[1].GraphNode})

	for _, connector := range connectors {
		assert.Equal(t, "same_iban", connector.Metadata.Label)
	}

	// Nothing in the payload is ambiguous: two nodes never answer to the same Type/Id, so a
	// caller keying nodes by their identity cannot silently merge the two groups.
	identities := map[models.GraphNode]int{}
	for _, resultNode := range result.Nodes {
		identities[resultNode.GraphNode]++
	}
	for identity, count := range identities {
		assert.Equal(t, 1, count, "node %v is reported more than once", identity)
	}

	// Each connector keeps its own edge to U1 and to U2, and all four are distinct.
	assert.Len(t, result.Edges, 4)
	for _, connector := range connectors {
		for _, user := range []string{"U1", "U2"} {
			_, ok := findEdge(result, node("users", user), connector.GraphNode)
			assert.True(t, ok, "%s hangs off connector %v", user, connector.GraphNode)
		}
	}
}

// loginsDataModel is the parties-share-a-device shape: a login belongs to both a user and a
// device, so a device is a hub two parties can hang off.
func loginsDataModel() models.DataModel {
	return graphTestDataModel(
		[]models.Table{
			graphTestTable("users", models.SemanticTypePerson),
			graphTestTable("devices", models.SemanticTypeOther),
			graphTestTable("logins", models.SemanticTypeOther, "user_id", "device_id", "ip"),
		},
		graphTestLink("login_user", "logins", "user_id", "users", "object_id"),
		graphTestLink("login_device", "logins", "device_id", "devices", "object_id"),
	)
}

func login(id, user, device, ip string) []fakeGraphRow {
	return []fakeGraphRow{
		{"logins", id, "user_id", user},
		{"logins", id, "device_id", device},
		{"logins", id, "ip", ip},
	}
}

func TestGraphWalk_SharedIntermediateDoesNotLeakAnotherPartysAttribute(t *testing.T) {
	// U1 and U2 both log in from device D, so they are genuinely related through a link. But
	// IP2 is carried only by U2's own logins, so U1 must not be reported as matching on it:
	// reaching it means going up from U1's login to the shared device and back down into U2's,
	// which is not what a shared attribute claims.
	config := graphTestRelation("same_ip", "logins", "ip", "logins", "ip")

	rows := []fakeGraphRow{
		{"users", "U1", "object_id", "U1"},
		{"users", "U2", "object_id", "U2"},
		{"devices", "D", "object_id", "D"},
		{"devices", "D2", "object_id", "D2"},
	}
	rows = append(rows, login("L1", "U1", "D", "IP1")...)
	rows = append(rows, login("L2", "U2", "D", "IP2")...)
	rows = append(rows, login("L3", "U2", "D2", "IP2")...)

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: loginsDataModel(), rows: rows,
		configs: []models.GraphRelation{config},
		start:   node("users", "U1"), degrees: 2,
	})

	nodes := resultNodes(result)
	assert.Contains(t, nodes, node("users", "U2"), "the shared device does relate them")

	// IP2 is shared, but only between two of U2's own logins, so it connects one party and is
	// not a connection at all.
	assert.Empty(t, connectorNodes(result))
	for _, edge := range result.Edges {
		assert.Equal(t, graphEdgeKindLink, edge.Kind,
			"the only relationship here is the shared device")
	}

	edge, ok := findEdge(result, node("users", "U1"), node("users", "U2"))
	require.True(t, ok)
	assert.Equal(t, "login_user > login_device > login_device > login_user", edge.Label)
	assert.Equal(t, []string{"logins", "devices", "logins"},
		throughFrom(edge, node("users", "U1")),
		"the route through the shared device is spelled out table by table")
}

func TestGraphWalk_ConnectorAttachesOnlyToTheOwnersOfItsValue(t *testing.T) {
	// Same shape, but now the IP really is shared across the two parties, so it must surface —
	// and the third party, related only through the device, must not be pulled onto it.
	config := graphTestRelation("same_ip", "logins", "ip", "logins", "ip")

	rows := []fakeGraphRow{
		{"users", "U1", "object_id", "U1"},
		{"users", "U2", "object_id", "U2"},
		{"users", "U3", "object_id", "U3"},
		{"devices", "D", "object_id", "D"},
	}
	rows = append(rows, login("L1", "U1", "D", "IP1")...)
	rows = append(rows, login("L2", "U2", "D", "IP1")...)
	rows = append(rows, login("L3", "U3", "D", "IP9")...)

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: loginsDataModel(), rows: rows,
		configs: []models.GraphRelation{config},
		start:   node("users", "U1"), degrees: 2,
	})

	connector := matchConnector(config, "IP1")
	require.Contains(t, resultNodes(result), connector)

	for _, user := range []string{"U1", "U2"} {
		_, ok := findEdge(result, node("users", user), connector)
		assert.True(t, ok, "%s carries IP1", user)
	}

	_, ok := findEdge(result, node("users", "U3"), connector)
	assert.False(t, ok, "U3 shares the device, not the IP")
}

///////////////////////////////
// Hypernodes and caps
///////////////////////////////

func TestGraphWalk_HypernodeIsReportedButNotWalked(t *testing.T) {
	rows := userWithAccount("U1", "A1", "IB1")
	for _, account := range []string{"A2", "A3", "A4"} {
		rows = append(rows, userWithAccount(fmt.Sprintf("U_%s", account), account, "IB1")...)
	}

	config := sameIbanConfig()

	result, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{config},
		start:   node("users", "U1"), degrees: 2,
		caps:     graphCaps{sameFieldFanout: 2},
		estimate: 9999,
	})

	assert.False(t, w.seen[node("accounts", "A2")], "an over-cardinality value is never walked through")

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1, "the connector is kept as a terminal marker")
	assert.Equal(t, matchConnector(config, "IB1"), connectors[0].GraphNode)
	assert.Equal(t, "same_iban", connectors[0].Metadata.Label)

	// The connector is the one node standing for the shared value, so the count belongs on it
	// rather than being repeated on every record that carries the value.
	assert.Equal(t, graphEdgeKindMatch, connectors[0].ConnectorKind)
	assert.Equal(t, 9999, connectors[0].HypernodeCount)
}

func TestGraphWalk_HypernodeReportsALowerBoundOnceEstimatesRunOut(t *testing.T) {
	rows := userWithAccount("U1", "A1", "IB1")
	for _, account := range []string{"A2", "A3", "A4"} {
		rows = append(rows, userWithAccount(fmt.Sprintf("U_%s", account), account, "IB1")...)
	}

	result, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{sameIbanConfig()},
		start:   node("users", "U1"), degrees: 1,
		caps:     graphCaps{sameFieldFanout: 2, estimates: -1},
		estimate: 9999,
	})

	assert.Zero(t, w.estimates, "the estimate budget was already spent")

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	// What the capped lookup proved, rather than the planner's number.
	assert.Equal(t, 3, connectors[0].HypernodeCount)
}

func TestGraphWalk_HypernodeNeverReportsFewerThanItProved(t *testing.T) {
	rows := userWithAccount("U1", "A1", "IB1")
	for _, account := range []string{"A2", "A3", "A4"} {
		rows = append(rows, userWithAccount(fmt.Sprintf("U_%s", account), account, "IB1")...)
	}

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		configs: []models.GraphRelation{sameIbanConfig()},
		start:   node("users", "U1"), degrees: 1,
		caps: graphCaps{sameFieldFanout: 2},
		// The planner routinely lands under the truth, and a count below the threshold that
		// caused the pruning would contradict the pruning itself.
		estimate: 1,
	})

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	assert.Equal(t, 3, connectors[0].HypernodeCount)
}

func TestGraphWalk_LinkFanoutUsesItsOwnHigherCap(t *testing.T) {
	rows := []fakeGraphRow{{"users", "U1", "object_id", "U1"}}
	for _, account := range []string{"A1", "A2", "A3", "A4"} {
		rows = append(rows,
			fakeGraphRow{"accounts", account, "object_id", account},
			fakeGraphRow{"accounts", account, "user_id", "U1"},
		)
	}

	// A fan-out well past the shared-attribute threshold is normal for a link: a party
	// legitimately owns many records.
	generousResult, generous := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 1,
		caps: graphCaps{sameFieldFanout: 2, linkFanout: 10},
	})
	for _, account := range []string{"A1", "A2", "A3", "A4"} {
		assert.True(t, generous.seen[node("accounts", account)])
	}
	assert.Empty(t, connectorNodes(generousResult), "nothing was pruned, so nothing stands in for it")

	result, strict := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 1,
		caps: graphCaps{linkFanout: 2}, estimate: 4,
	})
	assert.False(t, strict.seen[node("accounts", "A1")])

	// The children that were left out get a node of their own, named after the link and the
	// value they point at, exactly as a shared value gets one.
	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	assert.Equal(t, node("accounts_user", "U1"), connectors[0].GraphNode)
	assert.Equal(t, graphEdgeKindLink, connectors[0].ConnectorKind)
	assert.Equal(t, 4, connectors[0].HypernodeCount)
	// A link connector is already named after its link, but every connector reports a label, so
	// a caller never has to know which kinds carry a displayable Type and which do not.
	assert.Equal(t, "accounts_user", connectors[0].Metadata.Label)

	// ...and the party that reached it holds the edge, which names the link it stands for.
	edge, ok := findEdge(result, node("users", "U1"), connectors[0].GraphNode)
	require.True(t, ok)
	assert.Equal(t, graphEdgeKindLink, edge.Kind)
	assert.Equal(t, "accounts_user", edge.Label)
	assert.Equal(t, node("users", "U1"), edge.From, "a connector edge is oriented from the record")
	assert.Empty(t, edge.Through,
		"nothing lies between the party and the set of its records that was left out")
}

func TestGraphWalk_HypernodeKeepsBothValuesPrunedOnTheSameLink(t *testing.T) {
	// One party owning two accounts, each of which has more transactions than the link cap
	// allows. Both accounts collapse into the same party, and both were pruned on the same link
	// — differing only in the value they point at. Each set left out is its own node, so
	// reporting one must not hide the other.
	rows := []fakeGraphRow{{"users", "U1", "object_id", "U1"}}
	for _, account := range []string{"A1", "A2"} {
		rows = append(rows,
			fakeGraphRow{"accounts", account, "object_id", account},
			fakeGraphRow{"accounts", account, "user_id", "U1"},
		)
		for _, tx := range []string{"T1", "T2", "T3"} {
			rows = append(rows,
				fakeGraphRow{"transactions", account + tx, "object_id", account + tx},
				fakeGraphRow{"transactions", account + tx, "account_id", account},
			)
		}
	}

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows,
		start: node("users", "U1"), degrees: 2,
		caps: graphCaps{linkFanout: 2}, estimate: 3,
	})

	connectors := connectorNodes(result)
	require.Len(t, connectors, 2,
		"both accounts were pruned on transactions_account, for different values")

	ids := []string{connectors[0].Id, connectors[1].Id}
	slices.Sort(ids)
	assert.Equal(t, []string{"A1", "A2"}, ids)
	for _, connector := range connectors {
		assert.Equal(t, "transactions_account", connector.Type)
		assert.Equal(t, graphEdgeKindLink, connector.ConnectorKind)
		assert.Equal(t, 3, connector.HypernodeCount)
	}
}

func TestGraphWalk_NodeCapStopsTheWalkWithoutDanglingEdges(t *testing.T) {
	rows := []fakeGraphRow{{"users", "U1", "object_id", "U1"}}
	for _, account := range []string{"A1", "A2", "A3", "A4", "A5"} {
		rows = append(rows,
			fakeGraphRow{"accounts", account, "object_id", account},
			fakeGraphRow{"accounts", account, "user_id", "U1"},
		)
	}

	_, w := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 2,
		caps: graphCaps{nodes: 3},
	})

	assert.Len(t, w.seen, 3)
	assert.True(t, w.nodeCapHit)

	for from, edges := range w.adj {
		assert.True(t, w.seen[from], "%v is an edge endpoint but was never admitted", from)
		for _, edge := range edges {
			assert.True(t, w.seen[edge.to], "%v is an edge endpoint but was never admitted", edge.to)
		}
	}
}

///////////////////////////////
// Contraction
///////////////////////////////

func TestGraphWalk_LinkedPartiesAreReportedAsDirectlyConnected(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", ""),
		fakeGraphRow{"users", "U2", "object_id", "U2"},
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A1"},
		fakeGraphRow{"transactions", "T1", "counterparty_user_id", "U2"},
	)

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, start: node("users", "U1"), degrees: 2,
	})

	nodes := resultNodes(result)
	assert.Contains(t, nodes, node("users", "U2"))
	assert.NotContains(t, nodes, node("accounts", "A1"))
	assert.NotContains(t, nodes, node("transactions", "T1"))

	edge, ok := findEdge(result, node("users", "U1"), node("users", "U2"))
	require.True(t, ok)
	assert.Equal(t, graphEdgeKindLink, edge.Kind)
	assert.Equal(t, "accounts_user > transactions_account > transactions_counterparty", edge.Label)
	assert.Equal(t, []string{"accounts", "transactions"}, throughFrom(edge, node("users", "U1")),
		"the two records collapsed away, without repeating the parties at either end")

	// The relationship is found once from each end; it must be reported once.
	assert.Len(t, result.Edges, 1)
}

func TestGraphWalk_StartNodeIsReportedEvenWhenNotAParty(t *testing.T) {
	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: userWithAccount("U1", "A1", ""),
		start: node("accounts", "A1"), degrees: 1,
	})

	nodes := resultNodes(result)
	assert.Contains(t, nodes, node("accounts", "A1"), "the start node anchors the result")
	assert.Contains(t, nodes, node("users", "U1"))
	assert.Equal(t, node("accounts", "A1"), result.Start)

	edge, ok := findEdge(result, node("accounts", "A1"), node("users", "U1"))
	require.True(t, ok)
	assert.Equal(t, "accounts_user", edge.Label)
	assert.Equal(t, "user_id", edge.Field)
	assert.Equal(t, "U1", edge.Value)
	assert.Empty(t, edge.Through, "a single hop has nothing between its two ends")
}

func TestGraphWalk_ExplicitEndTypesReplaceParties(t *testing.T) {
	rows := append(userWithAccount("U1", "A1", ""),
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A1"},
	)

	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, endTypes: []string{"accounts"},
		start: node("users", "U1"), degrees: 2,
	})

	nodes := resultNodes(result)
	assert.Contains(t, nodes, node("accounts", "A1"))
	assert.NotContains(t, nodes, node("transactions", "T1"))
}

///////////////////////////////
// Schema and options
///////////////////////////////

func TestResolveGraphEndTypes(t *testing.T) {
	dataModel := amlDataModel()

	t.Run("defaults to party tables", func(t *testing.T) {
		endTypes, err := resolveGraphEndTypes(dataModel, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"users": true}, endTypes)
	})

	t.Run("honours an explicit request", func(t *testing.T) {
		endTypes, err := resolveGraphEndTypes(dataModel, []string{"accounts", "devices"})
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"accounts": true, "devices": true}, endTypes)
	})

	t.Run("rejects an unknown table", func(t *testing.T) {
		_, err := resolveGraphEndTypes(dataModel, []string{"nope"})
		assert.ErrorIs(t, err, models.BadParameterError)
	})
}

func TestBuildGraphSchema_DropsConfigsAbsentFromTheDataModel(t *testing.T) {
	// A config whose table or field no longer resolves has drifted from the data model it was
	// defined against. One stale config must not take the whole walk down with it.
	dataModel := amlDataModel()
	configs := []models.GraphRelation{
		sameIbanConfig(),
		graphTestRelation("same_ip", "gadgets", "ip", "gadgets", "ip"),
		graphTestRelation("same_x", "accounts", "nope", "accounts", "nope"),
	}

	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	sch := buildGraphSchema(ctx, dataModel, map[string]bool{"users": true}, configs)

	require.Len(t, sch.sameField["accounts"], 1)
	assert.Equal(t, "same_iban", sch.sameField["accounts"][0].Label)
	assert.Empty(t, sch.sameField["gadgets"])

	// A downward link is keyed by the parent it is followed from, an upward one by the child.
	assert.Len(t, sch.downward["users"], 3)
	assert.Len(t, sch.upward["transactions"], 2)

	assert.Equal(t, []string{"iban", "object_id", "user_id"}, sch.neededFields["accounts"])
}

func TestBuildGraphSchema_RegistersASameTableConfigOnce(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))

	// Both endpoints live on the same table, so it would be easy to register the config twice
	// and pay for every one of its lookups twice.
	config := graphTestRelation("shared_iban", "transactions", "sender_iban",
		"transactions", "receiver_iban")

	sch := buildGraphSchema(ctx, amlDataModel(), map[string]bool{"users": true},
		[]models.GraphRelation{config})

	assert.Len(t, sch.sameField["transactions"], 1)

	// Only the fields the walk can traverse are hydrated. Nothing links *to* a transaction, so
	// its object_id is never read.
	assert.Equal(t, []string{"account_id", "counterparty_user_id", "receiver_iban", "sender_iban"},
		sch.neededFields["transactions"])
}

func TestGraphWalk_SameTableConfigMatchesBothOfItsFields(t *testing.T) {
	config := graphTestRelation("shared_iban", "transactions", "sender_iban",
		"transactions", "receiver_iban")

	rows := append(userWithAccount("U1", "A1", ""), userWithAccount("U2", "A2", "")...)
	rows = append(rows,
		fakeGraphRow{"transactions", "T1", "object_id", "T1"},
		fakeGraphRow{"transactions", "T1", "account_id", "A1"},
		fakeGraphRow{"transactions", "T1", "sender_iban", "IB1"},
		fakeGraphRow{"transactions", "T2", "object_id", "T2"},
		fakeGraphRow{"transactions", "T2", "account_id", "A2"},
		fakeGraphRow{"transactions", "T2", "receiver_iban", "IB1"},
	)

	// Three degrees: reach the account, then its transaction, then match on the transaction.
	result, _ := runGraphWalk(t, graphWalkCase{
		dataModel: amlDataModel(), rows: rows, configs: []models.GraphRelation{config},
		start: node("users", "U1"), degrees: 3,
	})

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	assert.Equal(t, matchConnector(config, "IB1"), connectors[0].GraphNode)
	assert.Equal(t, "shared_iban", connectors[0].Metadata.Label)
	assert.Contains(t, resultNodes(result), node("users", "U2"))

	// Two links below the party, and on a different field on either side of the relation: the
	// route reads top down, from just under the party to the record carrying the value, and the
	// field is the one that record carries it on.
	for _, expected := range []struct {
		party string
		field string
	}{{"U1", "sender_iban"}, {"U2", "receiver_iban"}} {
		edge, ok := findEdge(result, node("users", expected.party), connectors[0].GraphNode)
		require.True(t, ok)
		assert.Equal(t, node("users", expected.party), edge.From)
		assert.Equal(t, []string{"accounts", "transactions"}, edge.Through)
		assert.Equal(t, expected.field, edge.Field)
	}
}

///////////////////////////////
// Entry point
///////////////////////////////

func TestWalkGraph_RejectsAnUnknownStartType(t *testing.T) {
	featureAccessReader := new(mocks.FeatureAccessReader)
	featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{GraphExploration: models.Allowed}, nil)

	enforceSecurity := new(mocks.EnforceSecurity)
	enforceSecurity.On("OrgId").Return(uuid.New())
	enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)

	dataModelRepository := new(mocks.DataModelRepository)
	dataModelRepository.On("GetDataModel", mock.Anything, mock.Anything, mock.Anything, false, true).
		Return(amlDataModel(), nil)

	uc := GraphWalkUsecase{
		enforceSecurity:         enforceSecurity,
		executorFactory:         executor_factory.NewExecutorFactoryStub(),
		featureAccessReader:     featureAccessReader,
		dataModelRepository:     dataModelRepository,
		graphRepository:         new(mocks.GraphRepository),
		graphRelationRepository: new(mocks.GraphRelationRepository),
	}

	_, err := uc.WalkGraph(context.Background(), "nope", "X1", models.GraphWalkOptions{})
	assert.ErrorIs(t, err, models.BadParameterError)
}

func TestWalkGraph_ClampsTheRequestedDegrees(t *testing.T) {
	// A chain long enough that an unclamped walk would keep going: each degree descends one
	// more level.
	rows := []fakeGraphRow{{"users", "U1", "object_id", "U1"}}
	for i := range 8 {
		rows = append(rows,
			fakeGraphRow{"accounts", fmt.Sprintf("A%d", i), "object_id", fmt.Sprintf("A%d", i)},
			fakeGraphRow{"accounts", fmt.Sprintf("A%d", i), "user_id", "U1"},
		)
	}

	featureAccessReader := new(mocks.FeatureAccessReader)
	featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{GraphExploration: models.Allowed}, nil)

	enforceSecurity := new(mocks.EnforceSecurity)
	enforceSecurity.On("OrgId").Return(uuid.New())
	enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)

	dataModelRepository := new(mocks.DataModelRepository)
	dataModelRepository.On("GetDataModel", mock.Anything, mock.Anything, mock.Anything, false, true).
		Return(amlDataModel(), nil)

	graphRelationRepository := new(mocks.GraphRelationRepository)
	graphRelationRepository.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, nil)

	uc := GraphWalkUsecase{
		enforceSecurity:         enforceSecurity,
		executorFactory:         executor_factory.NewExecutorFactoryStub(),
		featureAccessReader:     featureAccessReader,
		dataModelRepository:     dataModelRepository,
		graphRepository:         &fakeGraphRepository{rows: rows},
		graphRelationRepository: graphRelationRepository,
	}

	// Only that the call succeeds with an absurd request: the ceiling is enforced in the
	// usecase, so a caller bypassing the handler cannot ask for an unbounded walk.
	result, err := uc.WalkGraph(context.Background(), "users", "U1",
		models.GraphWalkOptions{Degrees: 10_000})
	require.NoError(t, err)
	assert.Equal(t, node("users", "U1"), result.Start)
}

func TestWalkGraph_LabelsRecordsWithTheirCaption(t *testing.T) {
	// users declares a caption field, accounts does not: a record of a table with nothing to be
	// called by is reported without a label rather than with an invented one.
	dataModel := amlDataModel()
	email := dataModel.Tables["users"].Fields["email"]
	email.Metadata = json.RawMessage(`{"hidden": false, "semanticSubType": "caption"}`)
	dataModel.Tables["users"].Fields["email"] = email

	rows := append(userWithAccount("U1", "A1", "IB1"), userWithAccount("U2", "A2", "IB1")...)

	featureAccessReader := new(mocks.FeatureAccessReader)
	featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{GraphExploration: models.Allowed}, nil)

	enforceSecurity := new(mocks.EnforceSecurity)
	enforceSecurity.On("OrgId").Return(uuid.New())
	enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)

	dataModelRepository := new(mocks.DataModelRepository)
	dataModelRepository.On("GetDataModel", mock.Anything, mock.Anything, mock.Anything, false, true).
		Return(dataModel, nil)

	graphRelationRepository := new(mocks.GraphRelationRepository)
	graphRelationRepository.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{sameIbanConfig()}, nil)

	uc := GraphWalkUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executor_factory.NewExecutorFactoryStub(),
		featureAccessReader: featureAccessReader,
		dataModelRepository: dataModelRepository,
		graphRepository: &fakeGraphRepository{
			rows:       rows,
			captions:   map[string]map[string]string{"users": {"U1": "a@b.c", "U2": "d@e.f"}},
			riskLevels: map[string]int{"U2": 3},
		},
		graphRelationRepository: graphRelationRepository,
	}

	result, err := uc.WalkGraph(context.Background(), "users", "U1",
		models.GraphWalkOptions{EndTypes: []string{"users", "accounts"}, Degrees: 1})
	require.NoError(t, err)

	nodes := resultNodes(result)

	require.Contains(t, nodes, node("users", "U1"))
	assert.Equal(t, "a@b.c", nodes[node("users", "U1")].Metadata.Label)
	assert.Equal(t, "d@e.f", nodes[node("users", "U2")].Metadata.Label)

	// Captions and scoring are two separate passes over the same nodes: neither may overwrite
	// what the other wrote.
	assert.Equal(t, 3, nodes[node("users", "U2")].Metadata.RiskLevel)
	assert.Zero(t, nodes[node("users", "U1")].Metadata.RiskLevel)

	assert.Empty(t, nodes[node("accounts", "A1")].Metadata.Label,
		"accounts declares no caption field")

	connectors := connectorNodes(result)
	require.Len(t, connectors, 1)
	assert.Equal(t, "same_iban", connectors[0].Metadata.Label,
		"a connector is not a record, so it carries no caption: its label is its relation group's")
}

func TestWalkGraph_StopsOnAForbiddenOrganization(t *testing.T) {
	featureAccessReader := new(mocks.FeatureAccessReader)
	featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{GraphExploration: models.Allowed}, nil)

	enforceSecurity := new(mocks.EnforceSecurity)
	enforceSecurity.On("OrgId").Return(uuid.New())
	enforceSecurity.On("ReadOrganization", mock.Anything).Return(errors.New("forbidden"))

	dataModelRepository := new(mocks.DataModelRepository)

	uc := GraphWalkUsecase{
		enforceSecurity:         enforceSecurity,
		executorFactory:         executor_factory.NewExecutorFactoryStub(),
		featureAccessReader:     featureAccessReader,
		dataModelRepository:     dataModelRepository,
		graphRepository:         new(mocks.GraphRepository),
		graphRelationRepository: new(mocks.GraphRelationRepository),
	}

	_, err := uc.WalkGraph(context.Background(), "users", "U1", models.GraphWalkOptions{})
	assert.Error(t, err)
	dataModelRepository.AssertNotCalled(t, "GetDataModel")
}
