-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    row_record RECORD;
    match_record RECORD;
    modified_text TEXT;
    old_snippet TEXT;
    new_snippet TEXT;
    val NUMERIC;
BEGIN
    -- 1. Find rows that match the pattern
    -- Note: replace "id" with your actual primary key column name
    FOR row_record IN
        SELECT id, formula_ast_expression::text AS ast_text
        FROM scenario_iteration_rules
        WHERE formula_ast_expression::text ~ '"percentile":\{\s*"constant":[0-9]+'
    LOOP
        -- Initialize our working text with the original JSON string
        modified_text := row_record.ast_text;

        -- 2. Extract the actual pieces of JSON
        -- We use regexp_matches with the 'g' (global) flag to find EVERY occurrence in the row.
        -- It returns an array of our capture groups:
        -- parts[1] = '"percentile":{"constant":'
        -- parts[2] = the number (e.g., '85' or '99.9')
        -- parts[3] = the closing brace '}' (accounting for potential spaces)
        FOR match_record IN
            SELECT regexp_matches(
                row_record.ast_text,
                '("percentile":\{"constant":)([0-9]+(?:\.[0-9]+)?)(\})',
                'g'
            ) AS parts
        LOOP
            -- Cast the extracted string number to numeric
            val := match_record.parts[2]::numeric;

            -- 3. Check the value
            IF val > 1 THEN
                -- Reconstruct the exact old string snippet so we can replace it safely
                old_snippet := match_record.parts[1] || match_record.parts[2] || match_record.parts[3];

                -- Create the new downscaled snippet
                new_snippet := match_record.parts[1] || trim_scale(val / 100)::text || match_record.parts[3];

                -- 4. Replace the old snippet with the new one in our working text.
                -- (If the exact same snippet exists twice in the row, this safely updates both)
                modified_text := REPLACE(modified_text, old_snippet, new_snippet);
                raise notice '%', modified_text;
                raise notice 'before: %, after: %', old_snippet, new_snippet;
            END IF;
        END LOOP;

        -- 5. If the text was actually changed, update the database row
        IF modified_text <> row_record.ast_text THEN
           UPDATE scenario_iteration_rules
            SET formula_ast_expression = modified_text::jsonb  -- Change to ::json if that's your column type
            WHERE id = row_record.id;
        END IF;


    END LOOP;
END $$;

-- +goose StatementEnd
-- +goose Down