UPDATE model_assets SET assembly_points='[]'::jsonb WHERE assembly_points='null'::jsonb;
-- Mirror JSON references into relations so PostgreSQL protects concurrent
-- project saves, asset deletion and anchor removal with foreign-key locks.
CREATE TABLE model_asset_anchors (
    asset_id UUID NOT NULL REFERENCES model_assets(id) ON DELETE CASCADE,
    anchor_id TEXT NOT NULL,
    PRIMARY KEY (asset_id, anchor_id)
);
INSERT INTO model_asset_anchors
SELECT a.id, point->>'id' FROM model_assets a,
LATERAL jsonb_array_elements(a.assembly_points) point;

CREATE TABLE project_asset_references (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    placement_id TEXT NOT NULL,
    asset_id UUID NOT NULL REFERENCES model_assets(id) ON DELETE RESTRICT,
    anchor_id TEXT,
    PRIMARY KEY (project_id, placement_id),
    FOREIGN KEY (asset_id, anchor_id) REFERENCES model_asset_anchors(asset_id, anchor_id) ON DELETE RESTRICT
);
CREATE INDEX project_asset_references_asset_idx ON project_asset_references(asset_id);

INSERT INTO project_asset_references
SELECT p.id, placement->>'id', (placement->>'asset_id')::uuid,
       NULLIF(BTRIM(placement->>'asset_point_id'), '')
FROM projects p, LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(p.config->'assembly'->'placements')='array' THEN p.config->'assembly'->'placements' ELSE '[]'::jsonb END) placement
WHERE NULLIF(placement->>'asset_id', '') IS NOT NULL;

CREATE FUNCTION sync_project_asset_references() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM project_asset_references WHERE project_id = NEW.id;
    INSERT INTO project_asset_references
    SELECT NEW.id, placement->>'id', (placement->>'asset_id')::uuid,
           NULLIF(BTRIM(placement->>'asset_point_id'), '')
    FROM jsonb_array_elements(CASE WHEN jsonb_typeof(NEW.config->'assembly'->'placements')='array' THEN NEW.config->'assembly'->'placements' ELSE '[]'::jsonb END) placement
    WHERE NULLIF(placement->>'asset_id', '') IS NOT NULL;
    RETURN NEW;
END $$;
CREATE TRIGGER project_asset_references_sync AFTER INSERT OR UPDATE OF config ON projects
FOR EACH ROW EXECUTE FUNCTION sync_project_asset_references();

CREATE FUNCTION sync_model_asset_anchors() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM model_asset_anchors a WHERE a.asset_id = NEW.id
    AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(NEW.assembly_points) point WHERE point->>'id' = a.anchor_id);
    INSERT INTO model_asset_anchors SELECT NEW.id, point->>'id'
    FROM jsonb_array_elements(NEW.assembly_points) point ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER model_asset_anchors_sync AFTER INSERT OR UPDATE OF assembly_points ON model_assets
FOR EACH ROW EXECUTE FUNCTION sync_model_asset_anchors();

CREATE INDEX projects_owner_page_idx ON projects(owner_id, updated_at DESC, id DESC);
CREATE INDEX model_assets_slot_page_idx ON model_assets(slot, updated_at DESC, id DESC);
CREATE INDEX export_jobs_project_page_idx ON export_jobs(project_id, updated_at DESC, id DESC);
