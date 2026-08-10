export type User = {
  id: string;
  username: string;
  email: string;
  display_name: string;
  is_active: boolean;
  created_at: string;
};

export type Organization = {
  id: string;
  slug: string;
  name: string;
  description: string;
  created_at: string;
  role?: string;
};

export type Project = {
  id: string;
  organization_id: string;
  slug: string;
  name: string;
  description: string;
  created_at: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(typeof body.error === "string" ? body.error : `request failed (${res.status})`);
  }
  return body as T;
}

export const api = {
  systemInfo: () => request<{ product: string; phase: string; allow_register: boolean }>("/api/v1/system/info"),
  me: () => request<{ user: User }>("/api/v1/me"),
  register: (payload: { username: string; email: string; display_name?: string; password: string }) =>
    request<{ user: User }>("/api/v1/auth/register", { method: "POST", body: JSON.stringify(payload) }),
  login: (payload: { login: string; password: string }) =>
    request<{ user: User }>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(payload) }),
  logout: () => request<{ status: string }>("/api/v1/auth/logout", { method: "POST", body: "{}" }),
  listOrgs: () => request<{ organizations: Organization[] }>("/api/v1/orgs"),
  createOrg: (payload: { slug: string; name: string; description?: string }) =>
    request<{ organization: Organization }>("/api/v1/orgs", { method: "POST", body: JSON.stringify(payload) }),
  listProjects: (orgID: string) =>
    request<{ projects: Project[] }>(`/api/v1/orgs/${orgID}/projects`),
  createProject: (orgID: string, payload: { slug: string; name: string; description?: string }) =>
    request<{ project: Project }>(`/api/v1/orgs/${orgID}/projects`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  listPipelines: (orgID: string, projectID: string) =>
    request<{ pipelines: Pipeline[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/pipelines`),
  upsertPipeline: (orgID: string, projectID: string, payload: { slug: string; yaml: string }) =>
    request<{ pipeline: Pipeline }>(`/api/v1/orgs/${orgID}/projects/${projectID}/pipelines`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  listRuns: (orgID: string, projectID: string) =>
    request<{ runs: PipelineRun[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/runs`),
  startRun: (orgID: string, projectID: string, pipelineID: string) =>
    request<{ run: PipelineRun }>(`/api/v1/orgs/${orgID}/projects/${projectID}/pipelines/${pipelineID}/runs`, {
      method: "POST",
      body: "{}",
    }),
  getRun: (orgID: string, projectID: string, runID: string) =>
    request<{ run: PipelineRun; jobs: Job[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/runs/${runID}`),
  jobLogs: (orgID: string, projectID: string, jobID: string) =>
    request<{ logs: LogLine[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/jobs/${jobID}/logs`),
  listRunners: () => request<{ runners: Runner[] }>("/api/v1/runners"),
};

export type Pipeline = {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  yaml_source: string;
  created_at: string;
};

export type PipelineRun = {
  id: string;
  pipeline_id: string;
  number: number;
  status: string;
  created_at: string;
};

export type Job = {
  id: string;
  run_id: string;
  name: string;
  status: string;
};

export type LogLine = {
  seq?: number;
  stream?: string;
  line?: string;
  created_at?: string;
};

export type Runner = {
  id: string;
  name: string;
  status: string;
  labels: string[];
};
