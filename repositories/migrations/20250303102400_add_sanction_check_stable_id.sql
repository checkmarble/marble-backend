-- +goose Up
-- +goose StatementBegin

alter table sanction_check_configs
    add column stable_id uuid null;

update sanction_check_configs scc
set stable_id = sci.scenario_id
from scenario_iterations sci
where
    scc.scenario_iteration_id = sci.id and
    stable_id is null;

alter table sanction_check_configs
    alter column stable_id set not null;

-- +goose StatementEnd

-- +goose Down

alter table sanction_check_configs
    drop column stable_id;