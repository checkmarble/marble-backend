-- +goose Up

-- The API key trigger was attached to users, duplicating the user audit.
DROP TRIGGER IF EXISTS api_keys ON users;

CREATE TRIGGER api_keys
AFTER INSERT OR UPDATE OR DELETE ON api_keys
FOR EACH ROW EXECUTE FUNCTION global_audit();

-- +goose Down

DROP TRIGGER IF EXISTS api_keys ON api_keys;

CREATE TRIGGER api_keys
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION global_audit();
