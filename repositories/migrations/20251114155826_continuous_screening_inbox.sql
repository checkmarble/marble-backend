-- +goose Up
-- +goose StatementBegin

alter table continuous_screening_configs
    add column inbox_id uuid not null,
    add constraint fk_continuous_screening_configs_inbox_id
        foreign key (inbox_id)
        references inboxes(id);

alter table continuous_screening
    add column case_id uuid,
    add constraint fk_continuous_screening_case_id
        foreign key (case_id)
        references cases(id)
        on delete set null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table continuous_screening
    drop constraint fk_continuous_screening_case_id,
    drop column case_id;

alter table continuous_screening_configs
    drop constraint fk_continuous_screening_configs_inbox_id,
    drop column inbox_id;

-- +goose StatementEnd
