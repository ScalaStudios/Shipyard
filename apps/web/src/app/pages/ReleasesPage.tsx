import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Release } from "../api";
import { formatTime } from "../lib/format";

export function ReleasesPage() {
  const { org, project, setError } = useWorkspace();
  const [releases, setReleases] = useState<Release[]>([]);
  const [version, setVersion] = useState("");
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    if (!org || !project) {
      setReleases([]);
      return;
    }
    const res = await api.listReleases(org.id, project.id);
    setReleases(res.releases ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load releases"));
  }, [org?.id, project?.id]);

  async function createRelease(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.createRelease(org.id, project.id, { version, title: title || version });
      setVersion("");
      setTitle("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create release");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader title="Releases" description="Versioned release records that can be deployed to environments." />
      {!org || !project ? (
        <EmptyState title="Select a project" />
      ) : (
        <Panel
          title="Releases"
          meta={<StatusBadge status="info">{releases.length}</StatusBadge>}
          actions={
            <form className={table.formRow} onSubmit={createRelease}>
              <input className={table.input} placeholder="version" value={version} onChange={(e) => setVersion(e.target.value)} required />
              <input className={table.input} placeholder="title" value={title} onChange={(e) => setTitle(e.target.value)} />
              <Button type="submit" variant="primary" loading={busy}>
                Create release
              </Button>
            </form>
          }
        >
          {releases.length === 0 ? (
            <EmptyState title="No releases" />
          ) : (
            <DataTable>
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Title</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {releases.map((r) => (
                  <tr key={r.id}>
                    <td className="mono">{r.version}</td>
                    <td>{r.title}</td>
                    <td className={table.muted}>{formatTime(r.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </DataTable>
          )}
        </Panel>
      )}
    </div>
  );
}
