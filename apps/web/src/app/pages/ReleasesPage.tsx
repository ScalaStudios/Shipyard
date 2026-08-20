import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Release } from "../api";
import { formatTime } from "../lib/format";

export function ReleasesPage() {
  const { org, project, setError } = useWorkspace();
  const [releases, setReleases] = useState<Release[]>([]);
  const [version, setVersion] = useState("");
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    if (!org || !project) {
      setReleases([]);
      setLoading(false);
      return;
    }
    try {
      const res = await api.listReleases(org.id, project.id);
      setReleases(res.releases ?? []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
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
        <EmptyState title="Select a project" description="Releases are scoped to a project." />
      ) : (
        <Panel title="Releases" meta={<StatusBadge status="info">{releases.length}</StatusBadge>}>
          <form className={form.row} onSubmit={createRelease}>
            <FormField label="Version" htmlFor="release-version">
              <input
                id="release-version"
                className={table.input}
                placeholder="1.4.0"
                value={version}
                onChange={(e) => setVersion(e.target.value)}
                required
              />
            </FormField>
            <FormField label="Title" htmlFor="release-title">
              <input
                id="release-title"
                className={table.input}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </FormField>
            <div className={form.action}>
              <Button type="submit" variant="primary" loading={busy}>
                Create release
              </Button>
            </div>
          </form>
          {loading ? (
            <TableSkeleton />
          ) : releases.length === 0 ? (
            <EmptyState title="No releases yet" description="Create one above to make a version deployable." />
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
