-- +goose Up
-- +goose StatementBegin
-- users
CREATE TABLE users(
  id uuid DEFAULT uuid_generate_v4(),
  email VARCHAR NOT NULL,
  firebase_uid VARCHAR NOT NULL,
  role INTEGER NOT NULL,
  organization_id uuid,
  PRIMARY KEY(id)
);

CREATE UNIQUE INDEX users_email_idx ON users(email);
CREATE INDEX users_firebase_idx ON users(firebase_uid);
CREATE INDEX users_organizationid_idx ON users(organization_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX users_email_idx;
DROP INDEX users_firebase_idx;
DROP INDEX users_organizationid_idx;
DROP TABLE users;
-- +goose StatementEnd
