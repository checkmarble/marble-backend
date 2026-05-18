-- +goose Up

alter table licenses
    add column lexisnexis boolean not null default false;

alter table organizations
    add column screening_providers jsonb not null default '{}';

alter table organization_feature_access
    add column lexisnexis text not null default 'allowed';

alter table screening_configs
    add column provider text not null default 'opensanctions',
    add column filters jsonb not null default '{}';

-- +goose Down

alter table licenses
    drop column lexisnexis;

alter table organizations
    drop column screening_providers;

alter table organization_feature_access
    drop column lexisnexis;

alter table screening_configs
    drop column provider,
    drop column filters;
