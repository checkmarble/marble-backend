-- +goose Up

create table graph_relations (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,
    group_id uuid not null,
    label text not null,
    left_type text not null,
    left_field text not null,
    right_type text not null,
    right_field text not null,
    created_at timestamp with time zone not null default current_timestamp,

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade,

    unique (org_id, group_id, left_type, left_field, right_type, right_field)
);

create index idx_graph_relations_org_id on graph_relations (org_id);

-- +goose Down

drop table graph_relations;
