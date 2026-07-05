import {
  AlertTriangle,
  Box,
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
  UserRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import CharacterPreview from "./CharacterPreview";
import type { Catalog, CharacterConfig, EditorCategory, ExportJob, Part, PrintReadiness, Project, SlotID } from "./types";

type EditorShellProps = {
  activeCategory: EditorCategory;
  catalog: Catalog | null;
  config: CharacterConfig;
  editorSlots: SlotID[];
  exportJobs: ExportJob[];
  isExporting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  lastExport: ExportJob | null;
  partsBySlot: Record<string, Part[]>;
  printReadiness: PrintReadiness | null;
  projectName: string;
  projects: Project[];
  selectedProject: Project | null;
  selectedProjectId: string | null;
  shareURL: string;
  status: string;
  onActiveCategoryChange: (category: EditorCategory) => void;
  onColorChange: (zone: string, value: string) => void;
  onCopyShareLink: () => void;
  onCreateExport: () => void;
  onOpenProject: (id: string) => void;
  onProjectNameChange: (value: string) => void;
  onRefreshProjects: () => void;
  onRemoveProject: (id: string) => void;
  onSave: () => void;
  onScaleChange: (value: number) => void;
  onSlotChange: (slot: SlotID, partID: string) => void;
};

export default function EditorShell({
  activeCategory,
  catalog,
  config,
  editorSlots,
  exportJobs,
  isExporting,
  isLoading,
  isSaving,
  lastExport,
  partsBySlot,
  printReadiness,
  projectName,
  projects,
  selectedProject,
  selectedProjectId,
  shareURL,
  status,
  onActiveCategoryChange,
  onColorChange,
  onCopyShareLink,
  onCreateExport,
  onOpenProject,
  onProjectNameChange,
  onRefreshProjects,
  onRemoveProject,
  onSave,
  onScaleChange,
  onSlotChange,
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
            <CharacterPreview config={config} />
          </div>
          <div className="viewport-footer">
            <span>{categoryTitle(activeCategory, catalog)}</span>
            <strong>{currentTargetPart?.label ?? config.product_target}</strong>
          </div>
        </section>

        <aside className="inspector-panel">
          {renderInspector({
            activeCategory,
            catalog,
            config,
            editorSlots,
            exportJobs,
            isExporting,
            isLoading,
            lastExport,
            partsBySlot,
            printReadiness,
            projects,
            selectedProject,
            selectedProjectId,
            shareURL,
            onActiveCategoryChange,
            onColorChange,
            onCopyShareLink,
            onCreateExport,
            onOpenProject,
            onRefreshProjects,
            onRemoveProject,
            onScaleChange,
            onSlotChange,
          })}
        </aside>
      </section>
    </main>
  );
}

type InspectorProps = Pick<
  EditorShellProps,
  | "activeCategory"
  | "catalog"
  | "config"
  | "editorSlots"
  | "exportJobs"
  | "isExporting"
  | "isLoading"
  | "lastExport"
  | "partsBySlot"
  | "printReadiness"
  | "projects"
  | "selectedProject"
  | "selectedProjectId"
  | "shareURL"
  | "onActiveCategoryChange"
  | "onColorChange"
  | "onCopyShareLink"
  | "onCreateExport"
  | "onOpenProject"
  | "onRefreshProjects"
  | "onRemoveProject"
  | "onScaleChange"
  | "onSlotChange"
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
  const categories: EditorCategory[] = [...editorSlots, "color", "projects", "export"];

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
  onSlotChange: (slot: SlotID, partID: string) => void;
};

function ExportPanel({
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
  onSlotChange,
}: ExportPanelProps) {
  return (
    <div className="inspector-content">
      <PanelHeading title="Share / Export" subtitle="Target, print readiness, share link, and export jobs" />
      <PartBrowser catalog={catalog} config={config} parts={parts} slot="product_target" onSlotChange={onSlotChange} />
      {printReadiness && <PrintReadinessPanel readiness={printReadiness} />}
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
      {lastExport && (
        <div className="export-status">
          <strong>{lastExport.status}</strong>
          <span>{lastExport.format.toUpperCase()} · {lastExport.id}</span>
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

function isSlotCategory(category: EditorCategory): category is SlotID {
  return !["color", "projects", "export"].includes(category);
}

function categoryTitle(category: EditorCategory, catalog: Catalog | null): string {
  if (isSlotCategory(category)) {
    return catalog?.slots.find((slot) => slot.id === category)?.label ?? category;
  }
  switch (category) {
    case "color":
      return "Color";
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
