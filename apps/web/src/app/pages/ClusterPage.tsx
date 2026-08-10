import { useEffect, useState } from "react";
import { EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, SystemInfo } from "../api";
import { formatTime } from "../lib/format";

export function ClusterPage() {
  const { setError } = useWorkspace();
  const [info, setInfo] = useState<SystemInfo | null>(null);

  useEffect(() => {
    void api
      .systemInfo()
      .then(setInfo)
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load system info"));
  }, [setError]);

  return (
    <div className={table.stack}>
      <PageHeader
        title="Cluster"
        description="Control-plane node identity and capability flags. Multi-node HA remains compatible with this surface."
      />
      {!info ? (
        <EmptyState title="Loading cluster status…" />
      ) : (
        <Panel title="Control plane" meta={<StatusBadge status="success">{info.phase}</StatusBadge>}>
          <DataTable>
            <thead>
              <tr>
                <th>Field</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>Product</td>
                <td className="mono">{info.product}</td>
              </tr>
              <tr>
                <td>Phase</td>
                <td>{info.phase}</td>
              </tr>
              <tr>
                <td>Node ID</td>
                <td className="mono">{info.node_id ?? "—"}</td>
              </tr>
              <tr>
                <td>Started</td>
                <td>{formatTime(info.started_at)}</td>
              </tr>
              <tr>
                <td>OIDC</td>
                <td>{info.oidc ? "enabled" : "disabled"}</td>
              </tr>
              <tr>
                <td>Secrets</td>
                <td>{info.secrets ? "configured" : "not configured"}</td>
              </tr>
              <tr>
                <td>Open registration</td>
                <td>{info.allow_register ? "allowed" : "closed"}</td>
              </tr>
            </tbody>
          </DataTable>
        </Panel>
      )}
    </div>
  );
}
