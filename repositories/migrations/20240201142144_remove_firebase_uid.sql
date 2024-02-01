-- +goose Up
-- +goose StatementBegin
DROP INDEX users_firebase_idx;
ALTER TABLE users DROP COLUMN firebase_uid;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN firebase_uid VARCHAR NOT NULL;
CREATE INDEX users_firebase_idx ON users(firebase_uid);
-- +goose StatementEnd
