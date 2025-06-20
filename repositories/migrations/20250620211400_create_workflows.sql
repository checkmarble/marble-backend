-- +goose Up

create table scenario_workflow_rules (
    id uuid primary key default gen_random_uuid(),
    scenario_id uuid not null,
    name text not null,
    priority int not null default 99,
    created_at timestamp with time zone default now(),
    updated_at timestamp with time zone,

    constraint fk_scenario
        foreign key (scenario_id) references scenarios (id)
        on delete cascade
);

create table scenario_workflow_conditions (
    id uuid primary key default gen_random_uuid(),
    rule_id uuid not null,
    function text not null,
    params jsonb,
    created_at timestamp with time zone default now(),
    updated_at timestamp with time zone,

    constraint fk_rule
        foreign key (rule_id) references scenario_workflow_rules (id)
        on delete cascade
);

create table scenario_workflow_actions (
    id uuid primary key default gen_random_uuid(),
    rule_id uuid not null,
    action text not null,
    params jsonb,
    created_at timestamp with time zone default now(),
    updated_at timestamp with time zone,

    constraint fk_rule
        foreign key (rule_id) references scenario_workflow_rules (id)
        on delete cascade
);

-- +goose Down

drop table scenario_workflow_conditions;
drop table scenario_workflow_rules;
