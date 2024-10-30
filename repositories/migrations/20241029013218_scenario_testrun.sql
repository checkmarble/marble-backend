-- +goose Up
-- +goose StatementBegin
CREATE TABLE scenario_test_run(
 id uuid DEFAULT uuid_generate_v4(),
 scenario_iteration_id uuid NOT NULL,
 created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
 expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
 status VARCHAR NOT NULL,
 PRIMARY KEY(id),
 CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE scenario_test_run;
-- +goose StatementEnd
