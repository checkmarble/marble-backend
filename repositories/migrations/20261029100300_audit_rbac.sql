-- +goose Up
-- +goose StatementBegin

create trigger audit
after insert or update or delete on roles
for each row execute function global_audit();

create trigger audit
after insert or update or delete on groups
for each row execute function global_audit();

create trigger audit
after insert or update or delete on group_memberships
for each row execute function global_audit();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop trigger if exists audit on group_memberships;
drop trigger if exists audit on groups;
drop trigger if exists audit on roles;

-- +goose StatementEnd
