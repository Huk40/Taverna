import type { AssemblyPointTarget, Catalog, CharacterConfig, CompatibilityRule, ExportJob, ModelAsset, Part, Project, Slot } from "./types";

const API_URL = (import.meta.env.VITE_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

type ProjectPayload = {
  name: string;
  config: CharacterConfig;
};

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = options?.body instanceof FormData
    ? options?.headers
    : {
        "Content-Type": "application/json",
        ...options?.headers,
      };

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export function listProjects(): Promise<Project[]> {
  return request<Project[]>("/api/projects");
}

export function getProject(id: string): Promise<Project> {
  return request<Project>(`/api/projects/${id}`);
}

export function createProject(payload: ProjectPayload): Promise<Project> {
  return request<Project>("/api/projects", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function updateProject(id: string, payload: ProjectPayload): Promise<Project> {
  return request<Project>(`/api/projects/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export function deleteProject(id: string): Promise<void> {
  return request<void>(`/api/projects/${id}`, {
    method: "DELETE",
  });
}

export function getCatalog(): Promise<Catalog> {
  return request<Catalog>("/api/catalog");
}

export function getCatalogSlots(): Promise<Slot[]> {
  return request<Slot[]>("/api/catalog/slots");
}

export function getCatalogParts(slot?: string): Promise<Part[]> {
  const query = slot ? `?slot=${encodeURIComponent(slot)}` : "";
  return request<Part[]>(`/api/catalog/parts${query}`);
}

export function getCatalogRules(): Promise<CompatibilityRule[]> {
  return request<CompatibilityRule[]>("/api/catalog/rules");
}

export function createExport(projectId: string, target: string, format?: string): Promise<ExportJob> {
  return request<ExportJob>(`/api/projects/${projectId}/exports`, {
    method: "POST",
    body: JSON.stringify({ target, format }),
  });
}

export function listProjectExports(projectId: string): Promise<ExportJob[]> {
  return request<ExportJob[]>(`/api/projects/${projectId}/exports`);
}

export function getExport(id: string): Promise<ExportJob> {
  return request<ExportJob>(`/api/exports/${id}`);
}

export function listModelAssets(slot?: string): Promise<ModelAsset[]> {
  const query = slot ? `?slot=${encodeURIComponent(slot)}` : "";
  return request<ModelAsset[]>(`/api/model-assets${query}`);
}

export function uploadModelAsset(payload: {
  file: File;
  label: string;
  slot: string;
  attachmentPoint: string;
  assemblyPoints?: unknown;
}): Promise<ModelAsset> {
  const formData = new FormData();
  formData.append("file", payload.file);
  formData.append("label", payload.label);
  formData.append("slot", payload.slot);
  formData.append("attachment_point", payload.attachmentPoint);
  if (payload.assemblyPoints) {
    formData.append("assembly_points", JSON.stringify(payload.assemblyPoints));
  }

  return request<ModelAsset>("/api/model-assets", {
    method: "POST",
    body: formData,
  });
}

export function deleteModelAsset(id: string): Promise<void> {
  return request<void>(`/api/model-assets/${id}`, {
    method: "DELETE",
  });
}

export function getAssemblyPointTargets(): Promise<AssemblyPointTarget[]> {
  return request<AssemblyPointTarget[]>("/api/model-assets/assembly-points");
}

export function assetFileURL(asset: ModelAsset): string {
  if (asset.url.startsWith("http://") || asset.url.startsWith("https://")) {
    return asset.url;
  }
  return `${API_URL}${asset.url}`;
}
