ALTER TABLE model_assets
	ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{"status":"unknown","warnings":[]}'::jsonb;
