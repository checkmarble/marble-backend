-- +goose Up
-- +goose StatementBegin

-- Update existing rows with a computed default value
UPDATE ai_case_reviews 
SET file_temp_reference = 'ai_case_reviews/temp/' || case_id::text || '/' || id::text || '.json'
WHERE file_temp_reference IS NULL;

-- Make the column NOT NULL now that all rows have values
ALTER TABLE ai_case_reviews 
ALTER COLUMN file_temp_reference SET NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_case_reviews 
ALTER COLUMN file_temp_reference DROP NOT NULL;
-- +goose StatementEnd
