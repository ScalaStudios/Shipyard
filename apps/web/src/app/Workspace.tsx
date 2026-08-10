import { FormEvent, useEffect, useState } from "react";
import { Button, StatusBadge } from "@shipyard/ui";
import { api, Organization, Project, User } from "./api";
import styles from "./Workspace.module.css";

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
  const [error, setError] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgName, setOrgName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [projectName, setProjectName] = useState("");
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
  }, []);

  useEffect(() => {
    if (!selectedOrg) {
      setProjects([]);
      return;
    }
    void api
      .listProjects(selectedOrg.id)
      .then((res) => setProjects(res.projects))
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load projects"));
  }, [selectedOrg?.id]);

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
      await api.createProject(selectedOrg.id, { slug: projectSlug, name: projectName || projectSlug });
      setProjectSlug("");
      setProjectName("");
      const res = await api.listProjects(selectedOrg.id);
      setProjects(res.projects);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create project");
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
        <Button variant="secondary" onClick={() => void onLogout()}>
          Sign out
        </Button>
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
                  <span>{org.slug} · {org.role}</span>
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
              <li key={project.id} className={styles.projectRow}>
                <strong>{project.name}</strong>
                <span className={styles.meta}>{project.slug}</span>
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
            <input
              placeholder="name"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              disabled={!selectedOrg}
            />
            <Button type="submit" variant="primary" loading={busy} disabled={!selectedOrg}>
              Create project
            </Button>
          </form>
        </div>
      </section>
    </div>
  );
}
