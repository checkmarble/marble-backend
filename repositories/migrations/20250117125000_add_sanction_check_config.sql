-- +goose Up
-- +goose StatementBegin
create table
    sanction_check_configs (
        id uuid primary key default uuid_generate_v4 (),
        scenario_iteration_id uuid unique,
        name text not null default '',
        description text not null default '',
        rule_group text not null default '',
        forced_outcome text null check (forced_outcome in (NULL, 'review', 'block_and_review', 'decline')),
        score_modifier int default 0,
        trigger_rule jsonb,
        datasets text[],
        query jsonb,
        created_at timestamp with time zone not null default CURRENT_TIMESTAMP,
        updated_at timestamp with time zone not null default CURRENT_TIMESTAMP,
        constraint fk_scenario_iteration foreign key (scenario_iteration_id) references scenario_iterations (id) on delete cascade
    );

alter table organizations
add column sanctions_threshold int not null default '70' check (sanctions_threshold between 0 and 100),
add column sanctions_limit int not null default '30' check (sanctions_limit >= 1);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
drop table sanction_check_configs;

alter table organizations
drop column sanctions_threshold,
drop column sanctions_limit;

-- +goose StatementEnd