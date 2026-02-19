-- +goose Up

create extension if not exists postgis;
create extension if not exists btree_gist;

alter type data_model_types add value if not exists 'IpAddress';
alter type data_model_types add value if not exists 'Coords';

-- +goose Down
