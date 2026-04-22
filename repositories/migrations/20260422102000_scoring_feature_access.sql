-- +goose Up

alter table licenses
    add column if not exists user_scoring boolean not null default false;

alter table organization_feature_access
    add column if not exists user_scoring text not null default 'allowed';

-- +goose Down

alter table licenses
    drop column if exists user_scoring;

alter table organization_feature_access
    drop column if exists user_scoring;
