import { FormEvent, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, type Project } from "../api";
import { formatTime, slugify } from "../lib/format";

export function ProjectsPage() {
  const navigate = useNavigate();
  const { orgs, projects, org, project, loading, setOrgID, setProjectID, refreshOrgs, refreshProjects, setError } =
    useWorkspace();
  const [orgName, setOrgName] = useState("");
  const [projectName, setProjectName] = useState("");
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const orgURLName = slugify(orgName);
  const projectURLName = slugify(projectName);

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
      const res = await api.createOrg({ slug: orgURLName, name: orgName });
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
      const res = await api.createProject(org.id, { slug: projectURLName, name: projectName });
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
                Import repositories
              </Button>
            </Link>
          ) : null
        }
      />

      <Panel title="Organizations" meta={<StatusBadge status="info">{orgs.length}</StatusBadge>}>
        <form className={form.row} onSubmit={createOrg}>
          <FormField
            label="Name"
            htmlFor="org-name"
            hint={orgURLName ? <>URL name: <span className="mono">{orgURLName}</span></> : null}
          >
            <input
              id="org-name"
              className={table.input}
              value={orgName}
              onChange={(e) => setOrgName(e.target.value)}
              required
            />
          </FormField>
          <div className={form.action}>
            <Button type="submit" variant="primary" loading={busy} disabled={!orgURLName}>
              Create organization
            </Button>
          </div>
        </form>
        {loading ? (
          <TableSkeleton />
        ) : orgs.length === 0 ? (
          <EmptyState
            title="No organizations yet"
            description="Create an organization above, then import repositories from Forgejo or GitHub into it."
          />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>URL name</th>
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
                  <td className={`mono ${table.muted}`}>{o.slug}</td>
                  <td>{o.role ?? "—"}</td>
                  <td className={table.muted}>{formatTime(o.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel
        title={org ? `Projects · ${org.name}` : "Projects"}
        meta={<StatusBadge status={org ? "success" : "neutral"}>{projects.length}</StatusBadge>}
      >
        <form className={form.row} onSubmit={createProject}>
          <FormField
            label="Name"
            htmlFor="project-name"
            hint={projectURLName ? <>URL name: <span className="mono">{projectURLName}</span></> : null}
          >
            <input
              id="project-name"
              className={table.input}
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              required
              disabled={!org}
            />
          </FormField>
          <div className={form.action}>
            <Button type="submit" variant="primary" loading={busy} disabled={!org || !projectURLName}>
              Create project
            </Button>
          </div>
        </form>
        {loading ? (
          <TableSkeleton />
        ) : !org ? (
          <EmptyState
            title="Select an organization"
            description="Pick an organization above to see and create its projects."
          />
        ) : projects.length === 0 ? (
          <EmptyState
            title="No projects yet"
            description="Create one above, or import every repository from a connected forge organization in one pass."
            action={
              <Link to="/projects/import">
                <Button type="button" variant="primary">
                  Import repositories
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
              <EmptyState title="No matching projects" description="No project name or description matches that search." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>URL name</th>
                    <th>Description</th>
                    <th>Created</th>
                    <th className={table.actionCol}>Open</th>
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
                        <td className={`mono ${table.muted}`}>{p.slug}</td>
                        <td className={table.muted}>{p.description || "—"}</td>
                        <td className={table.muted}>{formatTime(p.created_at)}</td>
                        <td className={table.actionCol}>
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
