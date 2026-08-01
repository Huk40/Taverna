CREATE TABLE IF NOT EXISTS model_assets (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	label TEXT NOT NULL,
	slot TEXT NOT NULL,
	attachment_point TEXT NOT NULL,
	file_name TEXT NOT NULL,
	content_type TEXT NOT NULL,
	format TEXT NOT NULL CHECK (format IN ('glb', 'gltf', 'obj', 'stl', '3mf')),
	size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
	storage_path TEXT NOT NULL,
	assembly_points JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS model_assets_slot_idx ON model_assets (slot);
CREATE INDEX IF NOT EXISTS model_assets_attachment_point_idx ON model_assets (attachment_point);
CREATE INDEX IF NOT EXISTS model_assets_updated_at_idx ON model_assets (updated_at DESC);
