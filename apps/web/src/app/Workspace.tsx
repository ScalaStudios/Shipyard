import { FormEvent, useEffect, useState } from "react";
import { Button, StatusBadge } from "@shipyard/ui";
import { api, Organization, Pipeline, PipelineRun, Project, Runner, User } from "./api";
import styles from "./Workspace.module.css";

const defaultYAML = `pipeline:
  name: hello
jobs:
  greet:
    runner:
      os: linux
    steps:
      - name: echo
        run: echo hello from shipyard
`;

export function Workspace({
  user,
  onLogout,
}: {
  user: User;
  onLogout: () => void;
}) {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [runs, setRuns] = useState<PipelineRun[]>([]);
  const [runners, setRunners] = useState<Runner[]>([]);
  const [error, setError] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgName, setOrgName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [projectName, setProjectName] = useState("");
  const [pipelineSlug, setPipelineSlug] = useState("hello");
  const [busy, setBusy] = useState(false);

  async function refreshOrgs(preferredID?: string) {
    const res = await api.listOrgs();
    setOrgs(res.organizations);
    const next =
      res.organizations.find((o) => o.id === preferredID) ??
      res.organizations.find((o) => o.id === selectedOrg?.id) ??
      res.organizations[0] ??
      null;
    setSelectedOrg(next);
  }

  useEffect(() => {
    void refreshOrgs().catch((err) => setError(err instanceof Error ? err.message : "failed to load orgs"));
    void api.listRunners().then((res) => setRunners(res.runners)).catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!selectedOrg) {
      setProjects([]);
      setSelectedProject(null);
      return;
    }
    void api
      .listProjects(selectedOrg.id)
      .then((res) => {
        setProjects(res.projects);
        setSelectedProject((prev) => res.projects.find((p) => p.id === prev?.id) ?? res.projects[0] ?? null);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load projects"));
  }, [selectedOrg?.id]);

  useEffect(() => {
    if (!selectedOrg || !selectedProject) {
      setPipelines([]);
      setRuns([]);
      return;
    }
    void Promise.all([
      api.listPipelines(selectedOrg.id, selectedProject.id),
      api.listRuns(selectedOrg.id, selectedProject.id),
    ])
      .then(([p, r]) => {
        setPipelines(p.pipelines);
        setRuns(r.runs);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load pipelines"));
  }, [selectedOrg?.id, selectedProject?.id]);

  async function createOrg(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await api.createOrg({ slug: orgSlug, name: orgName || orgSlug });
      setOrgSlug("");
      setOrgName("");
      await refreshOrgs(res.organization.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create organization");
    } finally {
      setBusy(false);
    }
  }

  async function createProject(event: FormEvent) {
    event.preventDefault();
    if (!selectedOrg) return;
    setBusy(true);
    setError("");
    try {
      const res = await api.createProject(selectedOrg.id, { slug: projectSlug, name: projectName || projectSlug });
      setProjectSlug("");
      setProjectName("");
      const list = await api.listProjects(selectedOrg.id);
      setProjects(list.projects);
      setSelectedProject(res.project);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create project");
    } finally {
      setBusy(false);
    }
  }

  async function savePipeline(event: FormEvent) {
    event.preventDefault();
    if (!selectedOrg || !selectedProject) return;
    setBusy(true);
    setError("");
    try {
      await api.upsertPipeline(selectedOrg.id, selectedProject.id, { slug: pipelineSlug, yaml: defaultYAML });
      const list = await api.listPipelines(selectedOrg.id, selectedProject.id);
      setPipelines(list.pipelines);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save pipeline");
    } finally {
      setBusy(false);
    }
  }

  async function runPipeline(pipelineID: string) {
    if (!selectedOrg || !selectedProject) return;
    setBusy(true);
    setError("");
    try {
      await api.startRun(selectedOrg.id, selectedProject.id, pipelineID);
      const list = await api.listRuns(selectedOrg.id, selectedProject.id);
      setRuns(list.runs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to start run");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <div>
          <div className={styles.eyebrow}>Signed in</div>
          <strong>{user.display_name || user.username}</strong>
          <span className={styles.meta}>{user.email}</span>
        </div>
        <div className={styles.headerActions}>
          <StatusBadge status={runners.length ? "success" : "neutral"}>{runners.length} runners</StatusBadge>
          <Button variant="secondary" onClick={() => void onLogout()}>
            Sign out
          </Button>
        </div>
      </header>

      {error ? (
        <div className={styles.error} role="alert">
          {error}
        </div>
      ) : null}

      <section className={styles.grid}>
        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <h2>Organizations</h2>
            <StatusBadge status="info">{orgs.length} orgs</StatusBadge>
          </div>
          <ul className={styles.list}>
            {orgs.map((org) => (
              <li key={org.id}>
                <button
                  type="button"
                  className={selectedOrg?.id === org.id ? styles.listActive : styles.listItem}
                  onClick={() => setSelectedOrg(org)}
                >
                  <strong>{org.name}</strong>
                  <span>
                    {org.slug} · {org.role}
                  </span>
                </button>
              </li>
            ))}
            {orgs.length === 0 ? <li className={styles.empty}>No organizations yet.</li> : null}
          </ul>
          <form className={styles.form} onSubmit={createOrg}>
            <input placeholder="slug" value={orgSlug} onChange={(e) => setOrgSlug(e.target.value)} required pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?" />
            <input placeholder="name" value={orgName} onChange={(e) => setOrgName(e.target.value)} />
            <Button type="submit" variant="primary" loading={busy}>
              Create org
            </Button>
          </form>
        </div>

        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <h2>Projects{selectedOrg ? ` · ${selectedOrg.slug}` : ""}</h2>
            <StatusBadge status={selectedOrg ? "success" : "neutral"}>
              {selectedOrg ? `${projects.length} projects` : "Select an org"}
            </StatusBadge>
          </div>
          <ul className={styles.list}>
            {projects.map((project) => (
              <li key={project.id}>
                <button
                  type="button"
                  className={selectedProject?.id === project.id ? styles.listActive : styles.listItem}
                  onClick={() => setSelectedProject(project)}
                >
                  <strong>{project.name}</strong>
                  <span className={styles.meta}>{project.slug}</span>
                </button>
              </li>
            ))}
            {selectedOrg && projects.length === 0 ? <li className={styles.empty}>No projects in this organization.</li> : null}
          </ul>
          <form className={styles.form} onSubmit={createProject}>
            <input
              placeholder="slug"
              value={projectSlug}
              onChange={(e) => setProjectSlug(e.target.value)}
              required
              disabled={!selectedOrg}
              pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?"
            />
            <input placeholder="name" value={projectName} onChange={(e) => setProjectName(e.target.value)} disabled={!selectedOrg} />
            <Button type="submit" variant="primary" loading={busy} disabled={!selectedOrg}>
              Create project
            </Button>
          </form>
        </div>

        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <h2>Pipelines</h2>
            <StatusBadge status={selectedProject ? "info" : "neutral"}>{pipelines.length} defs</StatusBadge>
          </div>
          <ul className={styles.list}>
            {pipelines.map((p) => (
              <li key={p.id} className={styles.projectRow}>
                <div>
                  <strong>{p.name}</strong>
                  <span className={styles.meta}>{p.slug}</span>
                </div>
                <Button variant="secondary" disabled={busy} onClick={() => void runPipeline(p.id)}>
                  Run
                </Button>
              </li>
            ))}
            {selectedProject && pipelines.length === 0 ? <li className={styles.empty}>No pipelines yet.</li> : null}
          </ul>
          <form className={styles.form} onSubmit={savePipeline}>
            <input
              placeholder="pipeline slug"
              value={pipelineSlug}
              onChange={(e) => setPipelineSlug(e.target.value)}
              required
              disabled={!selectedProject}
              pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?"
            />
            <Button type="submit" variant="primary" loading={busy} disabled={!selectedProject}>
              Save hello pipeline
            </Button>
          </form>
        </div>

        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <h2>Runs</h2>
            <StatusBadge status={runs.some((r) => r.status === "failed") ? "danger" : "success"}>{runs.length} recent</StatusBadge>
          </div>
          <ul className={styles.list}>
            {runs.map((run) => (
              <li key={run.id} className={styles.projectRow}>
                <strong>#{run.number}</strong>
                <StatusBadge
                  status={
                    run.status === "succeeded" ? "success" : run.status === "failed" ? "danger" : run.status === "running" ? "info" : "neutral"
                  }
                >
                  {run.status}
                </StatusBadge>
              </li>
            ))}
            {selectedProject && runs.length === 0 ? <li className={styles.empty}>No runs yet.</li> : null}
          </ul>
        </div>
      </section>
    </div>
  );
}
