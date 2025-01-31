-- +goose Up

alter table sanction_check_configs add column query jsonb not null default '{}';

alter table sanction_check_configs
drop constraint fk_scenario_iteration,
add constraint fk_scenario_iteration
    foreign key (scenario_iteration_id)
    references scenario_iterations (id)
    on delete cascade;

-- +goose Down

alter table sanction_check_configs drop column query;