-- +goose Up

create trigger audit
after insert
on scoring_scores
for each row
    when (new.source = 'override')
    execute function global_audit();

-- +goose Down

drop trigger if exists audit on scoring_scores;
