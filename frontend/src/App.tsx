import { FormEvent, useEffect, useMemo, useState } from "react";
import { FileDown, FolderOpen, RefreshCw, Save, Trash2 } from "lucide-react";
import CharacterPreview from "./CharacterPreview";
import { createExport, createProject, deleteProject, getCatalog, getProject, listProjects, updateProject } from "./api";
import type { Catalog, CharacterConfig, ExportJob, Part, Project, SlotID } from "./types";

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
};

const editorSlots: SlotID[] = ["species", "head", "body", "clothing", "gear", "pose", "stage", "base"];

export default function App() {
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [projectName, setProjectName] = useState("Taverna hero");
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [config, setConfig] = useState<CharacterConfig>(fallbackConfig);
  const [projects, setProjects] = useState<Project[]>([]);
  const [lastExport, setLastExport] = useState<ExportJob | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [status, setStatus] = useState("Готово");

  const partsBySlot = useMemo(() => groupPartsBySlot(catalog?.parts ?? []), [catalog]);
  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  );

  useEffect(() => {
    void boot();
  }, []);

  async function boot() {
    setIsLoading(true);
    try {
      const loadedCatalog = await getCatalog();
      setCatalog(loadedCatalog);
      setConfig(normalizeConfig(loadedCatalog.defaults, loadedCatalog));
      const loadedProjects = await listProjects();
      setProjects(loadedProjects);
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

  async function saveProject(event: FormEvent) {
    event.preventDefault();
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
      setProjectName(saved.name);
      setConfig(normalizeConfig(saved.config, catalog));
      await refreshProjects();
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
      setProjectName(project.name);
      setConfig(normalizeConfig(project.config, catalog));
      setLastExport(null);
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
      }
      await refreshProjects();
      setStatus("Проект удален");
    } catch (error) {
      setStatus(`Ошибка удаления: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  async function createExportJob() {
    if (!selectedProjectId) {
      setStatus("Сначала сохраните проект");
      return;
    }

    setIsExporting(true);
    try {
      const job = await createExport(selectedProjectId, config.product_target, formatForTarget(config.product_target));
      setLastExport(job);
      setStatus(`Export job ${job.status}`);
    } catch (error) {
      setStatus(`Ошибка export job: ${getErrorMessage(error)}`);
    } finally {
      setIsExporting(false);
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

  const currentTargetPart = partsBySlot.product_target?.find((part) => part.id === config.product_target);

  return (
    <main className="app-shell">
      <section className="workspace">
        <div className="preview-panel">
          <div className="preview-toolbar">
            <div>
              <p className="eyebrow">Taverna Catalog MVP+</p>
              <h1>{projectName}</h1>
            </div>
            <div className="status-pill">{status}</div>
          </div>
          <div className="preview-stage">
            {!catalog && <div className="preview-overlay">Загрузка каталога</div>}
            <CharacterPreview config={config} />
          </div>
        </div>

        <aside className="control-panel">
          <form onSubmit={saveProject} className="editor-form">
            <label className="field wide">
              <span>name</span>
              <input value={projectName} onChange={(event) => setProjectName(event.target.value)} />
            </label>

            {catalog ? (
              <>
                {editorSlots.map((slot) => (
                  <CatalogSelect
                    key={slot}
                    config={config}
                    catalog={catalog}
                    label={catalog.slots.find((item) => item.id === slot)?.label ?? slot}
                    slot={slot}
                    parts={partsBySlot[slot] ?? []}
                    onChange={(value) => updateSlot(slot, value)}
                  />
                ))}

                <label className="field wide">
                  <span>scale.mm</span>
                  <input
                    type="range"
                    min="24"
                    max="75"
                    value={config.scale.mm}
                    onChange={(event) => updateScale(Number(event.target.value))}
                  />
                  <strong>{config.scale.mm} mm</strong>
                </label>

                <div className="color-grid wide">
                  {catalog.color_zones.map((zone) => (
                    <ColorField
                      key={zone.id}
                      label={zone.label}
                      value={config.colors[zone.id] ?? zone.default}
                      onChange={(value) => updateColor(zone.id, value)}
                    />
                  ))}
                </div>

                <button className="primary-button wide" type="submit" disabled={isSaving}>
                  <Save size={18} />
                  {isSaving ? "Сохранение..." : "Сохранить проект"}
                </button>
              </>
            ) : (
              <p className="empty-state wide">Каталог загружается</p>
            )}
          </form>

          <div className="projects-panel">
            <div className="panel-heading">
              <h2>Проекты</h2>
              <button className="icon-button" type="button" onClick={refreshProjects} disabled={isLoading} title="Загрузить проекты">
                <RefreshCw size={18} />
              </button>
            </div>
            <button className="secondary-button" type="button" onClick={refreshProjects} disabled={isLoading}>
              <FolderOpen size={18} />
              Загрузить проекты
            </button>

            <div className="project-list">
              {projects.map((project) => (
                <article className={project.id === selectedProject?.id ? "project-item active" : "project-item"} key={project.id}>
                  <button type="button" onClick={() => openProject(project.id)}>
                    <span>{project.name}</span>
                    <small>{new Date(project.updated_at).toLocaleString()}</small>
                  </button>
                  <button className="delete-button" type="button" onClick={() => removeProject(project.id)} title="Удалить проект">
                    <Trash2 size={16} />
                  </button>
                </article>
              ))}
              {projects.length === 0 && <p className="empty-state">Сохраненных проектов пока нет</p>}
            </div>
          </div>

          <div className="projects-panel">
            <div className="panel-heading">
              <h2>Share / Export</h2>
              <span className="mini-badge">{currentTargetPart?.label ?? config.product_target}</span>
            </div>
            {catalog && (
              <CatalogSelect
                config={config}
                catalog={catalog}
                label="Product target"
                slot="product_target"
                parts={partsBySlot.product_target ?? []}
                onChange={(value) => updateSlot("product_target", value)}
              />
            )}
            <button className="secondary-button" type="button" onClick={createExportJob} disabled={!selectedProjectId || isExporting}>
              <FileDown size={18} />
              {isExporting ? "Создание job..." : "Создать export job"}
            </button>
            {lastExport && (
              <div className="export-status">
                <strong>{lastExport.status}</strong>
                <span>{lastExport.format.toUpperCase()} · {lastExport.id}</span>
              </div>
            )}
          </div>
        </aside>
      </section>
    </main>
  );
}

type CatalogSelectProps = {
  config: CharacterConfig;
  catalog: Catalog;
  label: string;
  slot: SlotID;
  parts: Part[];
  onChange: (value: string) => void;
};

function CatalogSelect({ config, catalog, label, slot, parts, onChange }: CatalogSelectProps) {
  return (
    <label className="field">
      <span>{label}</span>
      <select value={selectionFor(config, slot)} onChange={(event) => onChange(event.target.value)}>
        {parts.map((part) => {
          const reason = disabledReason(config, catalog, slot, part.id);
          return (
            <option key={part.id} value={part.id} disabled={Boolean(reason)}>
              {part.label}{reason ? " (blocked)" : ""}
            </option>
          );
        })}
      </select>
    </label>
  );
}

type ColorFieldProps = {
  label: string;
  value: string;
  onChange: (value: string) => void;
};

function ColorField({ label, value, onChange }: ColorFieldProps) {
  return (
    <label className="color-field">
      <span>{label}</span>
      <input type="color" value={value} onChange={(event) => onChange(event.target.value)} />
      <code>{value}</code>
    </label>
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
    colors: { ...defaults.colors, ...(isRecord(config.colors) ? config.colors : {}) },
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
  return next;
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

function isRecord(value: unknown): value is Record<string, string> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function clampNumber(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return min;
  }
  return Math.min(max, Math.max(min, value));
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "unknown error";
}
