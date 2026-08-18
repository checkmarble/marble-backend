package usecases

import (
	"context"
	"maps"
	"slices"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

// graphIngestionChunkSize bounds how many object ids go into one statement's array parameter,
// matching the convention the graph read queries already follow.
const graphIngestionChunkSize = 1000

// graphIndexedFieldsCache holds, per organization, which fields of which record types the adjacency
// table carries. Computing that needs both the data model and the graph relations, and the relations
// live in the marble database — a cross-database read the ingestion transaction would otherwise pay
// on every batch. A relation registered during the TTL is picked up seconds late; measured against
// the daily rebuild that used to be the only path, that is not a regression.
var graphIndexedFieldsCache = expirable.NewLRU[string, map[string][]models.Field](
	50, nil, utils.GlobalCacheDuration())

// maintainGraphRows brings the graph adjacency table in line with the records just ingested, so a
// graph walk can reach them without waiting for the next periodic rebuild.
//
// It reports no error by design. The graph is derived data with a rebuild job behind it, so the worst
// outcome of giving up here is the staleness that was the status quo — whereas propagating would
// fail a customer's ingestion over a feature they may not even use.
//
// That contract has to hold against a panic too, not just an error: this runs inside the ingestion
// transaction, so an unrecovered one takes the whole request down — exactly the outcome the rest of
// this is written to avoid. A missing dependency is the realistic way to get one, since nothing but
// the constructor guarantees the graph repositories are wired.
func (usecase *IngestionUseCase) maintainGraphRows(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	table models.Table,
	results models.IngestionResults,
) {
	defer utils.RecoverAndReportSentryError(ctx, "maintainGraphRows")

	// Keyed by the object ids actually written: a payload older than the stored version is dropped
	// during the merge and is correctly absent here, so a no-op ingestion does no graph work.
	if len(results) == 0 {
		return
	}

	logger := utils.LoggerFromContext(ctx)

	fa, err := usecase.featureAccessReader.GetOrganizationFeatureAccess(ctx, organizationId, nil)
	if err != nil {
		logger.WarnContext(ctx, "ingestion: could not check graph exploration access, skipping graph maintenance",
			"org_id", organizationId, "error", err.Error())
		return
	}
	if !fa.GraphExploration.IsAllowed() {
		return
	}

	fields, err := usecase.graphIndexedFields(ctx, organizationId, table.Name)
	if err != nil {
		logger.WarnContext(ctx, "ingestion: could not resolve the graph fields, skipping graph maintenance",
			"org_id", organizationId, "record_type", table.Name, "error", err.Error())
		return
	}
	if len(fields) == 0 {
		return
	}

	// Sorted for query plan stability, as the ingestion statements themselves are.
	objectIds := slices.Sorted(maps.Keys(results))

	for chunk := range slices.Chunk(objectIds, graphIngestionChunkSize) {
		if err := usecase.writeGraphRows(ctx, tx, table.Name, fields, chunk); err != nil {
			logger.WarnContext(ctx, "ingestion: could not maintain the graph table, it will be stale until the next build",
				"org_id", organizationId, "record_type", table.Name, "error", err.Error())
			utils.LogAndReportSentryError(ctx, err)
			return
		}
	}
}

// writeGraphRows runs the two adjacency statements in a subtransaction (savepoint).
//
// The savepoint is what makes the caller's best-effort contract achievable at all. The rebuild job
// swaps the table in by dropping it and renaming the freshly built one over it, so a write racing
// that swap finds the relation it resolved gone and errors — and an error on a plain statement would
// abort the whole ingestion transaction with it. Rolling back to a savepoint instead leaves the
// ingestion intact and costs the graph one batch, which the next build restores. The same isolation
// covers an organization whose first build has not run and so has no table yet, which is why nothing
// probes for its existence on this path.
func (usecase *IngestionUseCase) writeGraphRows(
	ctx context.Context,
	tx repositories.Transaction,
	recordType string,
	fields []models.Field,
	objectIds []string,
) error {
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return err
	}

	if _, err := usecase.graphIncrementalRepository.UpsertGraphRows(
		ctx, savepoint, recordType, fields, objectIds); err != nil {
		_ = savepoint.Rollback(ctx)
		return err
	}

	// After the upsert, so it sees the values that one wrote: a field the new version left empty has
	// no row to upsert, and this is what removes the one the previous version left behind.
	if _, err := usecase.graphIncrementalRepository.RetractGraphRows(
		ctx, savepoint, recordType, fields, objectIds); err != nil {
		_ = savepoint.Rollback(ctx)
		return err
	}

	return savepoint.Commit(ctx)
}

func (usecase *IngestionUseCase) graphIndexedFields(
	ctx context.Context,
	organizationId uuid.UUID,
	recordType string,
) ([]models.Field, error) {
	if fields, ok := graphIndexedFieldsCache.Get(organizationId.String()); ok {
		return fields[recordType], nil
	}

	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, true)
	if err != nil {
		return nil, err
	}

	relations, err := usecase.graphRelationRepository.ListGraphRelations(ctx, exec, organizationId)
	if err != nil {
		return nil, err
	}

	// Derived through the same function the build uses, so the two writers agree on which fields the
	// table is supposed to carry as well as on how their values are rendered.
	fields := models.GraphIndexedFields(dataModel, relations)
	graphIndexedFieldsCache.Add(organizationId.String(), fields)

	return fields[recordType], nil
}
