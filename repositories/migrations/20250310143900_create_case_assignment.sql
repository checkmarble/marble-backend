-- +goose Up

alter table cases
    add column assigned_to uuid null default null,
    add constraint fk_assigned_to_user
        foreign key (assigned_to) references users (id)
        on delete set null;

-- +goose Down

alter table cases
    drop column assigned_to;