-- +goose Up

create or replace trigger audit
after insert or update or delete
on entity_annotations
for each row execute function global_audit();

-- +goose Down

drop trigger if exists audit on entity_annotations;
