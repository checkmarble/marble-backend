-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_add_to_case_idx ON decisions (org_id, pivot_value, case_id)
WHERE
      pivot_value IS NOT NULL
      AND case_id IS NOT NULL;

CREATE INDEX cases_add_to_case_workflow_idx ON cases (org_id, inbox_id, id)
WHERE
      status IN ('open', 'investigating');

DROP INDEX decisions_case_id_idx;

CREATE INDEX decisions_case_id_idx ON decisions (org_id, case_id) INCLUDE (pivot_value);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_case_id_idx;

DROP INDEX cases_add_to_case_workflow_idx;

DROP INDEX decisions_add_to_case_idx;

CREATE INDEX decisions_case_id_idx ON decisions (org_id, case_id);

-- +goose StatementEnd