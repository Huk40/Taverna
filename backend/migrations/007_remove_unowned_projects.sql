-- Discard saved builds left over from before accounts were introduced.
-- Export jobs and asset references are removed by their ON DELETE CASCADE keys.
DELETE FROM projects WHERE owner_id IS NULL;

-- Every saved build must belong to an account from now on.
ALTER TABLE projects ALTER COLUMN owner_id SET NOT NULL;
