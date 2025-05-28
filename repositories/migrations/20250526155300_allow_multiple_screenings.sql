-- +goose Up
-- +goose StatementBegin

alter table sanction_check_configs
    add column was_of_kind text,
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

do $$
    declare new_stable_id uuid := gen_random_uuid();

    declare iteration_configs cursor for
    select stable_id, array_agg(sanction_check_configs.*) as rows
    from sanction_check_configs
    where query->>'name' is not null and query->>'label' is not null
    group by stable_id;

    declare config sanction_check_configs%rowtype;
begin
    for iteration_config in iteration_configs loop
    new_stable_id := gen_random_uuid();

    foreach config in array iteration_config.rows loop
        update sanction_check_configs
        set
            query = ((query - 'label')->>'name')::jsonb,
            was_of_kind = 'name'
        where id = config.id;

        insert into sanction_check_configs (scenario_iteration_id, name, description, rule_group, forced_outcome, trigger_rule, datasets, query, created_at, updated_at, counterparty_id_expression, stable_id, was_of_kind)
        values (
            config.scenario_iteration_id,
            config.name,
            config.description,
            config.rule_group,
            config.forced_outcome,
            config.trigger_rule,
            config.datasets,
            case (config.query->>'label')::jsonb->>'name'
            when 'StringConcat' then (config.query->>'label')::jsonb
            else jsonb_build_object(
                'name', 'StringConcat',
                'children', jsonb_build_array((config.query->>'label')::jsonb),
                'named_children', jsonb_build_object(
                    'with_separator', jsonb_build_object('constant', true)
                )
            )
            end,
            config.created_at,
            config.updated_at,
            config.counterparty_id_expression,
            new_stable_id,
            'label'
        );
        end loop;
    end loop;
end
$$ language plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table sanction_checks
    drop column sanction_check_config_id;

do $$
declare
    iteration_configs cursor for
    select
        scenario_iteration_id,
        (min(id::text) filter (where was_of_kind = 'name')::uuid) as initial_id,
        json_build_object(
            'name', min(query::text) filter (where was_of_kind = 'name')::jsonb,
            'label', min(query::text) filter (where was_of_kind = 'label')::jsonb
        ) as query
    from sanction_check_configs
    where was_of_kind is not null
    group by scenario_iteration_id;
begin
    for iteration_config in iteration_configs loop
    update sanction_check_configs
    set
        query = iteration_config.query,
        was_of_kind = null
    where
        id = iteration_config.initial_id and
        scenario_iteration_id = iteration_config.scenario_iteration_id;

    delete from sanction_check_configs
    where
        id <> iteration_config.initial_id and
        scenario_iteration_id = iteration_config.scenario_iteration_id;
    end loop;
end
$$ language plpgsql;

alter table sanction_check_configs
    drop column was_of_kind,
    add constraint sanction_check_configs_scenario_iteration_id_key
    unique (scenario_iteration_id);

-- +goose StatementEnd
