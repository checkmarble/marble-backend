-- +goose Up

alter table scenario_iteration_rules
  drop constraint scenario_iteration_rules_snooze_group_id_fkey,
  add constraint scenario_iteration_rules_snooze_group_id_fkey
    foreign key (scenario_iteration_id) references scenario_iterations
    on delete set null;

-- +goose Down

-- Omited because we really don't want this.