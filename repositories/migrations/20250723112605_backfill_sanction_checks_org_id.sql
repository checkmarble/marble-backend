-- +goose Up
-- +goose StatementBegin

with orgs as (
    select scc.id as scc_id, si.org_id si_org_id
    from sanction_check_configs scc
    inner join scenario_iterations si on si.id = scc.scenario_iteration_id
)
UPDATE sanction_checks 
SET org_id = orgs.si_org_id
FROM orgs
WHERE sanction_checks.sanction_check_config_id = orgs.scc_id
AND sanction_checks.org_id IS NULL;

-- +goose StatementEnd

-- +goose Down