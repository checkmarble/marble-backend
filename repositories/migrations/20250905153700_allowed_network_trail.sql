-- +goose Up

create or replace trigger audit
after update of allowed_networks
on organizations
for each row execute function global_audit();

-- +goose Down

drop trigger if exists audit on organizations;
