-- +goose Up
-- +goose StatementBegin

create table scenario_test_run_summaries (
    id uuid default gen_random_uuid(),
    test_run_id uuid not null,
    version int not null,
    rule_stable_id text,
    rule_name text,
    watermark timestamp with time zone not null,
    outcome text not null,
    total int not null default 0,

    primary key (id),

    constraint fk_scenario_test_run
        foreign key (test_run_id)
        references scenario_test_run (id)
);

alter table scenario_test_run
    add column summarized bool not null default false;

create unique index idx_unique_scenario_test_summaries on scenario_test_run_summaries (test_run_id, version, rule_stable_id, outcome);
create index idx_scenario_test_summaries_test_run on scenario_test_run_summaries (test_run_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop table scenario_test_run_summaries;

alter table scenario_test_run
    drop column summarized;

-- +goose StatementEnd