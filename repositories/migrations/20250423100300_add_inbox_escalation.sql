-- +goose Up

alter table inboxes
    add column escalation_inbox_id uuid,
    add constraint fk_escalation_inbox_id
        foreign key (escalation_inbox_id) references inboxes (id)
        on delete set null;

-- +goose Down

alter table inboxes
    drop column escalation_inbox_id;