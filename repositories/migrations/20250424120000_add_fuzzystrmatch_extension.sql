-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION if not exists fuzzystrmatch SCHEMA public;

do $$
declare
  extension_name text := 'pg_trgm';
begin
  perform true
  from pg_user pgu
  inner join pg_extension pge on pge.extowner = pgu.usesysid
  where
    pgu.usename = current_user and
    pge.extname = extension_name;

  if found then
    execute 'alter extension ' || quote_ident(extension_name) || ' set schema public';
  else
    raise notice 'WARN: could not install the %s extension into the public schema because we are not the owner', extension_name;
  end if;
end;
$$ language plpgsql;

-- +goose StatementEnd

-- +goose Down

DROP EXTENSION if exists fuzzystrmatch;
