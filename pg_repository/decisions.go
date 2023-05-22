package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"marble/marble-backend/app"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbDecision struct {
	ID                  string      `db:"id"`
	OrgID               string      `db:"org_id"`
	CreatedAt           time.Time   `db:"created_at"`
	Outcome             string      `db:"outcome"`
	ScenarioID          string      `db:"scenario_id"`
	ScenarioName        string      `db:"scenario_name"`
	ScenarioDescription string      `db:"scenario_description"`
	ScenarioVersion     int         `db:"scenario_version"`
	Score               int         `db:"score"`
	ErrorCode           int         `db:"error_code"`
	DeletedAt           pgtype.Time `db:"deleted_at"`
}

func (d *dbDecision) toDomain() app.Decision {
	return app.Decision{
		ID:                  d.ID,
		CreatedAt:           d.CreatedAt,
		Outcome:             app.OutcomeFrom(d.Outcome),
		ScenarioID:          d.ScenarioID,
		ScenarioName:        d.ScenarioName,
		ScenarioDescription: d.ScenarioDescription,
		ScenarioVersion:     d.ScenarioVersion,
		Score:               d.Score,
		DecisionError:       app.DecisionError(d.ErrorCode),
	}
}

type DbDecisionWithRules struct {
	dbDecision
	Rules []dbDecisionRule
}

func (r *PGRepository) GetDecision(ctx context.Context, orgID string, decisionID string) (app.Decision, error) {
	sql, args, err := r.queryBuilder.
		Select(
			"d.*",
			"array_agg(row(dr.*)) as rules",
		).
		From("decisions d").
		Join("decision_rules dr on dr.decision_id = d.id").
		Where("d.org_id = ?", orgID).
		Where("d.id = ?", decisionID).
		GroupBy("d.id").
		ToSql()
	if err != nil {
		return app.Decision{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	decision, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DbDecisionWithRules])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Decision{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Decision{}, fmt.Errorf("unable to get decision: %w", err)
	}

	decisionDTO := decision.toDomain()
	for _, rule := range decision.Rules {
		decisionDTO.RuleExecutions = append(decisionDTO.RuleExecutions, rule.toDomain())
	}
	return decisionDTO, nil
}

func (r *PGRepository) ListDecisions(ctx context.Context, orgID string) ([]app.Decision, error) {
	sql, args, err := r.queryBuilder.
		Select(
			"d.*",
			"array_agg(row(dr.*)) as rules",
		).
		From("decisions d").
		Join("decision_rules dr on dr.decision_id = d.id").
		Where("d.org_id = ?", orgID).
		GroupBy("d.id").
		OrderBy("d.created_at DESC").
		Limit(1000).
		ToSql()
	if err != nil {
		return []app.Decision{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	decisionsDTOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[DbDecisionWithRules])
	if err != nil {
		return nil, fmt.Errorf("unable to list decisions: %w", err)
	}
	decisions := make([]app.Decision, len(decisionsDTOs))
	for i, dbDecision := range decisionsDTOs {
		decisions[i] = dbDecision.toDomain()
		for _, dbRule := range dbDecision.Rules {
			decisions[i].RuleExecutions = append(decisions[i].RuleExecutions, dbRule.toDomain())
		}
	}

	return decisions, nil
}

func (r *PGRepository) StoreDecision(ctx context.Context, orgID string, decision app.Decision) (app.Decision, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return app.Decision{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	sql, args, err := r.queryBuilder.
		Insert("decisions").
		Columns(
			"org_id",
			"outcome",
			"scenario_id",
			"scenario_name",
			"scenario_description",
			"scenario_version",
			"score",
			"error_code",
		).
		Values(
			orgID,
			decision.Outcome.String(),
			decision.ScenarioID,
			decision.ScenarioName,
			decision.ScenarioDescription,
			decision.ScenarioVersion,
			decision.Score,
			decision.DecisionError,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Decision{}, fmt.Errorf("unable to build decision query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdDecision, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbDecision])
	if err != nil {
		return app.Decision{}, fmt.Errorf("unable to create decision: %w", err)
	}

	createdDecisionRules, err := r.createDecisionRules(ctx, tx, orgID, createdDecision.ID, decision.RuleExecutions)
	if err != nil {
		return app.Decision{}, fmt.Errorf("unable to create decision rules: %w", err)
	}

	createdDecisionDTO := createdDecision.toDomain()
	createdDecisionDTO.RuleExecutions = createdDecisionRules

	err = tx.Commit(ctx)
	if err != nil {
		return app.Decision{}, fmt.Errorf("transaction issue: %w", err)
	}

	return createdDecisionDTO, nil
}
