-- +goose Up

create or replace trigger audit
after insert or update or delete
on scenario_workflow_actions
for each row execute function global_audit();

create or replace trigger audit
after insert or update or delete
on scenario_workflow_conditions
for each row execute function global_audit();

create or replace trigger audit
after insert or update or delete
on scenario_workflow_rules
for each row execute function global_audit();

-- +goose Down

drop trigger if exists audit on scenario_workflow_actions;
drop trigger if exists audit on scenario_workflow_conditions;
drop trigger if exists audit on scenario_workflow_rules;
