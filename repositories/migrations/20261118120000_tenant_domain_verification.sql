-- +goose Up
ALTER TABLE tenant_domains ALTER COLUMN verified_at DROP NOT NULL;
CREATE UNIQUE INDEX tenant_domains_tenant_domain_unique_idx ON tenant_domains (tenant_id, domain);

-- +goose Down
DROP INDEX tenant_domains_tenant_domain_unique_idx;
UPDATE tenant_domains SET verified_at = now() WHERE verified_at IS NULL;
ALTER TABLE tenant_domains ALTER COLUMN verified_at SET NOT NULL;
