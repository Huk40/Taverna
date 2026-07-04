import { FormEvent, useEffect, useMemo, useState } from "react";
import { FolderOpen, RefreshCw, Save, Trash2 } from "lucide-react";
import CharacterPreview from "./CharacterPreview";
import { createProject, deleteProject, getProject, listProjects, updateProject } from "./api";
import type { CharacterConfig, Project } from "./types";

const defaultConfig: CharacterConfig = {
  body: "human_male_01",
  head: "default_head_01",
  hair: "short_hair_01",
  armor: "basic_armor_01",
  weapon: "sword_01",
  pose: "idle_01",
  base: "round_base_32mm",
  scale_mm: 32,
  colors: {
    armor: "#444444",
    hair: "#3b2416",
    cape: "#8B0000",
  },
};

const options = {
  body: [
    ["human_male_01", "Human male"],
    ["human_female_01", "Human female"],
    ["dwarf_01", "Dwarf"],
    ["orc_01", "Orc"],
  ],
  head: [
    ["default_head_01", "Default"],
    ["square_head_01", "Square"],
    ["hero_head_01", "Hero"],
  ],
  hair: [
    ["short_hair_01", "Short"],
    ["long_hair_01", "Long"],
    ["bald_01", "Bald"],
  ],
  armor: [
    ["basic_armor_01", "Basic armor"],
    ["leather_armor_01", "Leather"],
    ["robe_01", "Robe"],
    ["cape_armor_01", "Cape armor"],
  ],
  weapon: [
    ["sword_01", "Sword"],
    ["axe_01", "Axe"],
    ["staff_01", "Staff"],
    ["none_01", "None"],
  ],
  pose: [
    ["idle_01", "Idle"],
    ["guard_01", "Guard"],
    ["victory_01", "Victory"],
  ],
  base: [
    ["round_base_32mm", "Round 32 mm"],
    ["round_base_40mm", "Round 40 mm"],
  ],
};

export default function App() {
  const [projectName, setProjectName] = useState("Taverna hero");
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [config, setConfig] = useState<CharacterConfig>(defaultConfig);
  const [projects, setProjects] = useState<Project[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [status, setStatus] = useState("Готово");

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  );

  useEffect(() => {
    void refreshProjects();
  }, []);

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
    setIsSaving(true);
    try {
      const payload = { name: projectName, config };
      const saved = selectedProjectId
        ? await updateProject(selectedProjectId, payload)
        : await createProject(payload);
      setSelectedProjectId(saved.id);
      setProjectName(saved.name);
      setConfig(saved.config);
      await refreshProjects();
      setStatus("Проект сохранен");
    } catch (error) {
      setStatus(`Ошибка сохранения: ${getErrorMessage(error)}`);
    } finally {
      setIsSaving(false);
    }
  }

  async function openProject(id: string) {
    setIsLoading(true);
    try {
      const project = await getProject(id);
      setSelectedProjectId(project.id);
      setProjectName(project.name);
      setConfig({ ...defaultConfig, ...project.config, colors: { ...defaultConfig.colors, ...project.config.colors } });
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
        setConfig(defaultConfig);
      }
      await refreshProjects();
      setStatus("Проект удален");
    } catch (error) {
      setStatus(`Ошибка удаления: ${getErrorMessage(error)}`);
    } finally {
      setIsLoading(false);
    }
  }

  function updateField<K extends keyof CharacterConfig>(field: K, value: CharacterConfig[K]) {
    setConfig((current) => ({ ...current, [field]: value }));
  }

  function updateColor(field: keyof CharacterConfig["colors"], value: string) {
    setConfig((current) => ({ ...current, colors: { ...current.colors, [field]: value } }));
  }

  return (
    <main className="app-shell">
      <section className="workspace">
        <div className="preview-panel">
          <div className="preview-toolbar">
            <div>
              <p className="eyebrow">Taverna MVP</p>
              <h1>{projectName}</h1>
            </div>
            <div className="status-pill">{status}</div>
          </div>
          <div className="preview-stage">
            <CharacterPreview config={config} />
          </div>
        </div>

        <aside className="control-panel">
          <form onSubmit={saveProject} className="editor-form">
            <label className="field wide">
              <span>name</span>
              <input value={projectName} onChange={(event) => setProjectName(event.target.value)} />
            </label>

            <SelectField label="body" value={config.body} choices={options.body} onChange={(value) => updateField("body", value)} />
            <SelectField label="head" value={config.head} choices={options.head} onChange={(value) => updateField("head", value)} />
            <SelectField label="hair" value={config.hair} choices={options.hair} onChange={(value) => updateField("hair", value)} />
            <SelectField label="armor" value={config.armor} choices={options.armor} onChange={(value) => updateField("armor", value)} />
            <SelectField label="weapon" value={config.weapon} choices={options.weapon} onChange={(value) => updateField("weapon", value)} />
            <SelectField label="pose" value={config.pose} choices={options.pose} onChange={(value) => updateField("pose", value)} />
            <SelectField label="base" value={config.base} choices={options.base} onChange={(value) => updateField("base", value)} />

            <label className="field wide">
              <span>scale_mm</span>
              <input
                type="range"
                min="24"
                max="54"
                value={config.scale_mm}
                onChange={(event) => updateField("scale_mm", Number(event.target.value))}
              />
              <strong>{config.scale_mm} mm</strong>
            </label>

            <div className="color-grid wide">
              <ColorField label="armor" value={config.colors.armor} onChange={(value) => updateColor("armor", value)} />
              <ColorField label="hair" value={config.colors.hair} onChange={(value) => updateColor("hair", value)} />
              <ColorField label="cape" value={config.colors.cape} onChange={(value) => updateColor("cape", value)} />
            </div>

            <button className="primary-button wide" type="submit" disabled={isSaving}>
              <Save size={18} />
              {isSaving ? "Сохранение..." : "Сохранить проект"}
            </button>
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
        </aside>
      </section>
    </main>
  );
}

type SelectFieldProps = {
  label: string;
  value: string;
  choices: string[][];
  onChange: (value: string) => void;
};

function SelectField({ label, value, choices, onChange }: SelectFieldProps) {
  return (
    <label className="field">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {choices.map(([optionValue, text]) => (
          <option key={optionValue} value={optionValue}>
            {text}
          </option>
        ))}
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

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "unknown error";
}
