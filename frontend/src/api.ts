import type { CharacterConfig, Project } from "./types";

const API_URL = (import.meta.env.VITE_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

type ProjectPayload = {
  name: string;
  config: CharacterConfig;
};

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
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
