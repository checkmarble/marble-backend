package repositories

import (
	"context"
	"marble/marble-backend/models"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

type DecisionRepositoryLegacy interface {
	StoreDecision(ctx context.Context, orgID string, decision models.Decision) (models.Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (models.Decision, error)
	ListDecisions(ctx context.Context, orgID string) ([]models.Decision, error)
}

type DecisionRepository interface {
	DecisionsOfScheduledExecution(scheduledExecutionId string) (<-chan models.Decision, <-chan error)
}

type DecisionRepositoryImpl struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *DecisionRepositoryImpl) DecisionsOfScheduledExecution(scheduledExecutionId string) (<-chan models.Decision, <-chan error) {
	decisionsChan := make(chan models.Decision, 100)
	errorChan := make(chan error, 1)

	go func() {
		defer close(decisionsChan)
		defer close(errorChan)

		decisions := []models.Decision{
			{
				ID:                  uuid.NewString(),
				CreatedAt:           time.Now(),
				Outcome:             models.Approve,
				ScenarioID:          uuid.NewString(),
				ScenarioName:        "Scenario 1",
				ScenarioDescription: "Scenario 1",
				ScenarioVersion:     1,
				Score:               666,
			},
		}

		err := repo.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(transaction Transaction) error {
			// tx := adaptClientDatabaseTransaction(transaction)

			for i := 1; i <= 100; i++ {
				var decision = decisions[0]
				// for _, decision := range decisions {

				decisionsChan <- decision
			}
			return nil
		})

		errorChan <- err
	}()

	return decisionsChan, errorChan
}

// func gfet() {

// 	sql, args, err := r.queryBuilder.
// 	Select(
// 		"d.*",
// 		"array_agg(row(dr.*)) as rules",
// 	).
// 	From("decisions d").
// 	Join("decision_rules dr on dr.decision_id = d.id").
// 	Where("d.org_id = ?", orgID).
// 	GroupBy("d.id").
// 	OrderBy("d.created_at DESC").
// 	Limit(1000).
// 	ToSql()
// if err != nil {
// 	return []models.Decision{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
// }

// rows, _ := r.db.Query(ctx, sql, args...)
// decisionsDTOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[DbDecisionWithRules])
// if err != nil {
// 	return nil, fmt.Errorf("unable to list decisions: %w", err)
// }
// decisions := make([]models.Decision, len(decisionsDTOs))
// for i, dbDecision := range decisionsDTOs {
// 	decisions[i] = dbDecision.toDomain()
// 	for _, dbRule := range dbDecision.Rules {
// 		decisions[i].RuleExecutions = append(decisions[i].RuleExecutions, dbRule.toDomain())
// 	}
// }

// return decisions, nil
// }
