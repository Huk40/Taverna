import { useMemo, useState, type FormEvent } from "react";
import {
  AlertTriangle,
  Box,
  Boxes,
  CheckCircle2,
  Circle,
  Copy,
  FileDown,
  FolderOpen,
  Palette,
  RefreshCw,
  Ruler,
  Save,
  Shirt,
  Smile,
  Sword,
  Trash2,
  UploadCloud,
  UserRound,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import CharacterPreview from "./CharacterPreview";
import { exportArtifactURL } from "./api";
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

type EditorShellProps = {
  activeCategory: EditorCategory;
  assemblyManifest: AssemblyManifest | null;
  assemblyTargets: AssemblyPointTarget[];
  catalog: Catalog | null;
  config: CharacterConfig;
  editorSlots: SlotID[];
  exportJobs: ExportJob[];
  isExporting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  isUploadingAsset: boolean;
  lastExport: ExportJob | null;
  modelAssets: ModelAsset[];
  partsBySlot: Record<string, Part[]>;
  printReadiness: PrintReadiness | null;
  projectName: string;
  projects: Project[];
  selectedProject: Project | null;
  selectedProjectId: string | null;
  shareURL: string;
  status: string;
  onAddAssemblyPlacement: (asset: ModelAsset) => void;
  onActiveCategoryChange: (category: EditorCategory) => void;
  onColorChange: (zone: string, value: string) => void;
  onCopyShareLink: () => void;
  onCreateExport: () => void;
  onCreateOBJExport: () => void;
  onCreateSTLExport: () => void;
  onDeleteModelAsset: (id: string) => void;
  onOpenProject: (id: string) => void;
  onProjectNameChange: (value: string) => void;
  onRefreshAssemblyManifest: () => void;
  onRefreshProjects: () => void;
  onRemoveProject: (id: string) => void;
  onRemoveAssemblyPlacement: (id: string) => void;
  onSave: () => void;
  onScaleChange: (value: number) => void;
  onSlotChange: (slot: SlotID, partID: string) => void;
  onUpdateAssemblyPlacement: (id: string, patch: Partial<AssemblyPlacement>) => void;
  onUploadAsset: (file: File, label: string, slot: SlotID, attachmentPoint: string) => void;
};

export default function EditorShell({
  activeCategory,
  assemblyManifest,
  assemblyTargets,
  catalog,
  config,
  editorSlots,
  exportJobs,
  isExporting,
  isLoading,
  isSaving,
  isUploadingAsset,
  lastExport,
  modelAssets,
  partsBySlot,
  printReadiness,
  projectName,
  projects,
  selectedProject,
  selectedProjectId,
  shareURL,
  status,
  onAddAssemblyPlacement,
  onActiveCategoryChange,
  onColorChange,
  onCopyShareLink,
  onCreateExport,
  onCreateOBJExport,
  onCreateSTLExport,
  onDeleteModelAsset,
  onOpenProject,
  onProjectNameChange,
  onRefreshAssemblyManifest,
  onRefreshProjects,
  onRemoveProject,
  onRemoveAssemblyPlacement,
  onSave,
  onScaleChange,
  onSlotChange,
  onUpdateAssemblyPlacement,
  onUploadAsset,
}: EditorShellProps) {
  const currentTargetPart = partsBySlot.product_target?.find((part) => part.id === config.product_target);

  return (
    <main className="editor-shell">
      <header className="editor-topbar">
        <div className="project-heading">
          <p className="eyebrow">Taverna Catalog MVP+</p>
          <input
            className="project-name-input"
            aria-label="Project name"
            value={projectName}
            onChange={(event) => onProjectNameChange(event.target.value)}
          />
        </div>

        <div className="topbar-actions">
          <div className="status-pill">{status}</div>
          <button className="topbar-button primary-action" type="button" onClick={onSave} disabled={isSaving || !catalog}>
            <Save size={18} />
            {isSaving ? "Сохранение" : "Сохранить"}
          </button>
          <button className="topbar-button" type="button" onClick={() => onActiveCategoryChange("projects")}>
            <FolderOpen size={18} />
            Проекты
          </button>
          <button className="topbar-button" type="button" onClick={() => onActiveCategoryChange("export")}>
            <FileDown size={18} />
            Export
          </button>
        </div>
      </header>

      <section className="editor-workspace">
        <CategoryNav
          activeCategory={activeCategory}
          catalog={catalog}
          editorSlots={editorSlots}
          onChange={onActiveCategoryChange}
        />

        <section className="preview-panel" aria-label="3D editor viewport">
          <div className="preview-stage">
            {!catalog && <div className="preview-overlay">Загрузка каталога</div>}
            <CharacterPreview config={config} modelAssets={modelAssets} />
          </div>
          <div className="viewport-footer">
            <span>{categoryTitle(activeCategory, catalog)}</span>
            <strong>{currentTargetPart?.label ?? config.product_target}</strong>
          </div>
        </section>

        <aside className="inspector-panel">
          {renderInspector({
            activeCategory,
            assemblyManifest,
            assemblyTargets,
            catalog,
            config,
            editorSlots,
            exportJobs,
            isExporting,
            isLoading,
            isUploadingAsset,
            lastExport,
            modelAssets,
            partsBySlot,
            printReadiness,
            projects,
            selectedProject,
            selectedProjectId,
            shareURL,
            onAddAssemblyPlacement,
            onActiveCategoryChange,
            onColorChange,
            onCopyShareLink,
            onCreateExport,
            onCreateOBJExport,
            onCreateSTLExport,
            onDeleteModelAsset,
            onOpenProject,
            onRefreshAssemblyManifest,
            onRefreshProjects,
            onRemoveProject,
            onRemoveAssemblyPlacement,
            onScaleChange,
            onSlotChange,
            onUpdateAssemblyPlacement,
            onUploadAsset,
          })}
        </aside>
      </section>
    </main>
  );
}

type InspectorProps = Pick<
  EditorShellProps,
  | "activeCategory"
  | "assemblyManifest"
  | "assemblyTargets"
  | "catalog"
  | "config"
  | "editorSlots"
  | "exportJobs"
  | "isExporting"
  | "isLoading"
  | "isUploadingAsset"
  | "lastExport"
  | "modelAssets"
  | "partsBySlot"
  | "printReadiness"
  | "projects"
  | "selectedProject"
  | "selectedProjectId"
  | "shareURL"
  | "onAddAssemblyPlacement"
  | "onActiveCategoryChange"
  | "onColorChange"
  | "onCopyShareLink"
  | "onCreateExport"
  | "onCreateOBJExport"
  | "onCreateSTLExport"
  | "onDeleteModelAsset"
  | "onOpenProject"
  | "onRefreshAssemblyManifest"
  | "onRefreshProjects"
  | "onRemoveProject"
  | "onRemoveAssemblyPlacement"
  | "onScaleChange"
  | "onSlotChange"
  | "onUpdateAssemblyPlacement"
  | "onUploadAsset"
>;

function renderInspector(props: InspectorProps) {
  if (!props.catalog) {
    return <p className="empty-state">Каталог загружается</p>;
  }

  if (isSlotCategory(props.activeCategory)) {
    return (
      <PartsPanel
        catalog={props.catalog}
        config={props.config}
        parts={props.partsBySlot[props.activeCategory] ?? []}
        printReadiness={props.printReadiness}
        slot={props.activeCategory}
        onScaleChange={props.onScaleChange}
        onSlotChange={props.onSlotChange}
      />
    );
  }

  switch (props.activeCategory) {
    case "assets":
      return (
        <AssetPanel
          assemblyTargets={props.assemblyTargets}
          isUploadingAsset={props.isUploadingAsset}
          modelAssets={props.modelAssets}
          onAddAssemblyPlacement={props.onAddAssemblyPlacement}
          onDeleteModelAsset={props.onDeleteModelAsset}
          onUploadAsset={props.onUploadAsset}
        />
      );
    case "assembly":
      return (
        <AssemblyPanel
          assemblyTargets={props.assemblyTargets}
          config={props.config}
          modelAssets={props.modelAssets}
          onAddAssemblyPlacement={props.onAddAssemblyPlacement}
          onRemoveAssemblyPlacement={props.onRemoveAssemblyPlacement}
          onUpdateAssemblyPlacement={props.onUpdateAssemblyPlacement}
        />
      );
    case "color":
      return <ColorPanel catalog={props.catalog} config={props.config} onColorChange={props.onColorChange} />;
    case "projects":
      return (
        <ProjectPanel
          isLoading={props.isLoading}
          projects={props.projects}
          selectedProject={props.selectedProject}
          onOpenProject={props.onOpenProject}
          onRefreshProjects={props.onRefreshProjects}
          onRemoveProject={props.onRemoveProject}
        />
      );
    case "export":
      return (
        <ExportPanel
          assemblyManifest={props.assemblyManifest}
          catalog={props.catalog}
          config={props.config}
          exportJobs={props.exportJobs}
          isExporting={props.isExporting}
          lastExport={props.lastExport}
          parts={props.partsBySlot.product_target ?? []}
          printReadiness={props.printReadiness}
          selectedProjectId={props.selectedProjectId}
          shareURL={props.shareURL}
          onCopyShareLink={props.onCopyShareLink}
          onCreateExport={props.onCreateExport}
          onCreateOBJExport={props.onCreateOBJExport}
          onCreateSTLExport={props.onCreateSTLExport}
          onRefreshAssemblyManifest={props.onRefreshAssemblyManifest}
          onSlotChange={props.onSlotChange}
        />
      );
    default:
      return <p className="empty-state">Выберите категорию</p>;
  }
}

type CategoryNavProps = {
  activeCategory: EditorCategory;
  catalog: Catalog | null;
  editorSlots: SlotID[];
  onChange: (category: EditorCategory) => void;
};

function CategoryNav({ activeCategory, catalog, editorSlots, onChange }: CategoryNavProps) {
  const categories: EditorCategory[] = [...editorSlots, "assets", "assembly", "color", "projects", "export"];

  return (
    <nav className="category-nav" aria-label="Editor categories">
      {categories.map((category) => {
        const Icon = iconForCategory(category);
        return (
          <button
            className={activeCategory === category ? "category-button active" : "category-button"}
            key={category}
            type="button"
            onClick={() => onChange(category)}
            title={categoryTitle(category, catalog)}
          >
            <Icon size={18} />
            <span>{categoryNavLabel(category, catalog)}</span>
          </button>
        );
      })}
    </nav>
  );
}

type PartsPanelProps = {
  catalog: Catalog;
  config: CharacterConfig;
  parts: Part[];
  printReadiness: PrintReadiness | null;
  slot: SlotID;
  onScaleChange: (value: number) => void;
  onSlotChange: (slot: SlotID, partID: string) => void;
};

function PartsPanel({ catalog, config, parts, printReadiness, slot, onScaleChange, onSlotChange }: PartsPanelProps) {
  const slotMeta = catalog.slots.find((item) => item.id === slot);

  return (
    <div className="inspector-content">
      <PanelHeading title={slotMeta?.label ?? slot} subtitle={slotMeta?.description ?? "Choose a catalog part"} />
      <PartBrowser catalog={catalog} config={config} parts={parts} slot={slot} onSlotChange={onSlotChange} />
      {slot === "base" && printReadiness && (
        <>
          <ScaleControl config={config} maxScale={printReadiness.maxScale} onScaleChange={onScaleChange} />
          <PrintReadinessPanel readiness={printReadiness} />
        </>
      )}
    </div>
  );
}

type PartBrowserProps = {
  catalog: Catalog;
  config: CharacterConfig;
  parts: Part[];
  slot: SlotID;
  onSlotChange: (slot: SlotID, partID: string) => void;
};

function PartBrowser({ catalog, config, parts, slot, onSlotChange }: PartBrowserProps) {
  return (
    <div className="part-browser">
      {parts.map((part) => {
        const selected = selectionFor(config, slot) === part.id;
        const reason = disabledReason(config, catalog, slot, part.id);
        return (
          <button
            className={selected ? "part-card active" : "part-card"}
            disabled={Boolean(reason)}
            key={part.id}
            type="button"
            onClick={() => onSlotChange(slot, part.id)}
          >
            <span className="part-card-header">
              <strong>{part.label}</strong>
              {selected && <CheckCircle2 size={16} />}
            </span>
            <span className="part-card-assets">{part.preview_asset}</span>
            <span className="part-tags">
              {part.tags.slice(0, 3).map((tag) => (
                <em key={tag}>{tag}</em>
              ))}
            </span>
            <span className="part-meta">
              <span>Max {part.print_constraints.max_scale_mm} mm</span>
              <span>{part.color_zones.length || "no"} zones</span>
            </span>
            {reason && (
              <small className="disabled-reason">
                <AlertTriangle size={14} />
                {reason}
              </small>
            )}
          </button>
        );
      })}
      {parts.length === 0 && <p className="empty-state">В этой категории пока нет parts</p>}
    </div>
  );
}

type AssetPanelProps = {
  assemblyTargets: AssemblyPointTarget[];
  isUploadingAsset: boolean;
  modelAssets: ModelAsset[];
  onAddAssemblyPlacement: (asset: ModelAsset) => void;
  onDeleteModelAsset: (id: string) => void;
  onUploadAsset: (file: File, label: string, slot: SlotID, attachmentPoint: string) => void;
};

function AssetPanel({ assemblyTargets, isUploadingAsset, modelAssets, onAddAssemblyPlacement, onDeleteModelAsset, onUploadAsset }: AssetPanelProps) {
  const slotOptions = useMemo(() => uniqueSlotsForTargets(assemblyTargets), [assemblyTargets]);
  const [file, setFile] = useState<File | null>(null);
  const [label, setLabel] = useState("");
  const [slot, setSlot] = useState<SlotID>((slotOptions[0] ?? "gear") as SlotID);
  const [attachmentPoint, setAttachmentPoint] = useState("right_hand");
  const targetsForSlot = assemblyTargets.filter((target) => target.slot === slot);
  const selectedAttachmentPoint = targetsForSlot.some((target) => target.id === attachmentPoint)
    ? attachmentPoint
    : targetsForSlot[0]?.id ?? "";

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file || !selectedAttachmentPoint) {
      return;
    }
    onUploadAsset(file, label || stripExtension(file.name), slot, selectedAttachmentPoint);
    setFile(null);
    setLabel("");
  }

  return (
    <div className="inspector-content">
      <PanelHeading title="Assets" subtitle="Upload user GLB, OBJ, STL, or 3MF models to the server" />
      <form className="asset-upload-form" onSubmit={handleSubmit}>
        <label className="file-drop">
          <UploadCloud size={20} />
          <span>{file ? file.name : "Choose 3D model file"}</span>
          <input
            accept=".glb,.gltf,.obj,.stl,.3mf"
            type="file"
            onChange={(event) => {
              const nextFile = event.target.files?.[0] ?? null;
              setFile(nextFile);
              if (nextFile && !label) {
                setLabel(stripExtension(nextFile.name));
              }
            }}
          />
        </label>
        <label className="field">
          <span>Label</span>
          <input value={label} onChange={(event) => setLabel(event.target.value)} placeholder={file ? stripExtension(file.name) : "Custom sword"} />
        </label>
        <label className="field">
          <span>Slot</span>
          <select
            value={slot}
            onChange={(event) => {
              const nextSlot = event.target.value as SlotID;
              const nextTarget = assemblyTargets.find((target) => target.slot === nextSlot);
              setSlot(nextSlot);
              setAttachmentPoint(nextTarget?.id ?? "");
            }}
          >
            {slotOptions.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Attachment point</span>
          <select value={selectedAttachmentPoint} onChange={(event) => setAttachmentPoint(event.target.value)}>
            {targetsForSlot.map((target) => (
              <option key={target.id} value={target.id}>
                {target.label}
              </option>
            ))}
          </select>
        </label>
        <button className="wide-command primary-action" type="submit" disabled={!file || !selectedAttachmentPoint || isUploadingAsset}>
          <UploadCloud size={18} />
          {isUploadingAsset ? "Uploading..." : "Upload model"}
        </button>
      </form>

      <div className="asset-list">
        {modelAssets.map((asset) => (
          <article className="model-asset-card" key={asset.id}>
            <div className="asset-title-row">
              <strong>{asset.label}</strong>
              <span className="mini-badge">{asset.format.toUpperCase()}</span>
            </div>
            <span className="part-card-assets">{asset.file_name}</span>
            <span className="part-meta">
              <span>{asset.slot} / {asset.attachment_point}</span>
              <span>{formatBytes(asset.size_bytes)}</span>
            </span>
            <div className="asset-actions">
              <button className="wide-command" type="button" onClick={() => onAddAssemblyPlacement(asset)}>
                <Boxes size={16} />
                Add to assembly
              </button>
              <button className="delete-button" type="button" onClick={() => onDeleteModelAsset(asset.id)} title="Delete model asset">
                <Trash2 size={16} />
              </button>
            </div>
          </article>
        ))}
        {modelAssets.length === 0 && <p className="empty-state">Upload a model to start assembling custom parts</p>}
      </div>
    </div>
  );
}

type AssemblyPanelProps = {
  assemblyTargets: AssemblyPointTarget[];
  config: CharacterConfig;
  modelAssets: ModelAsset[];
  onAddAssemblyPlacement: (asset: ModelAsset) => void;
  onRemoveAssemblyPlacement: (id: string) => void;
  onUpdateAssemblyPlacement: (id: string, patch: Partial<AssemblyPlacement>) => void;
};

function AssemblyPanel({
  assemblyTargets,
  config,
  modelAssets,
  onAddAssemblyPlacement,
  onRemoveAssemblyPlacement,
  onUpdateAssemblyPlacement,
}: AssemblyPanelProps) {
  const assetsByID = useMemo(() => new Map(modelAssets.map((asset) => [asset.id, asset])), [modelAssets]);

  return (
    <div className="inspector-content">
      <PanelHeading title="Assembly" subtitle="Attach uploaded models to stable character assembly points" />
      <div className="assembly-source-list">
        {modelAssets.map((asset) => (
          <button className="part-card compact" type="button" key={asset.id} onClick={() => onAddAssemblyPlacement(asset)}>
            <span className="part-card-header">
              <strong>{asset.label}</strong>
              <Boxes size={16} />
            </span>
            <span className="part-meta">
              <span>{asset.slot}</span>
              <span>{asset.attachment_point}</span>
            </span>
          </button>
        ))}
        {modelAssets.length === 0 && <p className="empty-state">No uploaded models yet</p>}
      </div>

      <div className="placement-list">
        {config.assembly.placements.map((placement) => {
          const asset = assetsByID.get(placement.asset_id);
          const targets = assemblyTargets.filter((target) => target.slot === placement.slot);
          return (
            <article className="placement-card" key={placement.id}>
              <div className="asset-title-row">
                <strong>{asset?.label ?? "Missing asset"}</strong>
                <button className="delete-button small" type="button" onClick={() => onRemoveAssemblyPlacement(placement.id)} title="Remove placement">
                  <X size={15} />
                </button>
              </div>
              <label className="field">
                <span>Attachment point</span>
                <select
                  value={placement.attachment_point}
                  onChange={(event) => onUpdateAssemblyPlacement(placement.id, { attachment_point: event.target.value })}
                >
                  {targets.map((target) => (
                    <option key={target.id} value={target.id}>
                      {target.label}
                    </option>
                  ))}
                </select>
              </label>
              <VectorControl
                label="Position"
                step={0.05}
                value={placement.transform.position}
                onChange={(position) => onUpdateAssemblyPlacement(placement.id, { transform: { ...placement.transform, position } })}
              />
              <VectorControl
                label="Rotation"
                step={5}
                value={placement.transform.rotation}
                onChange={(rotation) => onUpdateAssemblyPlacement(placement.id, { transform: { ...placement.transform, rotation } })}
              />
              <label className="field">
                <span>Scale</span>
                <input
                  type="number"
                  min="0.05"
                  max="10"
                  step="0.05"
                  value={placement.transform.scale}
                  onChange={(event) =>
                    onUpdateAssemblyPlacement(placement.id, {
                      transform: { ...placement.transform, scale: Number(event.target.value) },
                    })
                  }
                />
              </label>
            </article>
          );
        })}
        {config.assembly.placements.length === 0 && <p className="empty-state">No assembly placements yet</p>}
      </div>
    </div>
  );
}

type VectorControlProps = {
  label: string;
  step: number;
  value: Vector3Tuple;
  onChange: (value: Vector3Tuple) => void;
};

function VectorControl({ label, step, value, onChange }: VectorControlProps) {
  const axes = ["x", "y", "z"] as const;
  return (
    <fieldset className="vector-control">
      <legend>{label}</legend>
      {axes.map((axis, index) => (
        <label key={axis}>
          <span>{axis}</span>
          <input
            type="number"
            step={step}
            value={value[index]}
            onChange={(event) => {
              const next = [...value] as Vector3Tuple;
              next[index] = Number(event.target.value);
              onChange(next);
            }}
          />
        </label>
      ))}
    </fieldset>
  );
}

type ColorPanelProps = {
  catalog: Catalog;
  config: CharacterConfig;
  onColorChange: (zone: string, value: string) => void;
};

function ColorPanel({ catalog, config, onColorChange }: ColorPanelProps) {
  return (
    <div className="inspector-content">
      <PanelHeading title="Color" subtitle="Material zones used by the selected catalog parts" />
      <div className="color-editor-grid">
        {catalog.color_zones.map((zone) => (
          <label className="color-tile" key={zone.id}>
            <span>
              <strong>{zone.label}</strong>
              <code>{config.colors[zone.id] ?? zone.default}</code>
            </span>
            <input
              aria-label={zone.label}
              type="color"
              value={config.colors[zone.id] ?? zone.default}
              onChange={(event) => onColorChange(zone.id, event.target.value)}
            />
          </label>
        ))}
      </div>
    </div>
  );
}

type ProjectPanelProps = {
  isLoading: boolean;
  projects: Project[];
  selectedProject: Project | null;
  onOpenProject: (id: string) => void;
  onRefreshProjects: () => void;
  onRemoveProject: (id: string) => void;
};

function ProjectPanel({ isLoading, projects, selectedProject, onOpenProject, onRefreshProjects, onRemoveProject }: ProjectPanelProps) {
  return (
    <div className="inspector-content">
      <PanelHeading title="Projects" subtitle="Saved character configs in PostgreSQL" />
      <button className="wide-command" type="button" onClick={onRefreshProjects} disabled={isLoading}>
        <RefreshCw size={18} />
        Загрузить проекты
      </button>
      <div className="project-list">
        {projects.map((project) => (
          <article className={project.id === selectedProject?.id ? "project-item active" : "project-item"} key={project.id}>
            <button type="button" onClick={() => onOpenProject(project.id)}>
              <span>{project.name}</span>
              <small>{new Date(project.updated_at).toLocaleString()}</small>
            </button>
            <button className="delete-button" type="button" onClick={() => onRemoveProject(project.id)} title="Удалить проект">
              <Trash2 size={16} />
            </button>
          </article>
        ))}
        {projects.length === 0 && <p className="empty-state">Сохраненных проектов пока нет</p>}
      </div>
    </div>
  );
}

type ExportPanelProps = {
  assemblyManifest: AssemblyManifest | null;
  catalog: Catalog;
  config: CharacterConfig;
  exportJobs: ExportJob[];
  isExporting: boolean;
  lastExport: ExportJob | null;
  parts: Part[];
  printReadiness: PrintReadiness | null;
  selectedProjectId: string | null;
  shareURL: string;
  onCopyShareLink: () => void;
  onCreateExport: () => void;
  onCreateOBJExport: () => void;
  onCreateSTLExport: () => void;
  onRefreshAssemblyManifest: () => void;
  onSlotChange: (slot: SlotID, partID: string) => void;
};

function ExportPanel({
  assemblyManifest,
  catalog,
  config,
  exportJobs,
  isExporting,
  lastExport,
  parts,
  printReadiness,
  selectedProjectId,
  shareURL,
  onCopyShareLink,
  onCreateExport,
  onCreateOBJExport,
  onCreateSTLExport,
  onRefreshAssemblyManifest,
  onSlotChange,
}: ExportPanelProps) {
  return (
    <div className="inspector-content">
      <PanelHeading title="Share / Export" subtitle="Target, print readiness, share link, and export jobs" />
      <PartBrowser catalog={catalog} config={config} parts={parts} slot="product_target" onSlotChange={onSlotChange} />
      {printReadiness && <PrintReadinessPanel readiness={printReadiness} />}
      <AssemblyManifestPanel manifest={assemblyManifest} selectedProjectId={selectedProjectId} onRefresh={onRefreshAssemblyManifest} />
      <div className="share-box">
        <label className="field">
          <span>Share link</span>
          <input value={shareURL} placeholder="Сохраните проект, чтобы получить ссылку" readOnly />
        </label>
        <button className="icon-button" type="button" onClick={onCopyShareLink} disabled={!shareURL} title="Скопировать share link">
          <Copy size={18} />
        </button>
      </div>
      <button className="wide-command" type="button" onClick={onCreateExport} disabled={!selectedProjectId || isExporting}>
        <FileDown size={18} />
        {isExporting ? "Создание job..." : "Создать export job"}
      </button>
      <button className="wide-command" type="button" onClick={onCreateOBJExport} disabled={!selectedProjectId || isExporting}>
        <Boxes size={18} />
        Create OBJ assembly
      </button>
      <button className="wide-command" type="button" onClick={onCreateSTLExport} disabled={!selectedProjectId || isExporting}>
        <Box size={18} />
        Create STL assembly
      </button>
      {lastExport && (
        <div className="export-status">
          <strong>{lastExport.status}</strong>
          <span>{lastExport.format.toUpperCase()} · {lastExport.id}</span>
          {lastExport.artifact.artifact_file?.url && (
            <a href={exportArtifactURL(lastExport.artifact.artifact_file.url)} target="_blank" rel="noreferrer">
              {lastExport.artifact.artifact_file.file_name}
            </a>
          )}
        </div>
      )}
      <div className="export-list">
        {exportJobs.map((job) => (
          <article className="export-item" key={job.id}>
            <div>
              <strong>{job.status}</strong>
              <span>{job.format.toUpperCase()} · {job.target}</span>
            </div>
            <small>{new Date(job.updated_at).toLocaleString()}</small>
            {job.artifact.artifact_file?.url && (
              <a href={exportArtifactURL(job.artifact.artifact_file.url)} target="_blank" rel="noreferrer">
                {job.artifact.artifact_file.file_name}
              </a>
            )}
            <code>{job.id}</code>
          </article>
        ))}
        {selectedProjectId && exportJobs.length === 0 && <p className="empty-state">Export jobs пока нет</p>}
        {!selectedProjectId && <p className="empty-state">Сначала сохраните проект</p>}
      </div>
    </div>
  );
}

type ScaleControlProps = {
  config: CharacterConfig;
  maxScale: number;
  onScaleChange: (value: number) => void;
};

type AssemblyManifestPanelProps = {
  manifest: AssemblyManifest | null;
  selectedProjectId: string | null;
  onRefresh: () => void;
};

function AssemblyManifestPanel({ manifest, selectedProjectId, onRefresh }: AssemblyManifestPanelProps) {
  return (
    <div className="manifest-panel">
      <div className="readiness-heading">
        <strong>Server assembly</strong>
        <button className="icon-button small" type="button" onClick={onRefresh} disabled={!selectedProjectId} title="Refresh assembly manifest">
          <RefreshCw size={15} />
        </button>
      </div>
      {manifest ? (
        <>
          <div className="readiness-grid">
            <span>
              Placements
              <strong>{manifest.placements.length}</strong>
            </span>
            <span>
              Character scale
              <strong>{manifest.character_scale.toFixed(2)}</strong>
            </span>
            <span>
              Generated
              <strong>{new Date(manifest.generated_at).toLocaleTimeString()}</strong>
            </span>
          </div>
          {manifest.warnings.length > 0 && (
            <small className="disabled-reason">
              <AlertTriangle size={14} />
              {manifest.warnings[0]}
            </small>
          )}
        </>
      ) : (
        <p className="empty-state">{selectedProjectId ? "Save or refresh to build server manifest" : "Save the project to build server manifest"}</p>
      )}
    </div>
  );
}

function ScaleControl({ config, maxScale, onScaleChange }: ScaleControlProps) {
  return (
    <label className="scale-control">
      <span>scale.mm</span>
      <input type="range" min="24" max={maxScale} value={config.scale.mm} onChange={(event) => onScaleChange(Number(event.target.value))} />
      <strong>
        {config.scale.mm} / {maxScale} mm
      </strong>
    </label>
  );
}

type PrintReadinessPanelProps = {
  readiness: PrintReadiness;
};

function PrintReadinessPanel({ readiness }: PrintReadinessPanelProps) {
  return (
    <div className="readiness-panel">
      <div className="readiness-heading">
        <strong>Print readiness</strong>
        <span className={readiness.supportsSTL ? "readiness-ok" : "readiness-warning"}>
          {readiness.supportsSTL ? "STL ready" : "Needs review"}
        </span>
      </div>
      <div className="readiness-grid">
        <span>
          Max scale
          <strong>{readiness.maxScale} mm</strong>
        </span>
        <span>
          Min thickness
          <strong>{readiness.minThicknessMM.toFixed(1)} mm</strong>
        </span>
        <span>
          Material zones
          <strong>{readiness.activeColorZones.length}</strong>
        </span>
        <span>
          Uploaded assets
          <strong>{readiness.uploadedAssetCount}</strong>
        </span>
      </div>
      <small>
        Constraint: {readiness.constrainedBy} · {readiness.selectedPartCount} selected parts
      </small>
    </div>
  );
}

type PanelHeadingProps = {
  title: string;
  subtitle: string;
};

function PanelHeading({ title, subtitle }: PanelHeadingProps) {
  return (
    <div className="panel-heading-block">
      <p className="eyebrow">Editor mode</p>
      <h2>{title}</h2>
      <span>{subtitle}</span>
    </div>
  );
}

function uniqueSlotsForTargets(targets: AssemblyPointTarget[]): SlotID[] {
  const seen = new Set<string>();
  const slots: SlotID[] = [];
  for (const target of targets) {
    if (target.slot === "product_target" || seen.has(target.slot)) {
      continue;
    }
    seen.add(target.slot);
    slots.push(target.slot);
  }
  return slots;
}

function stripExtension(fileName: string): string {
  return fileName.replace(/\.[^.]+$/, "");
}

function formatBytes(value: number): string {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function isSlotCategory(category: EditorCategory): category is SlotID {
  return !["assets", "assembly", "color", "projects", "export"].includes(category);
}

function categoryTitle(category: EditorCategory, catalog: Catalog | null): string {
  if (isSlotCategory(category)) {
    return catalog?.slots.find((slot) => slot.id === category)?.label ?? category;
  }
  switch (category) {
    case "color":
      return "Color";
    case "assets":
      return "Assets";
    case "assembly":
      return "Assembly";
    case "projects":
      return "Projects";
    case "export":
      return "Share / Export";
  }
}

function categoryNavLabel(category: EditorCategory, catalog: Catalog | null): string {
  if (category === "export") {
    return "Export";
  }
  return categoryTitle(category, catalog);
}

function iconForCategory(category: EditorCategory): LucideIcon {
  switch (category) {
    case "species":
      return UserRound;
    case "head":
      return Smile;
    case "body":
      return Ruler;
    case "clothing":
      return Shirt;
    case "gear":
      return Sword;
    case "pose":
      return Box;
    case "stage":
      return Box;
    case "base":
      return Circle;
    case "assets":
      return UploadCloud;
    case "assembly":
      return Boxes;
    case "color":
      return Palette;
    case "projects":
      return FolderOpen;
    case "export":
      return FileDown;
    default:
      return Box;
  }
}

function selectionFor(config: CharacterConfig, slot: SlotID): string {
  return config[slot];
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
