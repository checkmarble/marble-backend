-- +goose Up
alter table inboxes
    add column sla integer,
    add constraint inboxes_sla_check check (sla is null or sla >= 1);

-- +goose Down
alter table inboxes
    drop constraint inboxes_sla_check,
    drop column sla;
