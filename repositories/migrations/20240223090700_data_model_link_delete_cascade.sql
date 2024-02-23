-- +goose Up
-- +goose StatementBegin
ALTER TABLE data_model_links
DROP CONSTRAINT data_model_links_organization_id_fkey;

ALTER TABLE data_model_links
ADD CONSTRAINT data_model_links_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations ON DELETE CASCADE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE data_model_links
DROP CONSTRAINT data_model_links_organization_id_fkey;

ALTER TABLE data_model_links
ADD CONSTRAINT data_model_links_organization_id_fkey FOREIGN KEY organization_id REFERENCES organizations;

-- +goose StatementEnd