-- +goose Up

alter table tags
    add column target text not null default 'case',
    add constraint tag_kind_check check (target in ('case', 'object'));

-- +goose Down

alter table tags
    drop column target;
