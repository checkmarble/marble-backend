package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task 8 end-to-end scenarios. Only I/O is simulated: Marble DB, client DB,
// blob storage, case access and organization security. Configuration,
// selection, navigation, prefill, editing, validation, XML and export run
// unmodified, and the exported bytes are reread independently of the writer.

const (
	t8PP       = "/ds/personnes/pp"
	t8Civil    = t8PP + "/etatCivil"
	t8Card     = t8PP + "/referencesFinancieres/referenceFinanciereNumeroCarteBancaire"
	t8PM       = "/ds/personnes/pm"
	t8Activity = t8PM + "/activiteCoordonnee"
	t8Director = t8PM + "/dirigeant"
	t8PNC      = "/ds/personnes/pnc"
	t8PNCLink  = t8PNC + "/lienAutrePersonneDejaDeclaree"
	t8Op       = "/ds/operations/operation"
	t8Piece    = "/ds/pieceJointe/piece"
)

type task8Repo struct {
	*task6Repo
	org       string
	decisions map[string]models.TracfinDecisionSource
}

// Snapshots are only returned inside the scoped organization and case.
func (r *task8Repo) GetTracfinDecisionSources(_ context.Context, _ repositories.Executor, org, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	if org != r.org || caseID != "case" {
		return nil, nil
	}
	var out []models.TracfinDecisionSource
	for _, id := range ids {
		if d, ok := r.decisions[id]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *task8Repo) SaveTracfinConfiguration(_ context.Context, _ repositories.Executor, org string, c models.TracfinConfiguration, expected int64) error {
	current := int64(0)
	if r.configErr == nil {
		current = r.config.Revision
	}
	if org != r.org || expected != current {
		return models.ConflictError
	}
	c.OrgID, c.Revision = org, expected+1
	r.config, r.configErr = c, nil
	return nil
}

type task8Objects struct {
	tables  map[string]map[string]map[string]any
	queried []string
}

func (o *task8Objects) rows(table string) []map[string]any {
	ids := make([]string, 0, len(o.tables[table]))
	for id := range o.tables[table] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, o.tables[table][id])
	}
	return out
}

func (o *task8Objects) QueryIngestedObjectByUniqueField(_ context.Context, _ repositories.Executor, t models.Table, value, field string, _ ...string) ([]models.DataModelObject, error) {
	o.queried = append(o.queried, t.Name)
	var out []models.DataModelObject
	for _, d := range o.rows(t.Name) {
		if d[field] == value {
			out = append(out, models.DataModelObject{Data: d})
		}
	}
	return out, nil
}

func (o *task8Objects) ListIngestedObjects(_ context.Context, _ repositories.Executor, t models.Table, opts models.ExplorationOptions, cursor *string, limit int, _ ...string) ([]models.DataModelObject, error) {
	o.queried = append(o.queried, t.Name)
	var out []models.DataModelObject
	for _, d := range o.rows(t.Name) {
		if opts.FilterFieldValue.StringValue != nil && d[opts.FilterFieldName] == *opts.FilterFieldValue.StringValue && (cursor == nil || d["object_id"].(string) > *cursor) && len(out) < limit {
			out = append(out, models.DataModelObject{Data: d})
		}
	}
	return out, nil
}

type task8Cases struct {
	c      models.Case
	pivots []models.PivotObject
}

func (f task8Cases) GetCase(_ context.Context, id string) (models.Case, error) {
	if id != f.c.Id {
		return models.Case{}, models.NotFoundError
	}
	return f.c, nil
}

func (f task8Cases) ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error) {
	return f.pivots, nil
}

type task8Files struct{ byID map[string]models.CaseFile }

func (f *task8Files) GetCaseFileById(_ context.Context, _ repositories.Executor, id string) (models.CaseFile, error) {
	file, ok := f.byID[id]
	if !ok {
		return models.CaseFile{}, models.NotFoundError
	}
	return file, nil
}

type task8World struct {
	org     uuid.UUID
	ctx     context.Context
	repo    *task8Repo
	objects *task8Objects
	files   *task8Files
	blobs   *task6Blobs
	uc      TracfinUsecase
}

func task8NewWorld(dm models.DataModel, tables map[string]map[string]map[string]any, entities []models.CaseEntityRef, pivots []models.PivotObject) *task8World {
	org := uuid.New()
	repo := &task8Repo{task6Repo: &task6Repo{draftTestRepo: &draftTestRepo{configErr: models.NotFoundError}}, org: org.String(), decisions: map[string]models.TracfinDecisionSource{}}
	c := models.Case{Id: "case", OrganizationId: org, Outcome: models.CaseConfirmedRisk}
	for _, e := range entities {
		c.Entities = append(c.Entities, models.CaseEntity{CaseEntityRef: e})
	}
	w := &task8World{org: org, ctx: draftTestContext(org), repo: repo, objects: &task8Objects{tables: tables}, files: &task8Files{byID: map[string]models.CaseFile{}}, blobs: &task6Blobs{data: map[string][]byte{}}}
	w.uc = NewTracfinUsecase(org, repo, task8Cases{c, pivots}, tracfinDM{dm}, w.objects, tracfinExec{}, TracfinDraftDependencies{Repository: repo, Transactions: draftTestTransactions{}, Files: w.files, Blobs: w.blobs, Exports: repo, BucketURL: "exports"})
	w.uc.configurationDependencies = &TracfinConfigurationDependencies{Repository: repo, Organizations: &configOrgFake{}, Security: configSecurityFake{}}
	return w
}

func (w *task8World) configure(t *testing.T, settings []models.TracfinSettingInput, mappings []models.TracfinMapping) models.TracfinConfiguration {
	t.Helper()
	catalog, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	require.Empty(t, tracfin.ValidateConfiguration(models.TracfinConfiguration{Mappings: mappings}, catalog))
	expected := int64(0)
	if w.repo.configErr == nil {
		expected = w.repo.config.Revision
	}
	c, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{ExpectedRevision: expected, Settings: settings, Mappings: mappings})
	require.NoError(t, err)
	return c
}

func (w *task8World) update(t *testing.T, changes ...models.TracfinChangeInput) models.TracfinDraft {
	t.Helper()
	d, err := w.uc.UpdateDraft(w.ctx, "case", models.TracfinDraftInput{ExpectedRevision: w.repo.draft.Revision, Changes: changes})
	require.NoError(t, err)
	return d
}

func (w *task8World) prefill(t *testing.T) (tracfin.PrefillResult, map[string]tracfin.PrefillProposal) {
	t.Helper()
	p, err := w.uc.PrefillAtRevision(w.ctx, "case", w.repo.draft.Revision)
	require.NoError(t, err)
	byField := map[string]tracfin.PrefillProposal{}
	for _, proposal := range p.Proposals {
		key := proposal.Field.BlockID + " " + proposal.Field.FieldID
		_, duplicate := byField[key]
		require.False(t, duplicate, key)
		byField[key] = proposal
	}
	return p, byField
}

// export publishes the saved revision, then downloads and checks the exact bytes.
func (w *task8World) export(t *testing.T) (models.TracfinExport, []byte, *task8Node) {
	t.Helper()
	e, err := w.uc.Export(w.ctx, "case", w.repo.draft.Revision)
	require.NoError(t, err)
	require.Equal(t, w.repo.draft.Revision, e.DraftRevision)
	blob, info, err := w.uc.DownloadExport(w.ctx, "case", e.ID)
	require.NoError(t, err)
	data, err := io.ReadAll(blob.ReadCloser)
	require.NoError(t, err)
	require.NoError(t, blob.ReadCloser.Close())
	sum := sha256.Sum256(data)
	require.Equal(t, e.SHA256, hex.EncodeToString(sum[:]))
	require.Equal(t, e.Size, int64(len(data)))
	require.Equal(t, e.ID, info.ID)
	require.Equal(t, "unavailable", e.Validation.Layers["official_xsd"].Status)
	require.NotEmpty(t, e.Validation.Layers["official_xsd"].Reason)
	require.Equal(t, "passed", e.Validation.Layers["xml_syntax"].Status)
	return e, data, task8ParseXML(t, data)
}

func task8Add(parent, alias, catalogID string) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "add_block", ParentBlockID: parent, BlockID: alias, CatalogID: catalogID}
}

func task8Piece(parent, alias, fileID string) models.TracfinChangeInput {
	c := task8Add(parent, alias, t8Piece)
	c.SourceFileID = fileID
	return c
}

func task8Select(block string, s models.TracfinSelection) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "set_selection", BlockID: block, Selection: &s}
}

func task8Set(block, field string, v *models.TracfinFieldInput) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "set_field", Field: models.TracfinFieldRef{BlockID: block, FieldID: field}, Value: v}
}

func task8Relation(block, field, target, person string) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "set_relation", Field: models.TracfinFieldRef{BlockID: block, FieldID: field}, Relation: &models.TracfinReference{BlockID: target, PersonBlockID: person}}
}

func task8Apply(p tracfin.PrefillProposal, candidate int, version string) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "apply_prefill", ProposalID: p.ID, CandidateID: p.Candidates[candidate].ID, MappingVersion: version}
}

func task8Text(s string) *models.TracfinFieldInput {
	return &models.TracfinFieldInput{Kind: "text", Text: &s}
}

func task8Decimal(s string) *models.TracfinFieldInput {
	d := models.TracfinDecimal(s)
	return &models.TracfinFieldInput{Kind: "decimal", Decimal: &d}
}

func task8Setting(field, value string) models.TracfinSettingInput {
	return models.TracfinSettingInput{Field: models.TracfinFieldRef{FieldID: field}, Value: models.TracfinFieldInput{Kind: "text", Text: &value}}
}

func task8Mapping(id, target, entityType, table, field string, path ...models.TracfinPathStep) models.TracfinMapping {
	return models.TracfinMapping{ID: id, TargetField: target, Source: models.TracfinMappingSource{Kind: "ingested", EntityType: entityType, Table: table, Field: field, Path: path}}
}

func task8Snapshot(id, target, field string) models.TracfinMapping {
	return models.TracfinMapping{ID: id, TargetField: target, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Transfer", Field: field}}
}

func task8Blocks(d models.TracfinDraft, catalogID string) []models.TracfinBlock {
	var out []models.TracfinBlock
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == catalogID {
			out = append(out, b)
		}
	})
	return out
}

func task8Block(t *testing.T, d models.TracfinDraft, catalogID string) models.TracfinBlock {
	t.Helper()
	blocks := task8Blocks(d, catalogID)
	require.Len(t, blocks, 1, catalogID)
	return blocks[0]
}

func task8BlockByID(t *testing.T, d models.TracfinDraft, id string) models.TracfinBlock {
	t.Helper()
	var found *models.TracfinBlock
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.ID == id {
			found = &b
		}
	})
	require.NotNil(t, found, id)
	return *found
}

func task8Value(t *testing.T, d models.TracfinDraft, blockID, field string) models.TracfinValue {
	t.Helper()
	values := task8BlockByID(t, d, blockID).Fields[field]
	require.Len(t, values, 1, field)
	return values[0]
}

func task8Diag(ds []models.TracfinDiagnostic, code, blockID, fieldID string) bool {
	for _, d := range ds {
		if d.Code == code && (blockID == "" || d.Field != nil && d.Field.BlockID == blockID) && (fieldID == "" || d.Field != nil && d.Field.FieldID == fieldID) {
			return true
		}
	}
	return false
}

func task8Candidates(p tracfin.PrefillProposal) []string {
	var out []string
	for _, c := range p.Candidates {
		switch {
		case c.Text != nil:
			out = append(out, *c.Text)
		case c.Decimal != nil:
			out = append(out, string(*c.Decimal))
		}
	}
	return out
}

// task8Node is an independent reading of the exported XML, used instead of
// the writer's index to check values and references.
type task8Node struct {
	name     string
	attrs    map[string]string
	text     string
	children []*task8Node
}

func task8ParseXML(t *testing.T, data []byte) *task8Node {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []*task8Node
	var root *task8Node
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		switch v := token.(type) {
		case xml.StartElement:
			n := &task8Node{name: v.Name.Local, attrs: map[string]string{}}
			for _, a := range v.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(v)
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	require.NotNil(t, root)
	require.Equal(t, "ds", root.name)
	return root
}

func (n *task8Node) all(path string) []*task8Node {
	nodes := []*task8Node{n}
	for _, part := range strings.Split(path, "/") {
		var next []*task8Node
		for _, node := range nodes {
			for _, child := range node.children {
				if child.name == part {
					next = append(next, child)
				}
			}
		}
		nodes = next
	}
	return nodes
}

func (n *task8Node) one(t *testing.T, path string) *task8Node {
	t.Helper()
	nodes := n.all(path)
	require.Len(t, nodes, 1, path)
	return nodes[0]
}

func (n *task8Node) texts(path string) []string {
	var out []string
	for _, node := range n.all(path) {
		out = append(out, node.text)
	}
	return out
}

func task8IndependentXMLChecks(t *testing.T, data []byte) {
	t.Helper()
	catalog, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	reread := tracfin.ValidateXML(bytes.NewReader(data), catalog)
	for _, code := range []string{"INVALID_XML_REFERENCE", "DUPLICATE_PERSON_ID", "DUPLICATE_SUPPORT_ID", "DUPLICATE_PIECE_ID", "UNKNOWN_XML_ELEMENT", "UNKNOWN_XML_ATTRIBUTE", "XML_ELEMENT_ORDER", "XML_SYNTAX_ERROR"} {
		require.False(t, task8Diag(reread.Diagnostics, code, "", ""), code)
	}
}

func task8PersonCardModel() (models.DataModel, map[string]map[string]map[string]any) {
	dm := models.DataModel{Tables: map[string]models.Table{
		"User":    {Name: "User", SemanticType: models.SemanticTypePerson, NavigationOptions: []models.NavigationOption{{SourceFieldName: "object_id", TargetTableName: "Account", FilterFieldName: "owner_id", OrderingFieldName: "object_id", Status: models.IndexStatusValid}}},
		"Account": {Name: "Account", LinksToSingle: map[string]models.LinkToSingle{"owner": {Name: "owner", ParentTableName: "User", ParentFieldName: "object_id", ChildTableName: "Account", ChildFieldName: "owner_id"}}, NavigationOptions: []models.NavigationOption{{SourceFieldName: "object_id", TargetTableName: "Card", FilterFieldName: "account_id", OrderingFieldName: "object_id", Status: models.IndexStatusValid}}},
		"Card":    {Name: "Card", LinksToSingle: map[string]models.LinkToSingle{"account": {Name: "account", ParentTableName: "Account", ParentFieldName: "object_id", ChildTableName: "Card", ChildFieldName: "account_id"}}},
	}}
	tables := map[string]map[string]map[string]any{
		"User": {"U1": {"object_id": "U1", "last_name": "Martin", "birth_last_name": "Martin-Dupont", "first_name": "Claire", "birth_date": "14/07/1985", "gender": "F"}},
		"Account": {
			"A1": {"object_id": "A1", "owner_id": "U1"},
			"A2": {"object_id": "A2", "owner_id": "U1"},
		},
		"Card": {
			"C1": {"object_id": "C1", "account_id": "A1", "pan": "4970100000000001"},
			"C2": {"object_id": "C2", "account_id": "A2", "pan": "4970100000000002"},
			"C3": {"object_id": "C3", "account_id": "A2", "pan": "4970100000000003"},
		},
	}
	return dm, tables
}

var (
	t8Account = models.TracfinPathStep{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}
	t8CardHop = models.TracfinPathStep{ID: "card", ParentID: "account", Table: "Card", Link: "account", Direction: "inverse"}
)

func task8Bound(step models.TracfinPathStep, recordID string) models.TracfinPathStep {
	step.RecordID = recordID
	return step
}

func task8ObjectIDs(page TracfinNavigationPage) []string {
	var out []string
	for _, o := range page.Objects {
		out = append(out, o.Data["object_id"].(string))
	}
	return out
}

func TestTracfinEndToEndPersonAccountCardAndUningestedOperation(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, []models.PivotObject{{PivotType: models.PivotTypeObject, IsIngested: true, PivotObjectName: "User", PivotObjectId: "U1"}})
	// The historical "Transfer" type is not part of the current data model.
	w.repo.decisions["D1"] = models.TracfinDecisionSource{DecisionID: "D1", HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"object_id":"T-77","amount":10001,"currency":"EUR","kind":"card_payment","label":"Paiement <carte> & retrait"}`)}
	amount := task8Snapshot("op-amount", t8Op+"/montantTotal", "amount")
	amount.Unit, amount.TargetCurrency, amount.Currency = "minor", "EUR", models.TracfinCurrencyEvidence{Field: "currency"}
	count := task8Snapshot("op-count", t8Op+"/nombreOperations", "object_id")
	count.Source.UnitOperation = true
	kind := task8Snapshot("op-type", t8Op+"/typeOperation", "kind")
	kind.Codes = []models.TracfinCodeMapping{{Source: "card_payment", Target: "OT03"}}
	sex := task8Mapping("pp-sex", t8Civil+"/sexe", "Person", "User", "gender")
	sex.Codes = []models.TracfinCodeMapping{{Source: "F", Target: "SEX2"}, {Source: "M", Target: "SEX1"}}
	config := w.configure(t, []models.TracfinSettingInput{
		task8Setting("/ds/organisme/libelle", "Banque Exemple"),
		task8Setting("/ds/organisme/profession", "TPI1"),
		task8Setting("/ds/organisme/adresse/nomVoie", "rue de la Paix"),
		task8Setting("/ds/organisme/adresse/codePostal", "75002"),
		task8Setting("/ds/organisme/adresse/ville", "Paris"),
		task8Setting("/ds/organisme/adresse/pays", "FR"),
	}, []models.TracfinMapping{
		task8Mapping("pp-name", t8Civil+"/nomNaissance", "Person", "User", "last_name"),
		task8Mapping("pp-first", t8Civil+"/prenom", "Person", "User", "first_name"),
		task8Mapping("pp-birth", t8Civil+"/dateNaissance", "Person", "User", "birth_date"),
		sex,
		task8Mapping("card-pan", t8Card+"/@numeroCarteBancaire", "Person", "User", "pan", t8Account, t8CardHop),
		amount, count, kind,
		task8Snapshot("op-comment", t8Op+"/commentaires", "label"),
	})
	require.Equal(t, int64(1), config.Revision)

	d, err := w.uc.CreateDraft(w.ctx, "case", true)
	require.NoError(t, err)
	require.False(t, task8Diag(d.Diagnostics, "CONFIGURATION_UNAVAILABLE", "", ""))
	require.Equal(t, config.Revision, d.Configuration.Revision)
	organisme := task8Block(t, d, "/ds/organisme")
	require.Equal(t, "settings", organisme.Fields["/ds/organisme/libelle"][0].Origin.Source)
	root := d.Document.Root.ID

	ctxRoots, err := w.uc.GetContext(w.ctx, "case")
	require.NoError(t, err)
	require.Equal(t, []TracfinRoot{{Table: "User", RecordID: "U1", Origins: []string{"case_manual_entity", "decision_pivot"}}}, ctxRoots.Roots)

	// User U1 has two accounts; each navigation page is bounded and resumable.
	userSelection := models.TracfinSelection{MappingID: "card-pan", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}}
	page, err := w.uc.Navigate(w.ctx, "case", userSelection, "account", nil, 1)
	require.NoError(t, err)
	require.Equal(t, []string{"A1"}, task8ObjectIDs(page))
	require.True(t, page.HasNextPage)
	page, err = w.uc.Navigate(w.ctx, "case", userSelection, "account", page.NextCursor, 1)
	require.NoError(t, err)
	require.Equal(t, []string{"A2"}, task8ObjectIDs(page))
	require.False(t, page.HasNextPage)
	accountSelection := userSelection
	accountSelection.Bindings = []models.TracfinPathStep{task8Bound(t8Account, "A2")}
	page, err = w.uc.Navigate(w.ctx, "case", accountSelection, "card", nil, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"C2", "C3"}, task8ObjectIDs(page), "only cards of the selected account")
	cardSelection := accountSelection
	cardSelection.Bindings = append(cardSelection.Bindings, task8Bound(t8CardHop, "C3"))

	d = w.update(t,
		task8Add(root, "people", "/ds/personnes"),
		task8Add("people", "pp", t8PP),
		task8Add("pp", "civil", t8Civil),
		task8Select("civil", models.TracfinSelection{MappingID: "pp-name", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}}),
		task8Add("pp", "refs", t8PP+"/referencesFinancieres"),
		task8Add("refs", "card", t8Card),
		task8Select("card", cardSelection),
		task8Add(root, "ops", "/ds/operations"),
		task8Add("ops", "op", t8Op),
		task8Select("op", models.TracfinSelection{MappingID: "op-amount", DecisionID: "D1", HistoricalType: "Transfer"}),
	)
	pp, civil, card, op := task8Block(t, d, t8PP).ID, task8Block(t, d, t8Civil).ID, task8Block(t, d, t8Card).ID, task8Block(t, d, t8Op).ID
	require.Equal(t, cardSelection, *task8BlockByID(t, d, card).Selection)
	// A manual correction entered before prefill is never replaced by a source value.
	d = w.update(t, task8Set(civil, t8Civil+"/prenom", task8Text("Claire-Marie")))

	result, proposals := w.prefill(t)
	require.Equal(t, d.Revision, result.DraftRevision)
	require.NotContains(t, proposals, civil+" "+t8Civil+"/prenom")
	for key, want := range map[string]string{
		civil + " " + t8Civil + "/nomNaissance":       "Martin",
		civil + " " + t8Civil + "/dateNaissance":      "14/07/1985",
		civil + " " + t8Civil + "/sexe":               "SEX2",
		card + " " + t8Card + "/@numeroCarteBancaire": "4970100000000003",
		op + " " + t8Op + "/montantTotal":             "100.01",
		op + " " + t8Op + "/nombreOperations":         "1",
		op + " " + t8Op + "/typeOperation":            "OT03",
		op + " " + t8Op + "/commentaires":             "Paiement <carte> & retrait",
	} {
		require.Contains(t, proposals, key)
		require.Equal(t, []string{want}, task8Candidates(proposals[key]), key)
		require.Equal(t, "resolved", proposals[key].Resolution)
	}
	cardOrigin := proposals[card+" "+t8Card+"/@numeroCarteBancaire"].Candidates[0].Origin
	require.Equal(t, "ingested", cardOrigin.Source)
	require.Equal(t, []models.TracfinPathStep{userSelection.Root, task8Bound(t8Account, "A2"), task8Bound(t8CardHop, "C3")}, cardOrigin.Sources[0].Path)
	amountOrigin := proposals[op+" "+t8Op+"/montantTotal"].Candidates[0].Origin
	require.Equal(t, models.TracfinSourceRef{Kind: "decision_snapshot", Field: "amount", DecisionID: "D1", HistoricalType: "Transfer"}, amountOrigin.Sources[0])
	require.NotContains(t, w.objects.queried, "Transfer", "a decision snapshot must not require ingestion")

	var apply []models.TracfinChangeInput
	for _, p := range result.Proposals {
		apply = append(apply, task8Apply(p, 0, result.MappingVersion))
	}
	d = w.update(t, apply...)
	_, remaining := w.prefill(t)
	require.Empty(t, remaining, "applied proposals stay resolved")

	declarant := task8Block(t, d, "/ds/declarant/coordonnees").ID
	d = w.update(t,
		task8Set(pp, t8PP+"/implication", task8Text("Personne soupçonnée / Personne impliquée")),
		task8Relation(op, t8Op+"/emetteurTypeEmetteur", pp, ""),
		task8Relation(op, t8Op+"/refFinanciereEmission", card, pp),
		task8Set(op, t8Op+"/beneficiaireTypeBeneficiaire", task8Text("Commerce 75011")),
		task8Set(task8Block(t, d, "/ds/operations").ID, "/ds/operations/montantTotal", task8Decimal("150.00")),
		task8Set(declarant, "/ds/declarant/coordonnees/tel", task8Text("0102030405")),
		task8Set(root, "/ds/analyse", task8Text("Paiements carte incohérents avec le profil.")),
	)
	require.Equal(t, "manual", task8Value(t, d, civil, t8Civil+"/prenom").Origin.Source)
	stored := task8Value(t, d, civil, t8Civil+"/nomNaissance")
	require.Equal(t, "ingested", stored.Origin.Source)
	require.Equal(t, result.MappingVersion, stored.Origin.MappingVersion)
	require.Equal(t, "D1", task8Value(t, d, op, t8Op+"/montantTotal").Origin.Sources[0].DecisionID)

	e, data, x := w.export(t)
	require.Equal(t, d.Revision, e.DraftRevision)
	task8IndependentXMLChecks(t, data)
	person := x.one(t, "personnes/pp")
	require.Equal(t, []string{"Martin"}, person.texts("etatCivil/nomNaissance"))
	require.Equal(t, []string{"Claire-Marie"}, person.texts("etatCivil/prenom"))
	require.Equal(t, []string{"SEX2"}, person.texts("etatCivil/sexe"))
	support := person.one(t, "referencesFinancieres/referenceFinanciereNumeroCarteBancaire")
	require.Equal(t, "4970100000000003", support.attrs["numeroCarteBancaire"])
	for _, other := range []string{"4970100000000001", "4970100000000002"} {
		require.NotContains(t, string(data), other, "cards of other accounts are never exported")
	}
	line := x.one(t, "operations/operation")
	require.Equal(t, []string{"100.01"}, line.texts("montantTotal"))
	require.Equal(t, []string{"1"}, line.texts("nombreOperations"))
	require.Equal(t, []string{"OT03"}, line.texts("typeOperation"))
	require.Equal(t, []string{"Paiement <carte> & retrait"}, line.texts("commentaires"))
	require.Contains(t, string(data), "Paiement &lt;carte&gt; &amp; retrait")
	require.Equal(t, person.texts("id"), line.texts("emetteurTypeEmetteur"))
	require.Equal(t, []string{support.attrs["id"]}, line.texts("refFinanciereEmission"))
	require.Equal(t, []string{"Commerce 75011"}, line.texts("beneficiaireTypeBeneficiaire"))
	require.Equal(t, []string{"150.00"}, x.texts("operations/montantTotal"), "global amount is entered, not computed")
	require.Equal(t, []string{"Ada"}, x.texts("declarant/coordonnees/prenom"))
	require.Equal(t, []string{"Banque Exemple"}, x.texts("organisme/libelle"))
	require.Equal(t, []string{"Paris"}, x.texts("organisme/adresse/ville"))
	// Missing business data is reported, not invented.
	require.True(t, task8Diag(e.Validation.Diagnostics, "MISSING_REQUIRED_VALUE", "", "/ds/context"))
	require.Empty(t, x.all("context"))
	require.False(t, task8Diag(e.Validation.Diagnostics, "MISSING_REQUIRED_VALUE", op, t8Op+"/montantTotal"))
	// Exporting changes neither the draft nor the case.
	require.Equal(t, e.DraftRevision, w.repo.draft.Revision)
	require.Equal(t, e, *w.repo.draft.LastExport)
}

func TestTracfinEndToEndCompanyLinkedPeopleConflictsCodesAndPieces(t *testing.T) {
	dm := models.DataModel{Tables: map[string]models.Table{
		"Company":   {Name: "Company", SemanticType: models.SemanticTypeCompany, LinksToSingle: map[string]models.LinkToSingle{"kyb": {Name: "kyb", ChildTableName: "Company", ChildFieldName: "kyb_id", ParentTableName: "KybReport", ParentFieldName: "object_id"}}},
		"KybReport": {Name: "KybReport"},
		"Person":    {Name: "Person", SemanticType: models.SemanticTypePerson},
	}}
	tables := map[string]map[string]map[string]any{
		"Company":   {"CO1": {"object_id": "CO1", "name": "ACME SAS", "siren": "552100554", "legal_form": "SAS", "kyb_id": "K1"}},
		"KybReport": {"K1": {"object_id": "K1", "legal_name": "ACME Holding", "siren": "552100554"}},
		"Person":    {"U2": {"object_id": "U2", "last_name": "Durand", "first_name": "Paul", "gender": "X"}},
	}
	w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "Company", ObjectId: "CO1"}}, []models.PivotObject{{PivotType: models.PivotTypeObject, IsIngested: true, PivotObjectName: "Person", PivotObjectId: "U2"}})
	// Two distinct decisions recorded the same historical transfer with different contents.
	w.repo.decisions["D2"] = models.TracfinDecisionSource{DecisionID: "D2", HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"object_id":"T-9","amount":2500.50,"currency":"EUR","kind":"transfer"}`)}
	w.repo.decisions["D3"] = models.TracfinDecisionSource{DecisionID: "D3", HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"object_id":"T-9","amount":1200.00,"currency":"EUR","kind":"transfer"}`)}
	kbis := []byte("%PDF-1.7 extrait kbis ACME\n")
	passport := []byte{0xff, 0xd8, 0xff, 0x00, 0x01, 'p', 'a', 's', 's', 0x00, 0xfe}
	w.files.byID["F1"] = models.CaseFile{Id: "F1", CaseId: "case", BucketName: "case-files", FileReference: "kbis-key", FileName: "kbis.pdf"}
	w.files.byID["F2"] = models.CaseFile{Id: "F2", CaseId: "case", BucketName: "case-files", FileReference: "passport-key", FileName: "passeport.jpg"}
	w.files.byID["F3"] = models.CaseFile{Id: "F3", CaseId: "other-case", BucketName: "case-files", FileReference: "other-key", FileName: "other.pdf"}
	w.blobs.data["kbis-key"], w.blobs.data["passport-key"], w.blobs.data["other-key"] = kbis, passport, []byte("other case")

	kyb := models.TracfinPathStep{ID: "kyb", ParentID: "root", Table: "KybReport", Link: "kyb", Direction: "forward"}
	form := task8Mapping("pm-form", t8Activity+"/formeJuridique", "Company", "Company", "legal_form")
	form.Codes = []models.TracfinCodeMapping{{Source: "SAS", Target: "FJ3"}}
	sex := task8Mapping("pp-sex", t8Civil+"/sexe", "Person", "Person", "gender")
	sex.Codes = []models.TracfinCodeMapping{{Source: "M", Target: "SEX1"}, {Source: "F", Target: "SEX2"}}
	amount := task8Snapshot("op-amount", t8Op+"/montantTotal", "amount")
	amount.Unit, amount.TargetCurrency, amount.Currency = "major", "EUR", models.TracfinCurrencyEvidence{Field: "currency"}
	kind := task8Snapshot("op-type", t8Op+"/typeOperation", "kind")
	kind.Codes = []models.TracfinCodeMapping{{Source: "transfer", Target: "OT10"}}
	mappings := []models.TracfinMapping{
		task8Mapping("pm-name", t8Activity+"/denominationSociale", "Company", "Company", "name"),
		task8Mapping("pm-name-kyb", t8Activity+"/denominationSociale", "Company", "Company", "legal_name", kyb),
		task8Mapping("pm-siren", t8Activity+"/numeroIdentification", "Company", "Company", "siren"),
		task8Mapping("pm-siren-kyb", t8Activity+"/numeroIdentification", "Company", "Company", "siren", kyb),
		form,
		task8Mapping("pp-name", t8Civil+"/nomNaissance", "Person", "Person", "last_name"),
		task8Mapping("pp-first", t8Civil+"/prenom", "Person", "Person", "first_name"),
		sex, amount, kind,
	}
	w.configure(t, nil, mappings)
	d, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	require.Empty(t, task8Blocks(d, "/ds/declarant/coordonnees"), "declarant identity is not prefilled when the user is not the declarant")
	root := d.Document.Root.ID

	before := w.repo.draft.Revision
	_, err = w.uc.UpdateDraft(w.ctx, "case", models.TracfinDraftInput{ExpectedRevision: before, Changes: []models.TracfinChangeInput{task8Add(root, "pieces", "/ds/pieceJointe"), task8Piece("pieces", "foreign", "F3")}})
	require.ErrorIs(t, err, models.ForbiddenError, "a file of another case is refused")
	require.Equal(t, before, w.repo.draft.Revision)

	d = w.update(t,
		task8Add(root, "people", "/ds/personnes"),
		task8Add("people", "pm", t8PM),
		task8Add("pm", "activity", t8Activity),
		task8Select("activity", models.TracfinSelection{MappingID: "pm-name", Root: models.TracfinPathStep{ID: "root", Table: "Company", RecordID: "CO1"}, Bindings: []models.TracfinPathStep{task8Bound(kyb, "K1")}}),
		task8Add("people", "pp", t8PP),
		task8Add("pp", "civil", t8Civil),
		task8Select("civil", models.TracfinSelection{MappingID: "pp-name", Root: models.TracfinPathStep{ID: "root", Table: "Person", RecordID: "U2"}}),
		task8Add("pm", "director", t8Director),
		task8Add("people", "pnc", t8PNC),
		task8Add("pnc", "pnc-link", t8PNCLink),
		task8Add("pnc-link", "pnc-relation", t8PNCLink+"/typeRelation"),
		task8Add(root, "ops", "/ds/operations"),
		task8Add("ops", "op1", t8Op),
		task8Select("op1", models.TracfinSelection{MappingID: "op-amount", DecisionID: "D2", HistoricalType: "Transfer"}),
		task8Add("ops", "op2", t8Op),
		task8Select("op2", models.TracfinSelection{MappingID: "op-amount", DecisionID: "D3", HistoricalType: "Transfer"}),
		task8Add(root, "pieces", "/ds/pieceJointe"),
		task8Piece("pieces", "kbis", "F1"),
		task8Piece("pieces", "passport", "F2"),
		task8Add("passport", "passport-id", t8Piece+"/infoPieceIdentite"),
	)
	pm, activity, pp, civil := task8Block(t, d, t8PM).ID, task8Block(t, d, t8Activity).ID, task8Block(t, d, t8PP).ID, task8Block(t, d, t8Civil).ID
	director, pnc, pncLink, pncRelation := task8Block(t, d, t8Director).ID, task8Block(t, d, t8PNC).ID, task8Block(t, d, t8PNCLink).ID, task8Block(t, d, t8PNCLink+"/typeRelation").ID
	ops := task8Blocks(d, t8Op)
	require.Len(t, ops, 2)
	op1, op2 := ops[0].ID, ops[1].ID
	pieces := map[string]string{}
	for _, b := range task8Blocks(d, t8Piece) {
		require.NotNil(t, b.Attachment)
		pieces[b.Attachment.SourceFileID] = b.ID
		require.Equal(t, "case", b.Attachment.CaseID)
	}
	sum := sha256.Sum256(passport)
	require.Equal(t, hex.EncodeToString(sum[:]), task8BlockByID(t, d, pieces["F2"]).Attachment.SHA256)
	passportID := task8Block(t, d, t8Piece+"/infoPieceIdentite").ID

	result, proposals := w.prefill(t)
	name := proposals[activity+" "+t8Activity+"/denominationSociale"]
	require.Equal(t, "conflict", name.Resolution)
	require.ElementsMatch(t, []string{"ACME SAS", "ACME Holding"}, task8Candidates(name))
	siren := proposals[activity+" "+t8Activity+"/numeroIdentification"]
	require.Equal(t, "resolved", siren.Resolution, "equal sources do not create a false conflict")
	require.Len(t, siren.Candidates, 1)
	require.Len(t, siren.Candidates[0].Origin.Sources, 2)
	require.Equal(t, []string{"FJ3"}, task8Candidates(proposals[activity+" "+t8Activity+"/formeJuridique"]))
	require.NotContains(t, proposals, civil+" "+t8Civil+"/sexe")
	require.True(t, task8Diag(result.Diagnostics, "UNKNOWN_CODE", civil, t8Civil+"/sexe"), "an unmapped client value is visible")
	require.Equal(t, []string{"2500.50"}, task8Candidates(proposals[op1+" "+t8Op+"/montantTotal"]))
	require.Equal(t, []string{"1200.00"}, task8Candidates(proposals[op2+" "+t8Op+"/montantTotal"]))
	require.True(t, task8Diag(result.Diagnostics, "POTENTIAL_DUPLICATE_OPERATION", op1, ""))
	require.True(t, task8Diag(result.Diagnostics, "POTENTIAL_DUPLICATE_OPERATION", op2, ""))

	// The analyst resolves the conflict by choosing the KYB source; every other
	// unambiguous proposal is applied in the same atomic batch.
	var apply []models.TracfinChangeInput
	for _, p := range result.Proposals {
		choice := 0
		if p.ID == name.ID {
			for i, c := range p.Candidates {
				if *c.Text == "ACME Holding" {
					choice = i
				}
			}
		}
		apply = append(apply, task8Apply(p, choice, result.MappingVersion))
	}
	d = w.update(t, apply...)
	chosen := task8Value(t, d, activity, t8Activity+"/denominationSociale")
	require.Equal(t, "resolved", chosen.Origin.Resolution)
	require.Equal(t, "KybReport", chosen.Origin.Sources[0].Table)

	set := func(block, field, value string) models.TracfinChangeInput {
		return task8Set(block, field, task8Text(value))
	}
	implicated := "Personne soupçonnée / Personne impliquée"
	d = w.update(t,
		set(civil, t8Civil+"/sexe", "SEX1"),
		set(pp, t8PP+"/implication", implicated),
		set(pm, t8PM+"/implication", implicated),
		set(director, t8Director+"/fonctionDirigeant", "MF01"),
		task8Relation(director, t8Director+"/idPersonne", pp, ""),
		set(pnc, t8PNC+"/implication", "Je ne sais pas"),
		set(pnc, t8PNC+"/elementsIdentification", "Personne vue au guichet, identité inconnue"),
		task8Relation(pncLink, t8PNCLink+"/idPersonne", pm, ""),
		set(pncRelation, t8PNCLink+"/typeRelation/categorieRelation", "TLP57"),
		task8Relation(op1, t8Op+"/emetteurTypeEmetteur", pp, ""),
		task8Relation(op1, t8Op+"/beneficiaireTypeBeneficiaire", pm, ""),
		task8Relation(op2, t8Op+"/emetteurTypeEmetteur", pm, ""),
		set(op2, t8Op+"/beneficiaireTypeBeneficiaire", "Bénéficiaire 0042"),
		task8Relation(civil, t8Civil+"/idPieceJointe", pieces["F2"], ""),
		task8Relation(activity, t8Activity+"/idPieceJointe", pieces["F1"], ""),
		set(pieces["F1"], t8Piece+"/type", "TA6"),
		set(pieces["F1"], t8Piece+"/date", "02/09/2026"),
		set(pieces["F1"], t8Piece+"/libelle", "Extrait Kbis"),
		set(pieces["F1"], t8Piece+"/type_mime", "application/pdf"),
		set(pieces["F2"], t8Piece+"/type", "TA3"),
		set(pieces["F2"], t8Piece+"/date", "15/03/2021"),
		set(pieces["F2"], t8Piece+"/libelle", "Passeport"),
		set(pieces["F2"], t8Piece+"/type_mime", "image/jpeg"),
		set(passportID, t8Piece+"/infoPieceIdentite/numPieceIdentite", "19AB12345"),
		set(passportID, t8Piece+"/infoPieceIdentite/paysDelivrance", "FR"),
		set(passportID, t8Piece+"/infoPieceIdentite/dateValidite", "14/03/2031"),
		task8Set(task8Block(t, d, "/ds/operations").ID, "/ds/operations/montantTotal", task8Decimal("5000.00")),
	)
	// The draft-local code choice does not change the organization mapping.
	require.Equal(t, sex.Codes, w.repo.config.Mappings[7].Codes)

	e, data, x := w.export(t)
	task8IndependentXMLChecks(t, data)
	company := x.one(t, "personnes/pm")
	require.Equal(t, []string{"ACME Holding"}, company.texts("activiteCoordonnee/denominationSociale"))
	require.NotContains(t, string(data), "ACME SAS", "the rejected candidate is not exported")
	require.Equal(t, []string{"552100554"}, company.texts("activiteCoordonnee/numeroIdentification"))
	require.Equal(t, []string{"FJ3"}, company.texts("activiteCoordonnee/formeJuridique"))
	person := x.one(t, "personnes/pp")
	unknown := x.one(t, "personnes/pnc")
	require.Equal(t, []string{"Durand"}, person.texts("etatCivil/nomNaissance"))
	require.Equal(t, []string{"SEX1"}, person.texts("etatCivil/sexe"))
	ppID, pmID := person.one(t, "id").text, company.one(t, "id").text
	require.ElementsMatch(t, []string{"1", "2", "3"}, []string{ppID, pmID, unknown.one(t, "id").text})
	// The same identities are reused in several roles without duplicating persons.
	require.Equal(t, []string{ppID}, company.texts("dirigeant/idPersonne"))
	require.Equal(t, []string{"MF01"}, company.texts("dirigeant/fonctionDirigeant"))
	require.Equal(t, []string{pmID}, unknown.texts("lienAutrePersonneDejaDeclaree/idPersonne"))
	require.Equal(t, []string{"Personne vue au guichet, identité inconnue"}, unknown.texts("elementsIdentification"))
	lines := x.all("operations/operation")
	require.Len(t, lines, 2)
	require.Equal(t, []string{"2500.50"}, lines[0].texts("montantTotal"))
	require.Equal(t, []string{ppID}, lines[0].texts("emetteurTypeEmetteur"))
	require.Equal(t, []string{pmID}, lines[0].texts("beneficiaireTypeBeneficiaire"))
	require.Equal(t, []string{"1200.00"}, lines[1].texts("montantTotal"))
	require.Equal(t, []string{pmID}, lines[1].texts("emetteurTypeEmetteur"))
	require.Equal(t, []string{"Bénéficiaire 0042"}, lines[1].texts("beneficiaireTypeBeneficiaire"))
	require.Equal(t, []string{"5000.00"}, x.texts("operations/montantTotal"))
	require.NotContains(t, string(data), "3700.5", "line amounts are never summed")
	pieceIDs := map[string]string{}
	for _, piece := range x.all("pieceJointe/piece") {
		content, err := base64.StdEncoding.DecodeString(piece.one(t, "contenu").text)
		require.NoError(t, err)
		switch {
		case bytes.Equal(content, kbis):
			pieceIDs["F1"] = piece.attrs["id"]
			require.Equal(t, []string{"TA6"}, piece.texts("type"))
		case bytes.Equal(content, passport):
			pieceIDs["F2"] = piece.attrs["id"]
			require.Equal(t, []string{"19AB12345"}, piece.texts("infoPieceIdentite/numPieceIdentite"))
		default:
			t.Fatalf("unexpected piece content %q", content)
		}
	}
	require.Len(t, pieceIDs, 2)
	require.Equal(t, []string{pieceIDs["F2"]}, person.texts("etatCivil/idPieceJointe"))
	require.Equal(t, []string{pieceIDs["F1"]}, company.texts("activiteCoordonnee/idPieceJointe"))
	require.NotContains(t, string(data), base64.StdEncoding.EncodeToString([]byte("other case")))
	// The MIME catalogue is not among the retained sources; the gap stays visible.
	require.True(t, task8Diag(e.Validation.Diagnostics, "CODE_CATALOG_UNAVAILABLE", pieces["F1"], t8Piece+"/type_mime"))

	// A later organization mapping change does not alter the draft silently.
	w.configure(t, nil, append(append([]models.TracfinMapping{}, mappings[0]), mappings[2:]...))
	require.Equal(t, int64(2), w.repo.config.Revision)
	stale, err := w.uc.GetDraft(w.ctx, "case")
	require.NoError(t, err)
	require.Equal(t, int64(1), stale.Configuration.Revision)
	_, proposals = w.prefill(t)
	require.Empty(t, proposals)
	d = w.update(t, models.TracfinChangeInput{Action: "refresh_configuration"})
	require.Equal(t, int64(2), d.Configuration.Revision)
	kept := task8Value(t, d, activity, t8Activity+"/denominationSociale")
	require.Equal(t, "ACME Holding", *kept.Text)
	require.True(t, kept.NeedsReview)
	manual := task8Value(t, d, civil, t8Civil+"/sexe")
	require.Equal(t, "SEX1", *manual.Text)
	require.Equal(t, "manual", manual.Origin.Source)
	_, proposals = w.prefill(t)
	require.Equal(t, []string{"ACME SAS"}, task8Candidates(proposals[activity+" "+t8Activity+"/denominationSociale"]), "a new proposal requires an explicit choice")
	require.NotContains(t, proposals, civil+" "+t8Civil+"/sexe")
	// The published artifact keeps its bytes and revision.
	blob, info, err := w.uc.DownloadExport(w.ctx, "case", e.ID)
	require.NoError(t, err)
	again, err := io.ReadAll(blob.ReadCloser)
	require.NoError(t, err)
	require.Equal(t, data, again)
	require.Less(t, info.DraftRevision, w.repo.draft.Revision)
}

func TestTracfinEndToEndPartialDraftStillExports(t *testing.T) {
	t.Run("no configuration", func(t *testing.T) {
		dm, tables := task8PersonCardModel()
		w := task8NewWorld(dm, tables, nil, nil)
		d, err := w.uc.CreateDraft(w.ctx, "case", false)
		require.NoError(t, err)
		require.True(t, task8Diag(d.Diagnostics, "CONFIGURATION_UNAVAILABLE", "", ""))
		require.Empty(t, d.Configuration.Mappings)
		analysis := "Montant < 10 000 € & flux \"incohérents\""
		d = w.update(t, task8Set(d.Document.Root.ID, "/ds/analyse", task8Text(analysis)))
		result, _ := w.prefill(t)
		require.Empty(t, result.Proposals)
		e, data, x := w.export(t)
		require.Equal(t, []string{analysis}, x.texts("analyse"))
		require.Contains(t, string(data), "Montant &lt; 10 000 € &amp; flux")
		for _, field := range []string{"/ds/organisme", "/ds/declarant", "/ds/context"} {
			require.True(t, task8Diag(e.Validation.Diagnostics, "MISSING_REQUIRED_VALUE", "", field), field)
		}
		require.Equal(t, "failed", e.Validation.Layers["internal_structure"].Status)
		kept, err := w.uc.GetDraft(w.ctx, "case")
		require.NoError(t, err)
		require.Equal(t, analysis, *task8Value(t, kept, kept.Document.Root.ID, "/ds/analyse").Text)
	})

	t.Run("incomplete configuration and lost sources", func(t *testing.T) {
		dm, tables := task8PersonCardModel()
		w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, nil)
		w.files.byID["F1"] = models.CaseFile{Id: "F1", CaseId: "case", BucketName: "case-files", FileReference: "statement-key", FileName: "releve.pdf"}
		w.blobs.data["statement-key"] = []byte("relevé")
		// Only two competing name sources and the card number are configured.
		w.configure(t, nil, []models.TracfinMapping{
			task8Mapping("pp-name", t8Civil+"/nomNaissance", "Person", "User", "last_name"),
			task8Mapping("pp-birth-name", t8Civil+"/nomNaissance", "Person", "User", "birth_last_name"),
			task8Mapping("card-pan", t8Card+"/@numeroCarteBancaire", "Person", "User", "pan", t8Account, t8CardHop),
		})
		d, err := w.uc.CreateDraft(w.ctx, "case", false)
		require.NoError(t, err)
		root := d.Document.Root.ID
		userRoot := models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}
		d = w.update(t,
			task8Add(root, "people", "/ds/personnes"),
			task8Add("people", "pp", t8PP),
			task8Add("pp", "civil", t8Civil),
			task8Select("civil", models.TracfinSelection{MappingID: "pp-name", Root: userRoot}),
			task8Add("pp", "refs", t8PP+"/referencesFinancieres"),
			task8Add("refs", "card", t8Card),
			task8Select("card", models.TracfinSelection{MappingID: "card-pan", Root: userRoot, Bindings: []models.TracfinPathStep{task8Bound(t8Account, "A1"), task8Bound(t8CardHop, "C1")}}),
			task8Add("people", "pp2", t8PP),
			task8Add(root, "ops", "/ds/operations"),
			task8Add("ops", "op", t8Op),
			task8Add(root, "pieces", "/ds/pieceJointe"),
			task8Piece("pieces", "piece", "F1"),
		)
		civil, card, op := task8Block(t, d, t8Civil).ID, task8Block(t, d, t8Card).ID, task8Block(t, d, t8Op).ID
		people := task8Blocks(d, t8PP)
		require.Len(t, people, 2)
		second := people[1].ID
		// A fully manual operation line, with a business value left invalid.
		d = w.update(t,
			task8Relation(op, t8Op+"/emetteurTypeEmetteur", second, ""),
			task8Set(op, t8Op+"/montantTotal", task8Decimal("42.00")),
			task8Set(op, t8Op+"/typeOperation", task8Text("OT11")),
			task8Set(op, t8Op+"/commentaires", task8Text("Saisie manuelle")),
		)
		// The emitter is removed, the card is no longer ingested and the case file is deleted.
		d = w.update(t, models.TracfinChangeInput{Action: "remove_block", BlockID: second})
		delete(w.objects.tables["Card"], "C1")
		delete(w.files.byID, "F1")

		result, proposals := w.prefill(t)
		name := proposals[civil+" "+t8Civil+"/nomNaissance"]
		require.Equal(t, "conflict", name.Resolution)
		require.ElementsMatch(t, []string{"Martin", "Martin-Dupont"}, task8Candidates(name))
		require.True(t, task8Diag(result.Diagnostics, "SOURCE_UNAVAILABLE", card, ""))
		require.NotContains(t, proposals, card+" "+t8Card+"/@numeroCarteBancaire")
		require.Equal(t, models.TracfinSelection{MappingID: "card-pan", Root: userRoot, Bindings: []models.TracfinPathStep{task8Bound(t8Account, "A1"), task8Bound(t8CardHop, "C1")}}, *task8BlockByID(t, w.repo.draft, card).Selection, "a lost source keeps its selection for review")

		e, data, x := w.export(t)
		require.Empty(t, x.all("personnes/pp/etatCivil/nomNaissance"), "an unresolved conflict exports no arbitrary value")
		require.NotContains(t, string(data), "Martin")
		require.Empty(t, x.all("operations/operation/emetteurTypeEmetteur"))
		require.True(t, task8Diag(e.Validation.Diagnostics, "DANGLING_REFERENCE_NOT_EXPORTED", op, t8Op+"/emetteurTypeEmetteur"))
		require.True(t, task8Diag(e.Validation.Diagnostics, "OTHER_OPERATION_DESCRIPTION_REQUIRED", op, ""))
		require.Equal(t, []string{"OT11"}, x.texts("operations/operation/typeOperation"), "invalid but representable values stay exported")
		require.True(t, task8Diag(e.Validation.Diagnostics, "ATTACHMENT_MISSING", "", t8Piece))
		require.Empty(t, x.all("pieceJointe/piece"))
		require.Equal(t, "failed", e.Validation.Layers["references"].Status)

		// The saved draft keeps every user decision for later correction.
		kept, err := w.uc.GetDraft(w.ctx, "case")
		require.NoError(t, err)
		dangling := task8Value(t, kept, op, t8Op+"/emetteurTypeEmetteur")
		require.Equal(t, second, dangling.Reference.BlockID)
		require.True(t, dangling.NeedsReview)
		require.Len(t, task8Blocks(kept, t8Piece), 1)
		require.NotNil(t, task8Blocks(kept, t8Piece)[0].Attachment)
		require.Equal(t, "Saisie manuelle", *task8Value(t, kept, op, t8Op+"/commentaires").Text)
	})
}
