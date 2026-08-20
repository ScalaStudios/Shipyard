import { useEffect, useState } from "react";
import { EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Artifact } from "../api";
import { formatBytes } from "../lib/format";

export function ArtifactsPage() {
  const { org, project, setError } = useWorkspace();
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!org || !project) {
      setArtifacts([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    void api
      .listArtifacts(org.id, project.id)
      .then((res) => setArtifacts(res.artifacts ?? []))
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load artifacts"))
      .finally(() => setLoading(false));
  }, [org?.id, project?.id, setError]);

  return (
    <div className={table.stack}>
      <PageHeader
        title="Artifacts"
        description="Build outputs retained from pipeline jobs — browse by name and content digest."
      />
      {!org || !project ? (
        <EmptyState title="Select a project" description="Artifacts are scoped to a project." />
      ) : (
        <Panel title="Artifacts" meta={<StatusBadge status="info">{artifacts.length}</StatusBadge>}>
          {loading ? (
            <TableSkeleton />
          ) : artifacts.length === 0 ? (
            <EmptyState title="No artifacts" description="Artifacts appear here once a pipeline job uploads them." />
          ) : (
            <DataTable>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Digest</th>
                  <th>Size</th>
                  <th className={table.actionCol} />
                </tr>
              </thead>
              <tbody>
                {artifacts.map((a) => (
                  <tr key={a.id}>
                    <td>{a.name}</td>
                    <td className="mono">{a.digest}</td>
                    <td>{formatBytes(a.size_bytes)}</td>
                    <td className={table.actionCol}>
                      <a href={`/api/v1/orgs/${org.id}/projects/${project.id}/artifacts/${a.id}/download`}>Download</a>
                    </td>
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
