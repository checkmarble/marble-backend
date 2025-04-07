-- +goose Up

alter table cases
    add column boost text default null;

create index idx_cases_add_to_case_workflow
    on cases (org_id, inbox_id, id)
    where (status in ('pending', 'investigating'));

drop index cases_add_to_case_workflow_idx;

create index idx_inbox_cases
    on cases (org_id, inbox_id, (boost is null), created_at desc, id desc);

-- +goose Down

create index cases_add_to_case_workflow_idx
    on cases (org_id, inbox_id, id)
    where (status IN ('open', 'investigating'));

drop index idx_inbox_cases;
drop index idx_cases_add_to_case_workflow;

alter table cases
    drop column boost;