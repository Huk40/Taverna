import type {
  AssemblyManifest,
  AssemblyPointTarget,
  Catalog,
  CharacterConfig,
  CompatibilityRule,
  ExportJob,
  ModelAsset,
  ModelAssetAssemblyPoint,
  Part,
  Project,
  Slot,
  User,
} from "./types";

const API_URL = (import.meta.env.VITE_API_URL ?? "").replace(/\/$/, "");

export class APIError extends Error {
  readonly status: number;
  readonly payload: unknown;

  constructor(status: number, message: string, payload: unknown) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.payload = payload;
  }
}

type ProjectPayload = {
  name: string;
  config: CharacterConfig;
};

async function request<T>(path: string, options?: RequestInit, pagination?: { next: number | null }): Promise<T> {
  const headers = new Headers(options?.headers);
  headers.set("X-Taverna-Request", "1");
  if (!(options?.body instanceof FormData)) headers.set("Content-Type", "application/json");

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });

  if (!response.ok) {
    if (response.status === 401 && !path.startsWith("/api/auth/")) window.dispatchEvent(new Event("auth-expired"));
    const text = await response.text();
    const payload = parseErrorPayload(text);
    const message = errorMessageFromPayload(payload) ?? (text.trim() || `Request failed with status ${response.status}`);
    throw new APIError(response.status, message, payload);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  if (pagination) { const value = response.headers.get("X-Next-Offset"); pagination.next = value === null ? null : Number(value); }
  return response.json() as Promise<T>;
}

function parseErrorPayload(text: string): unknown {
  if (!text.trim()) {
    return null;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

function errorMessageFromPayload(payload: unknown): string | null {
  if (typeof payload !== "object" || payload === null || !("error" in payload)) {
    return null;
  }
  const message = (payload as { error?: unknown }).error;
  return typeof message === "string" && message.trim() ? message : null;
}

async function listAll<T>(path: string): Promise<T[]> {
  const items: T[] = [];
  let offset = 0;
  for (;;) {
    const page: { next: number | null } = { next: null };
    items.push(...await request<T[]>(path + (path.includes("?") ? "&" : "?") + `limit=100&offset=${offset}`, undefined, page));
    if (page.next === null) return items;
    if (!Number.isSafeInteger(page.next) || page.next <= offset) throw new Error("Некорректная страница ответа сервера");
    offset = page.next;
  }
}

export function listProjects(): Promise<Project[]> {
  return listAll<Project>("/api/projects");
}

export type AuthResponse = {
  user: User;
  email_delivery_configured?: boolean;
};

export function register(payload: { email: string; displayName: string; password: string }): Promise<AuthResponse> {
  return request<AuthResponse>("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email: payload.email, display_name: payload.displayName, password: payload.password }),
  });
}

export function login(payload: { email: string; password: string }): Promise<AuthResponse> {
  return request<AuthResponse>("/api/auth/login", { method: "POST", body: JSON.stringify(payload) });
}

export function logout(): Promise<void> {
  return request<void>("/api/auth/logout", { method: "POST" });
}

export function getCurrentUser(): Promise<AuthResponse> {
  return request<AuthResponse>("/api/auth/me");
}

export function requestPasswordReset(email: string): Promise<void> {
  return request<void>("/api/auth/forgot-password", { method: "POST", body: JSON.stringify({ email }) });
}

export function verifyEmail(token: string): Promise<void> {
  return request<void>("/api/auth/verify-email", { method: "POST", body: JSON.stringify({ token }) });
}

export function resetPassword(token: string, password: string): Promise<void> {
  return request<void>("/api/auth/reset-password", { method: "POST", body: JSON.stringify({ token, password }) });
}

export const logoutAll = () => request<void>("/api/auth/logout-all", {method:"POST"});
export const resendVerification = () => request<void>("/api/auth/resend-verification", {method:"POST"});

export function getProject(id: string): Promise<Project> {
  return request<Project>(`/api/projects/${id}`);
}

export function getProjectAssembly(id: string): Promise<AssemblyManifest> {
  return request<AssemblyManifest>(`/api/projects/${id}/assembly`);
}

export function previewAssembly(config: CharacterConfig): Promise<AssemblyManifest> {
  return request<AssemblyManifest>("/api/assembly/preview", {
    method: "POST",
    body: JSON.stringify({ config }),
  });
}

export async function downloadSTL(config: CharacterConfig): Promise<Blob> {
  const response = await fetch(`${API_URL}/api/exports/stl`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "model/stl" },
    body: JSON.stringify({ config }),
    credentials: "include",
  });
  if (!response.ok) {
    const payload = parseErrorPayload(await response.text());
    throw new APIError(response.status, errorMessageFromPayload(payload) ?? "Не удалось подготовить STL. Попробуйте ещё раз.", payload);
  }
  const file = await response.blob();
  if (!response.headers.get("Content-Type")?.includes("model/stl") || file.size < 134) {
    throw new Error("Сервер не вернул STL с геометрией модели.");
  }
  return file;
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
  return listAll<ExportJob>(`/api/projects/${projectId}/exports`);
}

export function getExport(id: string): Promise<ExportJob> {
  return request<ExportJob>(`/api/exports/${id}`);
}

export function listModelAssets(slot?: string): Promise<ModelAsset[]> {
  const query = slot ? `?slot=${encodeURIComponent(slot)}` : "";
  return listAll<ModelAsset>(`/api/model-assets${query}`);
}

export function uploadModelAsset(payload: {
  file: File;
  label: string;
  slot: string;
  attachmentPoint: string;
  assemblyPoints?: ModelAssetAssemblyPoint[];
}): Promise<ModelAsset> {
  const formData = new FormData();
  formData.append("file", payload.file);
  formData.append("label", payload.label);
  formData.append("slot", payload.slot);
  formData.append("attachment_point", payload.attachmentPoint);
  if (payload.assemblyPoints?.length) {
    formData.append("assembly_points", JSON.stringify(payload.assemblyPoints));
  }

  return request<ModelAsset>("/api/model-assets", {
    method: "POST",
    body: formData,
  });
}

export function updateModelAsset(id: string, payload: {
  label: string;
  assemblyPoints: ModelAssetAssemblyPoint[];
}): Promise<ModelAsset> {
  return request<ModelAsset>(`/api/model-assets/${id}`, {
    method: "PUT",
    body: JSON.stringify({
      label: payload.label,
      assembly_points: payload.assemblyPoints,
    }),
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
  return absoluteAPIURL(asset.url);
}

export function exportArtifactURL(url: string): string {
  return absoluteAPIURL(url);
}

function absoluteAPIURL(url: string): string {
  if (url.startsWith("http://") || url.startsWith("https://")) {
    return url;
  }
  return `${API_URL}${url}`;
}
