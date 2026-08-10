import { FormEvent, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api } from "../api";
import { formatTime } from "../lib/format";

export function ProjectsPage() {
  const { orgs, projects, org, setOrgID, setProjectID, refreshOrgs, refreshProjects, setError } = useWorkspace();
  const [orgSlug, setOrgSlug] = useState("");
  const [orgName, setOrgName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [projectName, setProjectName] = useState("");
  const [busy, setBusy] = useState(false);

  async function createOrg(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      const res = await api.createOrg({ slug: orgSlug, name: orgName || orgSlug });
      setOrgSlug("");
      setOrgName("");
      await refreshOrgs();
      setOrgID(res.organization.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create organization");
    } finally {
      setBusy(false);
    }
  }

  async function createProject(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      const res = await api.createProject(org.id, { slug: projectSlug, name: projectName || projectSlug });
      setProjectSlug("");
      setProjectName("");
      await refreshProjects();
      setProjectID(res.project.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create project");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Projects"
        description="Organizations own projects. Select a project in the topbar to scope pipelines, artifacts, and releases."
      />

      <Panel
        title="Organizations"
        meta={<StatusBadge status="info">{orgs.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={createOrg}>
            <input className={table.input} placeholder="slug" value={orgSlug} onChange={(e) => setOrgSlug(e.target.value)} required pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?" />
            <input className={table.input} placeholder="name" value={orgName} onChange={(e) => setOrgName(e.target.value)} />
            <Button type="submit" variant="primary" loading={busy}>
              Create org
            </Button>
          </form>
        }
      >
        {orgs.length === 0 ? (
          <EmptyState title="No organizations" description="Create an organization to begin." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Role</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {orgs.map((o) => (
                <tr key={o.id}>
                  <td>
                    <button type="button" className={table.rowButton} onClick={() => setOrgID(o.id)}>
                      {o.name}
                    </button>
                    {org?.id === o.id ? <span className={table.muted}> · selected</span> : null}
                  </td>
                  <td className="mono">{o.slug}</td>
                  <td>{o.role ?? "—"}</td>
                  <td className={table.muted}>{formatTime(o.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel
        title={org ? `Projects · ${org.slug}` : "Projects"}
        meta={<StatusBadge status={org ? "success" : "neutral"}>{projects.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={createProject}>
            <input
              className={table.input}
              placeholder="slug"
              value={projectSlug}
              onChange={(e) => setProjectSlug(e.target.value)}
              required
              disabled={!org}
              pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?"
            />
            <input className={table.input} placeholder="name" value={projectName} onChange={(e) => setProjectName(e.target.value)} disabled={!org} />
            <Button type="submit" variant="primary" loading={busy} disabled={!org}>
              Create project
            </Button>
          </form>
        }
      >
        {!org ? (
          <EmptyState title="Select an organization" />
        ) : projects.length === 0 ? (
          <EmptyState title="No projects" description="Create a project in this organization." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => (
                <tr key={p.id}>
                  <td>
                    <button type="button" className={table.rowButton} onClick={() => setProjectID(p.id)}>
                      {p.name}
                    </button>
                  </td>
                  <td className="mono">{p.slug}</td>
                  <td className={table.muted}>{formatTime(p.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
