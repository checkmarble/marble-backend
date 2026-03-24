-- +goose Up
ALTER TABLE organizations
DROP COLUMN transfer_check_scenario_id;

ALTER TABLE users
DROP COLUMN partner_id;

ALTER TABLE api_keys
DROP COLUMN partner_id;

ALTER TABLE webhook_events
DROP COLUMN partner_id;

DROP TABLE transfer_alerts;

DROP TABLE transfer_mappings;

DROP TABLE partners;

-- +goose Down
create table
    partners (
        id uuid default uuid_generate_v4 () not null primary key,
        created_at timestamp with time zone default now() not null,
        name varchar(255) not null,
        bic varchar default ''::character varying not null
    );

create table
    transfer_mappings (
        id uuid default uuid_generate_v4 () not null primary key,
        created_at timestamp with time zone default now() not null,
        organization_id uuid not null references organizations on delete cascade,
        client_transfer_id varchar(60) not null,
        partner_id uuid not null references partners on delete set null
    );

create table
    transfer_alerts (
        id uuid default uuid_generate_v4 () not null primary key,
        transfer_id uuid not null references transfer_mappings,
        organization_id uuid not null references organizations,
        sender_partner_id uuid not null references partners,
        beneficiary_partner_id uuid not null references partners,
        created_at timestamp with time zone default now() not null,
        status varchar(255) default 'pending'::character varying not null,
        message text not null,
        transfer_end_to_end_id varchar(255) not null,
        beneficiary_iban varchar(255) not null,
        sender_iban varchar(255) not null
    );

ALTER TABLE api_keys
ADD COLUMN partner_id uuid references partners (id) on delete set null;

ALTER TABLE users
ADD COLUMN partner_id uuid references partners (id) on delete set null;

ALTER TABLE organizations
ADD COLUMN transfer_check_scenario_id uuid references transfer_check_scenarios (id) on delete set null;

ALTER TABLE webhook_events
ADD COLUMN partner_id uuid references partners (id) on delete set null;