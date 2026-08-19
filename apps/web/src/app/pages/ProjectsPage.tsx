import { FormEvent, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, type Project } from "../api";
import { formatTime } from "../lib/format";

export function ProjectsPage() {
  const navigate = useNavigate();
  const { orgs, projects, org, project, setOrgID, setProjectID, refreshOrgs, refreshProjects, setError } =
    useWorkspace();
  const [orgSlug, setOrgSlug] = useState("");
  const [orgName, setOrgName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [projectName, setProjectName] = useState("");
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = [...projects].sort((a, b) => a.name.localeCompare(b.name));
    if (!q) return list;
    return list.filter(
      (p) =>
        p.name.toLowerCase().includes(q) ||
        p.slug.toLowerCase().includes(q) ||
        (p.description ?? "").toLowerCase().includes(q),
    );
  }, [projects, query]);

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
      openProject(res.project);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create project");
    } finally {
      setBusy(false);
    }
  }

  function openProject(p: Project, dest: "/" | "/pipelines" = "/") {
    setProjectID(p.id);
    navigate(dest);
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Projects"
        description="Browse organization projects. Open one to work pipelines, artifacts, and releases."
        actions={
          org ? (
            <Link to="/projects/import">
              <Button type="button" variant="secondary">
                Import from forge
              </Button>
            </Link>
          ) : null
        }
      />

      <Panel
        title="Organizations"
        meta={<StatusBadge status="info">{orgs.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={createOrg}>
            <input
              className={table.input}
              placeholder="slug"
              value={orgSlug}
              onChange={(e) => setOrgSlug(e.target.value)}
              required
              pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
              aria-label="Organization slug"
            />
            <input className={table.input} placeholder="name" value={orgName} onChange={(e) => setOrgName(e.target.value)} aria-label="Organization name" />
            <Button type="submit" variant="primary" loading={busy}>
              Create org
            </Button>
          </form>
        }
      >
        {orgs.length === 0 ? (
          <EmptyState title="No organizations" description="Create an organization to begin, then import repos from Forgejo or GitHub." />
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
              pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
              aria-label="Project slug"
            />
            <input
              className={table.input}
              placeholder="name"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              disabled={!org}
              aria-label="Project name"
            />
            <Button type="submit" variant="primary" loading={busy} disabled={!org}>
              Create project
            </Button>
          </form>
        }
      >
        {!org ? (
          <EmptyState title="Select an organization" />
        ) : projects.length === 0 ? (
          <EmptyState
            title="No projects"
            description="Create one manually, or import an existing forge organization."
            action={
              <Link to="/projects/import">
                <Button type="button" variant="primary">
                  Import from forge
                </Button>
              </Link>
            }
          />
        ) : (
          <>
            <div className={table.toolbar}>
              <input
                className={table.input}
                placeholder="Search projects…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                aria-label="Search projects"
              />
              <span className={table.muted}>
                {filtered.length === projects.length
                  ? `${projects.length} projects`
                  : `${filtered.length} of ${projects.length}`}
              </span>
            </div>
            {filtered.length === 0 ? (
              <EmptyState title="No matches" description="Try a different search." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Slug</th>
                    <th>Description</th>
                    <th>Created</th>
                    <th>Open</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((p) => {
                    const selected = project?.id === p.id;
                    return (
                      <tr key={p.id}>
                        <td>
                          <button type="button" className={table.rowButton} onClick={() => openProject(p)}>
                            {p.name}
                          </button>
                          {selected ? <span className={table.muted}> · selected</span> : null}
                        </td>
                        <td className="mono">{p.slug}</td>
                        <td className={table.muted}>{p.description || "—"}</td>
                        <td className={table.muted}>{formatTime(p.created_at)}</td>
                        <td>
                          <div className={table.formRow}>
                            <Button type="button" variant="secondary" onClick={() => openProject(p, "/")}>
                              Overview
                            </Button>
                            <Button type="button" variant="ghost" onClick={() => openProject(p, "/pipelines")}>
                              Pipelines
                            </Button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </DataTable>
            )}
          </>
        )}
      </Panel>
    </div>
  );
}
