-- +goose Up
-- +goose StatementBegin
ALTER TABLE case_contributors ADD CONSTRAINT case_contributors_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE upload_logs DROP CONSTRAINT upload_logs_user_id_fkey;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE case_contributors DROP CONSTRAINT case_contributors_user_id_fkey;
ALTER TABLE upload_logs ADD CONSTRAINT upload_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
