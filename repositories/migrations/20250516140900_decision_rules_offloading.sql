-- +goose Up

create table offloading_watermarks (
  org_id uuid,
  table_name text,
  watermark_time timestamp with time zone not null,
  watermark_id uuid not null,
  created_at timestamp with time zone,
  updated_at timestamp with time zone,

  primary key (org_id, table_name),
  constraint fk_org_id
    foreign key (org_id) references organizations (id)
    on delete cascade
);

-- +goose Down

drop table offloading_watermarks;