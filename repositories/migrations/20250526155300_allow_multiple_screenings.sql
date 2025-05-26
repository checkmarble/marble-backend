-- +goose Up
-- +goose StatementBegin

alter table sanction_check_configs
drop constraint if exists sanction_check_configs_scenario_iteration_id_key;

create index if not exists idx_scc_iteration_id
    on sanction_check_configs (scenario_iteration_id);

alter table sanction_checks
    add column sanction_check_config_id uuid,
    add constraint fk_sanction_check_config
        foreign key (sanction_check_config_id) references sanction_check_configs (id)
        on delete cascade;

with mapping as (
    select decision_id, scc.id as config_id
    from sanction_checks sc
    inner join decisions d on d.id = sc.decision_id
    inner join sanction_check_configs scc on scc.scenario_iteration_id = d.scenario_iteration_id
    where scc.scenario_iteration_id = d.scenario_iteration_id
)
update sanction_checks sc
set sanction_check_config_id = m.config_id
from mapping m
where sc.decision_id = m.decision_id;

alter table sanction_checks
    alter column sanction_check_config_id set not null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table sanction_checks
    drop column sanction_check_config_id;

alter table sanction_check_configs
    add constraint sanction_check_configs_scenario_iteration_id_key
    unique (scenario_iteration_id);

-- +goose StatementEnd
