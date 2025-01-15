-- +goose Up
-- +goose StatementBegin

create table sanction_check_configs (
    id uuid primary key default uuid_generate_v4(),
    scenario_iteration_id uuid unique,
    enabled boolean,
    updated_at timestamp with time zone not null default CURRENT_TIMESTAMP,

    constraint fk_scneario_iteration
        foreign key (scenario_iteration_id)
        references scenario_iterations (id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop table sanction_check_configs;

-- +goose StatementEnd