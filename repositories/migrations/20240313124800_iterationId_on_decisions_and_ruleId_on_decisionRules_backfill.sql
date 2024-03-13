-- +goose Up
-- +goose StatementBegin
-- update decisions scenario_iteration_id with the id of the iteration matching by scenario_id and version number
UPDATE decisions
SET
      scenario_iteration_id = (
            SELECT
                  id
            FROM
                  scenario_iterations AS si
            WHERE
                  scenario_id = decisions.scenario_id
                  AND version = decisions.scenario_version
      )
WHERE
      scenario_iteration_id IS NULL;

-- delete decisions without a matching iteration (should not happen unless decision or scenario iteration version numbers have been manually changed in the db)
DELETE decisions
WHERE
      scenario_iteration_id IS NULL;

-- update decision_rules rule_id with the id of the rule matching by name and iteration_id - if it is ambiguous and several rules on the iteration have the same name, just take the first one
UPDATE decision_rules AS dr
SET
      rule_id = (
            SELECT
                  id
            FROM
                  scenario_iteration_rules AS sir
            WHERE
                  sir.scenario_iteration_id = d.scenario_iteration_id
                  AND sir.name = dr.name
            LIMIT
                  1
      )
FROM
      decisions AS d
WHERE
      d.id = dr.decision_id
      AND dr.rule_id IS NULL;

-- delete decision_rules without a matching rule (should not happen unless rule names have been manually changed in the db)
DELETE decision_rules
WHERE
      rule_id IS NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd