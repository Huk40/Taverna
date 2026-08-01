import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  createExport,
  createProject,
  deleteModelAsset,
  deleteProject,
  getAssemblyPointTargets,
  getCatalog,
  getProject,
  getProjectAssembly,
  listModelAssets,
  listProjectExports,
  listProjects,
  updateProject,
  uploadModelAsset,
} from "./api";
import EditorShell from "./EditorShell";
import type {
  AssemblyPlacement,
  AssemblyManifest,
  AssemblyPointTarget,
  Catalog,
  CharacterConfig,
  EditorCategory,
  ExportJob,
  ModelAsset,
  Part,
  PrintReadiness,
  Project,
  SlotID,
  Vector3Tuple,
} from "./types";

const fallbackConfig: CharacterConfig = {
  schema_version: 1,
  species: "human_01",
  body: "balanced_body_01",
  head: "default_head_01",
  clothing: "basic_armor_01",
  gear: "sword_01",
  pose: "idle_01",
  stage: "plain_stage_01",
  base: "round_base_32mm",
  scale: { mm: 32 },
  product_target: "stl_print",
  colors: {
    skin: "#c99672",
    hair: "#3b2416",
    clothing_primary: "#444444",
    clothing_secondary: "#8B0000",
    gear_metal: "#d8dee9",
    stage: "#29313d",
  },
  assembly: { placements: [] },
};

const editorSlots: SlotID[] = ["species", "head", "body", "clothing", "gear", "pose", "stage", "base"];

export default function App() {
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [projectName, setProjectName] = useState("Taverna hero");
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [config, setConfig] = useState<CharacterConfig>(fallbackConfig);
  const [projects, setProjects] = useState<Project[]>([]);
  const [lastExport, setLastExport] = useState<ExportJob | null>(null);
  const [exportJobs, setExportJobs] = useState<ExportJob[]>([]);
  const [modelAssets, setModelAssets] = useState<ModelAsset[]>([]);
  const [assemblyManifest, setAssemblyManifest] = useState<AssemblyManifest | null>(null);
  const [assemblyTargets, setAssemblyTargets] = useState<AssemblyPointTarget[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [isUploadingAsset, setIsUploadingAsset] = useState(false);
  const [activeCategory, setActiveCategory] = useState<EditorCategory>("species");
  const [status, setStatus] = useState("Готово");

  const partsBySlot = useMemo(() => groupPartsBySlot(catalog?.parts ?? []), [catalog]);
  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  );
  const shareURL = useMemo(() => shareURLForProject(selectedProjectId), [selectedProjectId]);
  const printReadiness = useMemo(() => (catalog ? printReadinessForConfig(config, catalog) : null), [catalog, config]);

  useEffect(() => {
    void boot();
  }, []);

  async function boot() {
    setIsLoading(true);
    try {
      const [loadedCatalog, loadedProjects, loadedAssets, loadedTargets] = await Promise.all([
        getCatalog(),
        listProjects(),
        listModelAssets(),
        getAssemblyPointTargets(),
      ]);
      setCatalog(loadedCatalog);
      setConfig(normalizeConfig(loadedCatalog.defaults, loadedCatalog));
      setProjects(loadedProjects);
      setModelAssets(loadedAssets);
      setAssemblyTargets(loadedTargets);
      const sharedProjectID = projectIDFromURL();
      if (sharedProjectID) {
        try {
          const sharedProject = await getProject(sharedProjectID);
          setSelectedProjectId(sharedProject.id);
          setProjectName(sharedProject.name);
          setConfig(normalizeConfig(sharedProject.config, loadedCatalog));
          await syncExportJobs(sharedProject.id);
          await syncAssemblyManifest(sharedProject.id);
          setStatus("Проект открыт по ссылке");
          return;
        } catch (error) {
          clearProjectURLParam();
          setStatus(`Ссылка на проект недоступна: ${getErrorMessage(error)}`);
          return;
        }
      }
      setStatus(`Каталог ${loadedCatalog.version}, проектов: ${loadedProjects.length}`);
    } catch (error) {
      setStatus(`Ошибка загрузки: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  async function refreshProjects() {
    setIsLoading(true);
    try {
      const loaded = await listProjects();
      setProjects(loaded);
      setStatus(`Загружено проектов: ${loaded.length}`);
    } catch (error) {
      setStatus(`Ошибка загрузки: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  async function refreshModelAssets() {
    try {
      const loaded = await listModelAssets();
      setModelAssets(loaded);
      setStatus(`Загружено 3D-моделей: ${loaded.length}`);
    } catch (error) {
      setStatus(`Ошибка загрузки 3D-моделей: ${getErrorMessage(error)}`);
    }
  }

  async function uploadAsset(file: File, label: string, slot: SlotID, attachmentPoint: string) {
    setIsUploadingAsset(true);
    try {
      const asset = await uploadModelAsset({
        file,
        label,
        slot,
        attachmentPoint,
      });
      setModelAssets((current) => [asset, ...current.filter((item) => item.id !== asset.id)]);
      setActiveCategory("assembly");
      setStatus(`3D-модель загружена: ${asset.label}`);
    } catch (error) {
      setStatus(`Ошибка загрузки 3D-модели: ${getErrorMessage(error)}`);
    } finally {
      setIsUploadingAsset(false);
    }
  }

  async function removeModelAsset(id: string) {
    try {
      await deleteModelAsset(id);
      setModelAssets((current) => current.filter((asset) => asset.id !== id));
      setConfig((current) => ({
        ...current,
        assembly: {
          placements: current.assembly.placements.filter((placement) => placement.asset_id !== id),
        },
      }));
      setAssemblyManifest(null);
      setStatus("3D-модель удалена");
    } catch (error) {
      setStatus(`Ошибка удаления 3D-модели: ${getErrorMessage(error)}`);
    }
  }

  async function saveProject(event?: FormEvent) {
    event?.preventDefault();
    if (!catalog) {
      setStatus("Каталог еще загружается");
      return;
    }

    setIsSaving(true);
    try {
      const payload = { name: projectName, config: normalizeConfig(config, catalog) };
      const saved = selectedProjectId
        ? await updateProject(selectedProjectId, payload)
        : await createProject(payload);
      setSelectedProjectId(saved.id);
      setProjectURLParam(saved.id);
      setProjectName(saved.name);
      setConfig(normalizeConfig(saved.config, catalog));
      await refreshProjects();
      await syncExportJobs(saved.id);
      await syncAssemblyManifest(saved.id);
      setStatus("Проект сохранен");
    } catch (error) {
      setStatus(`Ошибка сохранения: ${getErrorMessage(error)}`);
    } finally {
      setIsSaving(false);
    }
  }

  async function openProject(id: string) {
    if (!catalog) {
      setStatus("Каталог еще загружается");
      return;
    }

    setIsLoading(true);
    try {
      const project = await getProject(id);
      setSelectedProjectId(project.id);
      setProjectURLParam(project.id);
      setProjectName(project.name);
      setConfig(normalizeConfig(project.config, catalog));
      await syncExportJobs(project.id);
      await syncAssemblyManifest(project.id);
      setStatus("Проект открыт");
    } catch (error) {
      setStatus(`Ошибка открытия: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  async function removeProject(id: string) {
    setIsLoading(true);
    try {
      await deleteProject(id);
      if (selectedProjectId === id) {
        setSelectedProjectId(null);
        setProjectName("Taverna hero");
        setConfig(catalog?.defaults ?? fallbackConfig);
        setLastExport(null);
        setExportJobs([]);
        setAssemblyManifest(null);
        clearProjectURLParam();
      }
      await refreshProjects();
      setStatus("Проект удален");
    } catch (error) {
      setStatus(`Ошибка удаления: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  async function createExportJob(formatOverride?: string) {
    if (!selectedProjectId) {
      setStatus("Сначала сохраните проект");
      return;
    }

    setIsExporting(true);
    try {
      const job = await createExport(selectedProjectId, config.product_target, formatOverride ?? formatForTarget(config.product_target));
      setLastExport(job);
      await syncExportJobs(selectedProjectId);
      setStatus(`Export job ${job.status}`);
    } catch (error) {
      setStatus(`Ошибка export job: ${getErrorMessage(error)}`);
    } finally {
      setIsExporting(false);
    }
  }

  async function copyShareLink() {
    if (!shareURL) {
      setStatus("Сначала сохраните проект");
      return;
    }

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(shareURL);
      } else {
        copyTextFallback(shareURL);
      }
      setStatus("Share link скопирован");
    } catch (error) {
      setStatus(`Ошибка копирования: ${getErrorMessage(error)}`);
    }
  }

  function updateSlot(slot: SlotID, partID: string) {
    if (!catalog) {
      return;
    }

    setConfig((current) => normalizeConfig(withSelection(current, slot, partID), catalog));
  }

  function updateScale(value: number) {
    if (!catalog) {
      return;
    }
    setConfig((current) => normalizeConfig({ ...current, scale: { mm: value } }, catalog));
  }

  function updateColor(zone: string, value: string) {
    setConfig((current) => ({ ...current, colors: { ...current.colors, [zone]: value } }));
  }

  function addAssemblyPlacement(asset: ModelAsset) {
    const placement: AssemblyPlacement = {
      id: makeClientID("placement"),
      slot: asset.slot,
      attachment_point: asset.attachment_point,
      asset_id: asset.id,
      transform: {
        position: [0, 0, 0],
        rotation: [0, 0, 0],
        scale: 1,
      },
    };

    setConfig((current) => ({
      ...current,
      assembly: {
        placements: [...current.assembly.placements, placement],
      },
    }));
    setAssemblyManifest(null);
    setActiveCategory("assembly");
    setStatus(`${asset.label} добавлен в сборку`);
  }

  function removeAssemblyPlacement(id: string) {
    setConfig((current) => ({
      ...current,
      assembly: {
        placements: current.assembly.placements.filter((placement) => placement.id !== id),
      },
    }));
    setAssemblyManifest(null);
  }

  function updateAssemblyPlacement(id: string, patch: Partial<AssemblyPlacement>) {
    setConfig((current) => ({
      ...current,
      assembly: {
        placements: current.assembly.placements.map((placement) =>
          placement.id === id
            ? {
                ...placement,
                ...patch,
                transform: {
                  ...placement.transform,
                  ...patch.transform,
                },
              }
            : placement,
        ),
      },
    }));
    setAssemblyManifest(null);
  }

  async function syncExportJobs(projectID: string) {
    const jobs = await listProjectExports(projectID);
    setExportJobs(jobs);
    setLastExport(jobs[0] ?? null);
    return jobs;
  }

  async function syncAssemblyManifest(projectID: string) {
    const manifest = await getProjectAssembly(projectID);
    setAssemblyManifest(manifest);
    return manifest;
  }

  async function refreshAssemblyManifest() {
    if (!selectedProjectId) {
      setStatus("Сначала сохраните проект");
      return;
    }
    try {
      const manifest = await syncAssemblyManifest(selectedProjectId);
      setStatus(`Assembly manifest: ${manifest.placements.length} placements`);
    } catch (error) {
      setStatus(`Ошибка assembly manifest: ${getErrorMessage(error)}`);
    }
  }

  return (
    <EditorShell
      activeCategory={activeCategory}
      assemblyManifest={assemblyManifest}
      assemblyTargets={assemblyTargets}
      catalog={catalog}
      config={config}
      editorSlots={editorSlots}
      exportJobs={exportJobs}
      isExporting={isExporting}
      isLoading={isLoading}
      isSaving={isSaving}
      isUploadingAsset={isUploadingAsset}
      lastExport={lastExport}
      modelAssets={modelAssets}
      partsBySlot={partsBySlot}
      printReadiness={printReadiness}
      projectName={projectName}
      projects={projects}
      selectedProject={selectedProject}
      selectedProjectId={selectedProjectId}
      shareURL={shareURL}
      status={status}
      onAddAssemblyPlacement={addAssemblyPlacement}
      onActiveCategoryChange={setActiveCategory}
      onColorChange={updateColor}
      onCopyShareLink={() => void copyShareLink()}
      onCreateExport={() => void createExportJob()}
      onCreateOBJExport={() => void createExportJob("obj")}
      onDeleteModelAsset={(id) => void removeModelAsset(id)}
      onOpenProject={(id) => void openProject(id)}
      onProjectNameChange={setProjectName}
      onRefreshAssemblyManifest={() => void refreshAssemblyManifest()}
      onRefreshProjects={() => void refreshProjects()}
      onRemoveProject={(id) => void removeProject(id)}
      onRemoveAssemblyPlacement={removeAssemblyPlacement}
      onSave={() => void saveProject()}
      onScaleChange={updateScale}
      onSlotChange={updateSlot}
      onUpdateAssemblyPlacement={updateAssemblyPlacement}
      onUploadAsset={(file, label, slot, attachmentPoint) => void uploadAsset(file, label, slot, attachmentPoint)}
    />
  );
}

function groupPartsBySlot(parts: Part[]): Record<string, Part[]> {
  return parts.reduce<Record<string, Part[]>>((groups, part) => {
    groups[part.slot] = groups[part.slot] ?? [];
    groups[part.slot].push(part);
    return groups;
  }, {});
}

function normalizeConfig(config: Partial<CharacterConfig> & Record<string, unknown>, catalog: Catalog): CharacterConfig {
  const defaults = catalog.defaults;
  const legacy = config as Record<string, unknown>;
  const normalized: CharacterConfig = {
    schema_version: 1,
    species: asString(config.species) ?? defaults.species,
    body: legacyPart("body", asString(config.body) ?? asString(legacy.body), defaults.body),
    head: legacyPart("head", asString(config.head) ?? asString(legacy.head), defaults.head),
    clothing: legacyPart("clothing", asString(config.clothing) ?? asString(legacy.armor), defaults.clothing),
    gear: legacyPart("gear", asString(config.gear) ?? asString(legacy.weapon), defaults.gear),
    pose: legacyPart("pose", asString(config.pose) ?? asString(legacy.pose), defaults.pose),
    stage: asString(config.stage) ?? defaults.stage,
    base: asString(config.base) ?? defaults.base,
    scale: {
      mm: clampNumber(
        typeof config.scale?.mm === "number" ? config.scale.mm : typeof legacy.scale_mm === "number" ? legacy.scale_mm : defaults.scale.mm,
        24,
        75,
      ),
    },
    product_target: asString(config.product_target) ?? defaults.product_target,
    colors: normalizeColors(defaults.colors, config.colors),
    assembly: normalizeAssembly(config.assembly),
  };

  return normalizeSelections(normalized, catalog);
}

function normalizeSelections(config: CharacterConfig, catalog: Catalog): CharacterConfig {
  let next = { ...config, colors: { ...config.colors }, scale: { ...config.scale } };
  for (let pass = 0; pass < 4; pass += 1) {
    for (const slot of [...editorSlots, "product_target" as SlotID]) {
      const selected = selectionFor(next, slot);
      const parts = catalog.parts.filter((part) => part.slot === slot);
      const selectedExists = parts.some((part) => part.id === selected);
      if (!selectedExists || disabledReason(next, catalog, slot, selected)) {
        const replacement = parts.find((part) => !disabledReason(next, catalog, slot, part.id));
        if (replacement) {
          next = withSelection(next, slot, replacement.id);
        }
      }
    }
  }
  next.scale.mm = clampNumber(next.scale.mm, 24, maxScaleForConfig(next, catalog));
  return next;
}

function normalizeAssembly(value: unknown): CharacterConfig["assembly"] {
  if (!isRecord(value) || !Array.isArray(value.placements)) {
    return { placements: [] };
  }

  return {
    placements: value.placements
      .map((item, index) => normalizePlacement(item, index))
      .filter((placement): placement is AssemblyPlacement => Boolean(placement)),
  };
}

function normalizeColors(defaults: Record<string, string>, value: unknown): Record<string, string> {
  const colors = { ...defaults };
  if (!isRecord(value)) {
    return colors;
  }
  for (const [zone, color] of Object.entries(value)) {
    if (typeof color === "string") {
      colors[zone] = color;
    }
  }
  return colors;
}

function normalizePlacement(value: unknown, index: number): AssemblyPlacement | null {
  if (!isRecord(value)) {
    return null;
  }

  const transform = isRecord(value.transform) ? value.transform : {};
  const slot = asString(value.slot);
  const attachmentPoint = asString(value.attachment_point);
  const assetID = asString(value.asset_id);
  if (!slot || !attachmentPoint || !assetID) {
    return null;
  }

  return {
    id: asString(value.id) ?? `placement-${index + 1}`,
    slot: slot as SlotID,
    attachment_point: attachmentPoint,
    asset_id: assetID,
    transform: {
      position: asVector3(transform.position, [0, 0, 0]),
      rotation: asVector3(transform.rotation, [0, 0, 0]),
      scale: clampNumber(asNumber(transform.scale, 1), 0.05, 10),
    },
  };
}

function selectionFor(config: CharacterConfig, slot: SlotID): string {
  return config[slot];
}

function withSelection(config: CharacterConfig, slot: SlotID, partID: string): CharacterConfig {
  return { ...config, [slot]: partID };
}

function disabledReason(config: CharacterConfig, catalog: Catalog, slot: SlotID, partID: string): string | null {
  for (const rule of catalog.rules) {
    if (selectionFor(config, rule.when.slot) !== rule.when.part_id) {
      continue;
    }
    if (rule.disables.some((disabled) => disabled.slot === slot && disabled.part_id === partID)) {
      return rule.reason;
    }
  }
  return null;
}

function formatForTarget(target: string): string {
  if (target === "glb_preview") {
    return "glb";
  }
  return "stl";
}

function printReadinessForConfig(config: CharacterConfig, catalog: Catalog): PrintReadiness {
  const selectedParts = selectedPartsForConfig(config, catalog);
  const geometryParts = selectedParts.filter((part) => part.slot !== "product_target");
  let maxScale = 75;
  let constrainedBy = "catalog default";
  let minThicknessMM = Number.POSITIVE_INFINITY;
  let supportsSTL = true;
  const colorZones = new Set<string>();

  for (const part of selectedParts) {
    if (part.print_constraints.max_scale_mm > 0 && part.print_constraints.max_scale_mm < maxScale) {
      maxScale = part.print_constraints.max_scale_mm;
      constrainedBy = part.label;
    }
  }

  for (const part of geometryParts) {
    if (part.print_constraints.min_thickness_mm > 0) {
      minThicknessMM = Math.min(minThicknessMM, part.print_constraints.min_thickness_mm);
    }
    if (!part.print_constraints.supports_stl) {
      supportsSTL = false;
    }
    part.color_zones.forEach((zone) => colorZones.add(zone));
  }

  return {
    maxScale,
    constrainedBy,
    minThicknessMM: Number.isFinite(minThicknessMM) ? minThicknessMM : 0,
    supportsSTL,
    activeColorZones: [...colorZones].sort(),
    selectedPartCount: geometryParts.length,
    uploadedAssetCount: config.assembly.placements.length,
  };
}

function maxScaleForConfig(config: CharacterConfig, catalog: Catalog): number {
  return printReadinessForConfig(config, catalog).maxScale;
}

function selectedPartsForConfig(config: CharacterConfig, catalog: Catalog): Part[] {
  return [...editorSlots, "product_target" as SlotID]
    .map((slot) => catalog.parts.find((part) => part.slot === slot && part.id === selectionFor(config, slot)))
    .filter((part): part is Part => Boolean(part));
}

function legacyPart(slot: SlotID, value: string | undefined, fallback: string): string {
  const legacy: Record<string, Record<string, string>> = {
    body: {
      human_male_01: "balanced_body_01",
      human_female_01: "balanced_body_01",
      dwarf_01: "compact_body_01",
      orc_01: "heroic_body_01",
    },
    clothing: {
      basic_armor_01: "basic_armor_01",
      leather_armor_01: "travel_cloak_01",
      robe_01: "mage_robe_01",
      cape_armor_01: "travel_cloak_01",
    },
    gear: {
      sword_01: "sword_01",
      axe_01: "axe_01",
      staff_01: "staff_01",
      none_01: "none_01",
    },
  };
  if (!value) {
    return fallback;
  }
  return legacy[slot]?.[value] ?? value;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function asVector3(value: unknown, fallback: Vector3Tuple): Vector3Tuple {
  if (!Array.isArray(value) || value.length !== 3) {
    return fallback;
  }
  return [
    asNumber(value[0], fallback[0]),
    asNumber(value[1], fallback[1]),
    asNumber(value[2], fallback[2]),
  ];
}

function clampNumber(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return min;
  }
  return Math.min(max, Math.max(min, value));
}

function makeClientID(prefix: string): string {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return `${prefix}-${crypto.randomUUID()}`;
  }
  return `${prefix}-${Date.now()}-${Math.round(Math.random() * 100000)}`;
}

function projectIDFromURL(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  const value = new URL(window.location.href).searchParams.get("project");
  return value && value.trim() ? value : null;
}

function shareURLForProject(projectID: string | null): string {
  if (!projectID || typeof window === "undefined") {
    return "";
  }
  const url = new URL(window.location.href);
  url.searchParams.set("project", projectID);
  url.hash = "";
  return url.toString();
}

function setProjectURLParam(projectID: string) {
  if (typeof window === "undefined") {
    return;
  }
  const url = new URL(window.location.href);
  url.searchParams.set("project", projectID);
  url.hash = "";
  window.history.replaceState(null, "", url);
}

function clearProjectURLParam() {
  if (typeof window === "undefined") {
    return;
  }
  const url = new URL(window.location.href);
  url.searchParams.delete("project");
  url.hash = "";
  window.history.replaceState(null, "", url);
}

function copyTextFallback(value: string) {
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "unknown error";
}
