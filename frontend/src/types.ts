export type SlotID =
  | "species"
  | "body"
  | "head"
  | "clothing"
  | "gear"
  | "pose"
  | "stage"
  | "base"
  | "product_target";

export type CharacterConfig = {
  schema_version: 1;
  species: string;
  body: string;
  head: string;
  clothing: string;
  gear: string;
  pose: string;
  stage: string;
  base: string;
  scale: {
    mm: number;
  };
  product_target: string;
  colors: Record<string, string>;
  assembly: AssemblyConfig;
};

export type Vector3Tuple = [number, number, number];

export type AssemblyConfig = {
  placements: AssemblyPlacement[];
};

export type AssemblyPlacement = {
  id: string;
  slot: SlotID;
  attachment_point: string;
  asset_id: string;
  transform: {
    position: Vector3Tuple;
    rotation: Vector3Tuple;
    scale: number;
  };
};

export type Slot = {
  id: SlotID;
  label: string;
  description: string;
};

export type Part = {
  id: string;
  slot: SlotID;
  label: string;
  preview_asset: string;
  render_asset: string;
  tags: string[];
  color_zones: string[];
  print_constraints: {
    min_thickness_mm: number;
    max_scale_mm: number;
    supports_stl: boolean;
  };
};

export type ColorZone = {
  id: string;
  label: string;
  default: string;
};

export type Selection = {
  slot: SlotID;
  part_id: string;
};

export type CompatibilityRule = {
  id: string;
  description: string;
  when: Selection;
  disables: Selection[];
  reason: string;
};

export type Catalog = {
  version: string;
  slots: Slot[];
  parts: Part[];
  rules: CompatibilityRule[];
  color_zones: ColorZone[];
  defaults: CharacterConfig;
};

export type Project = {
  id: string;
  name: string;
  config: CharacterConfig;
  created_at: string;
  updated_at: string;
};

export type ExportJob = {
  id: string;
  project_id: string;
  target: string;
  format: string;
  status: "queued" | "processing" | "completed" | "failed";
  artifact: Record<string, unknown>;
  error: string | null;
  created_at: string;
  updated_at: string;
};

export type ModelAssetAssemblyPoint = {
  id: string;
  label: string;
  position: Vector3Tuple;
  rotation: Vector3Tuple;
};

export type ModelAsset = {
  id: string;
  label: string;
  slot: SlotID;
  attachment_point: string;
  file_name: string;
  content_type: string;
  format: "glb" | "gltf" | "obj" | "stl" | "3mf";
  size_bytes: number;
  url: string;
  assembly_points: ModelAssetAssemblyPoint[];
  created_at: string;
  updated_at: string;
};

export type AssemblyPointTarget = {
  id: string;
  slot: SlotID;
  label: string;
  description: string;
};

export type AssemblyTransform = {
  position: Vector3Tuple;
  rotation: Vector3Tuple;
  scale: number;
};

export type ResolvedAssemblyPlacement = {
  id: string;
  slot: SlotID;
  attachment_point: string;
  asset: {
    id: string;
    label: string;
    file_name: string;
    content_type: string;
    format: ModelAsset["format"];
    size_bytes: number;
    url: string;
  };
  asset_points: ModelAssetAssemblyPoint[];
  socket_transform: AssemblyTransform;
  user_transform: AssemblyTransform;
  world_transform: AssemblyTransform;
};

export type AssemblyManifest = {
  schema_version: 1;
  project_id: string;
  generated_at: string;
  scale_mm: number;
  character_scale: number;
  unit: string;
  selections: Record<string, string>;
  placements: ResolvedAssemblyPlacement[];
  warnings: string[];
};

export type EditorCategory = SlotID | "assets" | "assembly" | "color" | "projects" | "export";

export type PrintReadiness = {
  maxScale: number;
  constrainedBy: string;
  minThicknessMM: number;
  supportsSTL: boolean;
  activeColorZones: string[];
  selectedPartCount: number;
  uploadedAssetCount: number;
};
