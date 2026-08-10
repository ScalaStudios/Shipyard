import { useEffect, useState } from "react";
import { EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Artifact } from "../api";
import { formatBytes } from "../lib/format";

export function ArtifactsPage() {
  const { org, project, setError } = useWorkspace();
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);

  useEffect(() => {
    if (!org || !project) {
      setArtifacts([]);
      return;
    }
    void api
      .listArtifacts(org.id, project.id)
      .then((res) => setArtifacts(res.artifacts ?? []))
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load artifacts"));
  }, [org?.id, project?.id, setError]);

  return (
    <div className={table.stack}>
      <PageHeader
        title="Artifacts"
        description="Build outputs retained from pipeline jobs — browse by name and content digest."
      />
      {!org || !project ? (
        <EmptyState title="Select a project" />
      ) : (
        <Panel title="Artifacts" meta={<StatusBadge status="info">{artifacts.length}</StatusBadge>}>
          {artifacts.length === 0 ? (
            <EmptyState title="No artifacts" description="Artifacts appear after jobs upload them." />
          ) : (
            <DataTable>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Digest</th>
                  <th>Size</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {artifacts.map((a) => (
                  <tr key={a.id}>
                    <td>{a.name}</td>
                    <td className="mono">{a.digest}</td>
                    <td>{formatBytes(a.size_bytes)}</td>
                    <td>
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
