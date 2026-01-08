package main

// import (
// 	"context"
// 	"fmt"

// 	"github.com/checkmarble/marble-backend/utils"
// 	"github.com/google/uuid"
// 	"github.com/jackc/pgx/v5/pgxpool"
// )

// // Before deletion:
// // - disable Datastream CDC for decisions table

// // After deletion:
// // - re-enable Datastream CDC for decisions table
// // - run batch sync of decisions if needed

// const (
// 	orgId     = "314f26e4-41a1-45fb-a630-b04a7f8c85f2"
// 	batchSize = 10000
// )

// func main2() {
// 	logger := utils.NewLogger("text")
// 	ctx := context.Background()

// 	logger.InfoContext(ctx, "Starting batch deletion", "org_id", orgId, "batch_size", batchSize)

// 	pool, err := pgxpool.New(context.Background(),
// 		utils.GetRequiredEnv[string]("CONNECTION_STRING"))
// 	if err != nil {
// 		logger.ErrorContext(ctx, "failed to create pool", "error", err)
// 		return
// 	}
// 	defer pool.Close()

// 	totalDeleted := 0
// 	batchNum := 0

// 	for {
// 		batchNum++
// 		logger.InfoContext(ctx, "Processing batch", "batch_number", batchNum, "total_deleted", totalDeleted)

// 		// Delete a batch of records
// 		deleted, err := deleteBatch(ctx, pool, orgId, batchSize)
// 		if err != nil {
// 			logger.ErrorContext(ctx, "failed to delete batch", "error", err)
// 			return
// 		}

// 		totalDeleted += deleted
// 		logger.InfoContext(ctx, "Batch completed", "batch_number", batchNum,
// 			"deleted_in_batch", deleted, "total_deleted", totalDeleted)

// 		// If we deleted less than batchSize, we're done
// 		if deleted < batchSize {
// 			logger.InfoContext(ctx, "Deletion completed", "total_deleted", totalDeleted)
// 			break
// 		}
// 	}
// }

// func deleteBatch(ctx context.Context, pool *pgxpool.Pool, orgId uuid.UUID, limit int) (int, error) {
// 	// Use a transaction for the deletion
// 	tx, err := pool.Begin(ctx)
// 	if err != nil {
// 		return 0, fmt.Errorf("failed to begin transaction: %w", err)
// 	}
// 	defer tx.Rollback(ctx)

// 	// Delete records and return the count
// 	query := `
// 		DELETE FROM decision_rules
// 		WHERE id IN (
// 			SELECT id
// 			FROM decision_rules
// 			WHERE org_id = $1
// 			LIMIT $2
// 		)
// 	`

// 	result, err := tx.Exec(ctx, query, orgId, limit)
// 	if err != nil {
// 		return 0, fmt.Errorf("failed to execute delete: %w", err)
// 	}

// 	if err := tx.Commit(ctx); err != nil {
// 		return 0, fmt.Errorf("failed to commit transaction: %w", err)
// 	}

// 	return int(result.RowsAffected()), nil
// }
