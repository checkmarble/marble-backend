package pg_repository

import (
	"context"
	"fmt"
)

func (r *PGRepository) PublishScenarioIteration(orgID string, scenarioID string, scenarioIterationID string) error {

	sql, args, err := r.queryBuilder.
		Update("scenarios").
		Set("live_scenario_iteration_id", scenarioIterationID).
		Where("id = ?", scenarioID).
		Where("org_id = ?", orgID).
		ToSql()

	if err != nil {
		return fmt.Errorf("unable to build query: %w", err)
	}

	_, err = r.db.Exec(context.TODO(), sql, args...)
	if err != nil {
		return fmt.Errorf("unable to run query: %w", err)
	}

	return nil
}

func (r *PGRepository) UnpublishScenarioIteration(orgID string, scenarioID string, scenarioIterationID string) error {

	sql, args, err := r.queryBuilder.
		Update("scenarios").
		Set("live_scenario_iteration_id", nil).
		Where("id = ?", scenarioID).
		Where("org_id = ?", orgID).
		ToSql()

	if err != nil {
		return fmt.Errorf("unable to build query: %w", err)
	}

	_, err = r.db.Exec(context.TODO(), sql, args...)
	if err != nil {
		return fmt.Errorf("unable to run query: %w", err)
	}

	return nil
}
