-- +goose Up
-- +goose StatementBegin
CREATE TABLE
  phantom_decisions (
    id uuid DEFAULT uuid_generate_v4 (),
    org_id uuid NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    outcome decision_outcome NOT NULL,
    scenario_id uuid NOT NULL,
    score INT NOT NULL,
    scenario_iteration_id uuid NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT fk_phantom_decisions_org FOREIGN KEY (org_id) REFERENCES organizations (id) ON DELETE CASCADE,
    CONSTRAINT fk_phantom_decisions_scenario_ite_id FOREIGN KEY (scenario_iteration_id) REFERENCES scenario_iterations (id) ON DELETE CASCADE
  );

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE phantom_decisions;

-- +goose StatementEnd