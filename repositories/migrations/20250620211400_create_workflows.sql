-- +goose Up
-- +goose StatementBegin

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

do $$
    declare workflows cursor for
    select id, decision_to_case_inbox_id, decision_to_case_outcomes, decision_to_case_workflow_type, decision_to_case_name_template
    from scenarios
    where decision_to_case_workflow_type != 'DISABLED';

    declare workflow scenarios%rowtype;
    declare current_rule_id uuid;
begin
  for workflow in workflows loop
    insert into scenario_workflow_rules (scenario_id, name, priority)
    values (workflow.id, 'Migrated Workflow', 1)
    returning id into current_rule_id;

    if workflow.decision_to_case_outcomes is not null then
      insert into scenario_workflow_conditions (rule_id, function, params)
      values (current_rule_id, 'if_outcome_in', array_to_json(workflow.decision_to_case_outcomes)::jsonb);
    else
      insert into scenario_workflow_conditions (rule_id, function, params)
      values (current_rule_id, 'always', null);
    end if;

    insert into scenario_workflow_actions (rule_id, action, params)
    values (
      current_rule_id,
      workflow.decision_to_case_workflow_type,
      jsonb_build_object(
        'inbox_id', workflow.decision_to_case_inbox_id,
        'title_template', workflow.decision_to_case_name_template
      )
    );
  end loop;
end $$;

update scenario_iteration_rules
set stable_rule_id = gen_random_uuid()
where stable_rule_id is null;

alter table scenario_iteration_rules
alter column stable_rule_id set not null;

-- +goose StatementEnd

-- +goose Down

drop table scenario_workflow_conditions;
drop table scenario_workflow_actions;
drop table scenario_workflow_rules;
