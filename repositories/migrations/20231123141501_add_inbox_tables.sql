-- +goose Up
-- +goose StatementBegin

CREATE TYPE inbox_status AS ENUM (
    'active',
    'archived'
);

CREATE TABLE inboxes (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name varchar(255) NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp with time zone NOT NULL DEFAULT NOW(),
  organization_id UUID NOT NULL,
  status inbox_status NOT NULL DEFAULT 'active',
  CONSTRAINT fk_inboxes_org FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE TYPE inbox_roles AS ENUM (
    'member',
    'admin'
);

CREATE TABLE inbox_users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at timestamp with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp with time zone NOT NULL DEFAULT NOW(),
  inbox_id UUID NOT NULL,
  user_id UUID NOT NULL,
  role inbox_roles NOT NULL DEFAULT 'member',
  CONSTRAINT fk_inbox_users_inbox FOREIGN KEY(inbox_id) REFERENCES inboxes(id) ON DELETE CASCADE,
  CONSTRAINT fk_inbox_users_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE (inbox_id, user_id)
)



-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE inbox_users;
DROP TABLE inboxes;
DROP TYPE inbox_roles;
DROP TYPE inbox_status;

-- +goose StatementEnd
