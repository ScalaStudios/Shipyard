export type User = {
  id: string;
  username: string;
  email: string;
  display_name: string;
  is_active: boolean;
  created_at: string;
};

export type OIDCProvider = {
  name: string;
  kind: string;
};

export type SystemInfo = {
  product: string;
  phase: string;
  allow_register: boolean;
  started_at?: string;
  node_id?: string;
  oidc?: boolean;
  secrets?: boolean;
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

export type Member = {
  user_id: string;
  username: string;
  display_name: string;
  role: string;
  created_at: string;
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
  project_id?: string;
  number: number;
  status: string;
  trigger_type?: string;
  git_ref?: string;
  git_sha?: string;
  error_message?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
};

export type Job = {
  id: string;
  run_id: string;
  name: string;
  status: string;
  needs?: string[];
  runner_labels?: string[];
  error_message?: string;
  created_at?: string;
  started_at?: string;
  finished_at?: string;
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
  capabilities?: string[];
  last_heartbeat_at?: string;
  drained?: boolean;
  created_at?: string;
};

export type Artifact = {
  id: string;
  name: string;
  digest: string;
  size_bytes: number;
};

export type PackageRepo = {
  id: string;
  name: string;
  format: string;
};

export type PackageVersion = {
  id: string;
  version: string;
  digest?: string;
  created_at?: string;
};

export type Release = {
  id: string;
  version: string;
  title: string;
  notes?: string;
  run_id?: string;
  created_at?: string;
};

export type OCIRepo = {
  id: string;
  name: string;
};

export type Environment = {
  id: string;
  project_id: string;
  slug: string;
  name: string;
  created_at: string;
};

export type Deployment = {
  id: string;
  project_id: string;
  environment_id: string;
  release_id: string;
  status: string;
  created_at: string;
  finished_at?: string;
};

export type SecretMeta = {
  id: string;
  name: string;
  scope: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...(init?.headers as Record<string, string> | undefined),
  };
  if (init?.body && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(typeof body.error === "string" ? body.error : `request failed (${res.status})`);
  }
  return body as T;
}

export const api = {
  systemInfo: () => request<SystemInfo>("/api/v1/system/info"),
  oidcProviders: () => request<{ providers: OIDCProvider[] }>("/api/v1/auth/oidc/providers"),
  me: () => request<{ user: User }>("/api/v1/me"),
  register: (payload: { username: string; email: string; display_name?: string; password: string }) =>
    request<{ user: User }>("/api/v1/auth/register", { method: "POST", body: JSON.stringify(payload) }),
  login: (payload: { login: string; password: string }) =>
    request<{ user: User }>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(payload) }),
  logout: () => request<{ status: string }>("/api/v1/auth/logout", { method: "POST", body: "{}" }),
  createToken: (payload: { name: string; ttl?: string }) =>
    request<{ token: string; prefix: string; expires_at?: string }>("/api/v1/me/tokens", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  listOrgs: () => request<{ organizations: Organization[] }>("/api/v1/orgs"),
  createOrg: (payload: { slug: string; name: string; description?: string }) =>
    request<{ organization: Organization }>("/api/v1/orgs", { method: "POST", body: JSON.stringify(payload) }),
  listMembers: (orgID: string) => request<{ members: Member[] }>(`/api/v1/orgs/${orgID}/members`),
  addMember: (orgID: string, payload: { login: string; role: string }) =>
    request<{ member: Member }>(`/api/v1/orgs/${orgID}/members`, { method: "POST", body: JSON.stringify(payload) }),

  listProjects: (orgID: string) => request<{ projects: Project[] }>(`/api/v1/orgs/${orgID}/projects`),
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
  cancelRun: (orgID: string, projectID: string, runID: string) =>
    request<{ run: PipelineRun }>(`/api/v1/orgs/${orgID}/projects/${projectID}/runs/${runID}/cancel`, {
      method: "POST",
      body: "{}",
    }),
  jobLogs: (orgID: string, projectID: string, jobID: string) =>
    request<{ logs: LogLine[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/jobs/${jobID}/logs`),

  listRunners: () => request<{ runners: Runner[] }>("/api/v1/runners"),
  createRunnerRegToken: (payload?: { organization_id?: string; ttl?: string }) =>
    request<{ token: string; expires_at: string }>("/api/v1/runners/registration-tokens", {
      method: "POST",
      body: JSON.stringify(payload ?? {}),
    }),

  listArtifacts: (orgID: string, projectID: string) =>
    request<{ artifacts: Artifact[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/artifacts`),
  listPackages: (orgID: string, projectID: string) =>
    request<{ repositories: PackageRepo[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/packages`),
  createPackageRepo: (orgID: string, projectID: string, payload: { name: string; format: string }) =>
    request<{ repository: PackageRepo }>(`/api/v1/orgs/${orgID}/projects/${projectID}/packages`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  listPackageVersions: (orgID: string, projectID: string, repoID: string) =>
    request<{ versions: PackageVersion[] }>(
      `/api/v1/orgs/${orgID}/projects/${projectID}/packages/${repoID}/versions`,
    ),
  listOCI: (orgID: string, projectID: string) =>
    request<{ repositories: OCIRepo[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/oci`),
  listOCITags: (orgID: string, projectID: string, name: string) =>
    request<{ tags: string[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/oci/${encodeURIComponent(name)}/tags`),

  listReleases: (orgID: string, projectID: string) =>
    request<{ releases: Release[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/releases`),
  createRelease: (orgID: string, projectID: string, payload: { version: string; title?: string; notes?: string; run_id?: string }) =>
    request<{ release: Release }>(`/api/v1/orgs/${orgID}/projects/${projectID}/releases`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  listEnvironments: (orgID: string, projectID: string) =>
    request<{ environments: Environment[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/environments`),
  createEnvironment: (orgID: string, projectID: string, payload: { slug: string; name: string }) =>
    request<{ environment: Environment }>(`/api/v1/orgs/${orgID}/projects/${projectID}/environments`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  listDeployments: (orgID: string, projectID: string) =>
    request<{ deployments: Deployment[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/deployments`),
  createDeployment: (orgID: string, projectID: string, payload: { environment_id: string; release_id: string }) =>
    request<{ deployment: Deployment }>(`/api/v1/orgs/${orgID}/projects/${projectID}/deployments`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  listSecrets: (params?: { organization_id?: string; project_id?: string }) => {
    const q = new URLSearchParams();
    if (params?.organization_id) q.set("organization_id", params.organization_id);
    if (params?.project_id) q.set("project_id", params.project_id);
    const qs = q.toString();
    return request<{ secrets: SecretMeta[] }>(`/api/v1/secrets${qs ? `?${qs}` : ""}`);
  },
  createSecret: (payload: {
    name: string;
    value: string;
    organization_id?: string;
    project_id?: string;
    environment_id?: string;
  }) => request<{ secret: SecretMeta }>("/api/v1/secrets", { method: "POST", body: JSON.stringify(payload) }),
};
