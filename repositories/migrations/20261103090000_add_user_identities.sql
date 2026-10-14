-- +goose Up

create table user_identities (
    user_id uuid not null references users (id) on delete cascade,
    issuer text not null check (issuer <> ''),
    subject text not null check (subject <> ''),
    primary key (issuer, subject)
);

create index user_identities_user_id_idx on user_identities (user_id);

-- +goose Down

drop table user_identities;
