CREATE TABLE IF NOT EXISTS export_jobs (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	target TEXT NOT NULL,
	format TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('queued', 'processing', 'completed', 'failed')),
	artifact JSONB NOT NULL DEFAULT '{}'::jsonb,
	error TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS export_jobs_project_id_idx ON export_jobs (project_id);
CREATE INDEX IF NOT EXISTS export_jobs_status_idx ON export_jobs (status);
CREATE INDEX IF NOT EXISTS export_jobs_updated_at_idx ON export_jobs (updated_at DESC);
