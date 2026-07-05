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
