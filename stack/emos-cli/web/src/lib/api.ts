// Typed fetch wrapper around the EMOS REST API. Hand-written rather than
// generated from openapi.yaml; the surface is small enough that the
// type-safety win from a generator wouldn't outweigh the build-step cost.

import { getToken, clearToken } from './auth';

export const API_BASE = '/api/v1';

// ---- Wire types ----

export interface APIError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export class ApiException extends Error {
  status: number;
  code: string;
  details?: Record<string, unknown>;
  constructor(status: number, body: APIError | string) {
    const e = typeof body === 'string' ? { code: 'unknown', message: body } : body;
    super(e.message);
    this.status = status;
    this.code = e.code;
    this.details = (typeof body !== 'string' && body.details) || undefined;
  }
}

export interface Info {
  version: string;
  name: string;            // human-friendly device identity (e.g. "epic-otter")
  started_at: string;
  uptime: string;
  hostname: string;
  platform: string;
  installed: boolean;
  mode?: string;
  ros_distro?: string;
  image_tag?: string;
  workspace_path?: string;
  pixi_project_dir?: string;
  license_present?: boolean;
  recipes_dir: string;
  logs_dir: string;
  home_dir: string;
}

export interface Capabilities {
  can_run_recipes: boolean;
  can_pull_recipes: boolean;
  has_robot_identity: boolean;
  docker_available: boolean;
  pixi_available: boolean;
}

export interface Connectivity {
  online: boolean;
  last_checked_at: string;
  target: string;
}

export interface RobotInfo {
  name?: string;
  model?: string;
  serial?: string;
  vendor?: string;
  kinematics?: string;
  sensors?: string[];
  plugin?: string;
  source: string;
}

export interface LocalRecipe {
  name: string;
  display_name?: string;
  description?: string;
  path: string;
  has_recipe_py: boolean;
  manifest?: Record<string, unknown>;
}

export interface RemoteRecipe {
  name: string;          // slug — used for /pull and /run
  display_name?: string; // human-readable label
}

export interface ExtractedTopic {
  name: string;
  msg_type: string;
  is_sensor: boolean;
}

export interface RecipeDetail extends LocalRecipe {
  topics?: ExtractedTopic[];
  sensor_topics?: ExtractedTopic[];
}

export type RunStatus = 'preparing' | 'running' | 'finished' | 'failed' | 'canceled';

export interface Run {
  id: string;
  recipe: string;
  status: RunStatus;
  started_at: string;
  finished_at?: string;
  exit_code: number;
  log_path: string;
  rmw: string;
  error?: string;
}

export type JobStatus = 'running' | 'finished' | 'failed';

export interface Job {
  id: string;
  kind: string;
  target: string;
  status: JobStatus;
  started_at: string;
  finished_at?: string;
  progress: number;
  message: string;
  error?: string;
}

// ---- Core fetch ----

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  if (!headers.has('Content-Type') && init.body) {
    headers.set('Content-Type', 'application/json');
  }
  const tok = getToken();
  if (tok) headers.set('Authorization', `Bearer ${tok}`);

  const res = await fetch(API_BASE + path, { ...init, headers });

  if (res.status === 204) return undefined as unknown as T;
  let parsed: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = text;
    }
  }

  if (!res.ok) {
    if (res.status === 401) clearToken();
    throw new ApiException(res.status, (parsed as APIError) ?? text);
  }
  return parsed as T;
}

// ---- Endpoints ----

export const api = {
  // public
  health: () => request<{ status: string; version: string; uptime: string }>('/health'),
  info: () => request<Info>('/info'),
  capabilities: () => request<Capabilities>('/capabilities'),
  connectivity: (refresh = false) =>
    request<Connectivity>(refresh ? '/connectivity?refresh=1' : '/connectivity'),

  authPair: (code: string, label?: string) =>
    request<{ token: string; expires_at: string }>('/auth/pair', {
      method: 'POST',
      body: JSON.stringify({ code, label }),
    }),
  authMe: () => request<{ authenticated: boolean }>('/auth/me'),

  // Mints a single-use ticket for SSE endpoints; the ticket is presented on
  // the `?ticket=` query string.
  authSSETicket: () =>
    request<{ ticket: string; expires_at: string }>('/auth/sse-ticket', { method: 'POST' }),

  // protected
  robot: () => request<RobotInfo>('/robot'),

  recipesLocal: () => request<LocalRecipe[]>('/recipes/local'),
  recipesRemote: () => request<RemoteRecipe[]>('/recipes/remote'),
  recipeDetail: (name: string) => request<RecipeDetail>(`/recipes/${encodeURIComponent(name)}`),
  recipeDelete: (name: string) =>
    request<void>(`/recipes/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  recipePull: (name: string) =>
    request<{ job_id: string }>(`/recipes/${encodeURIComponent(name)}/pull`, { method: 'POST' }),

  runs: () => request<Run[]>('/runs'),
  runStart: (recipe: string, opts: { rmw?: string; skip_sensor_check?: boolean } = {}) =>
    request<Run>('/runs', { method: 'POST', body: JSON.stringify({ recipe, ...opts }) }),
  runGet: (id: string) => request<Run>(`/runs/${encodeURIComponent(id)}`),
  runCancel: (id: string) =>
    request<void>(`/runs/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  jobs: () => request<Job[]>('/jobs'),
  jobGet: (id: string) => request<Job>(`/jobs/${encodeURIComponent(id)}`),
};
