import { useEffect, useState } from "react";
import { EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { Link } from "react-router-dom";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, WebhookDelivery } from "../../api";
import { formatTime, runStatus } from "../../lib/format";

export function WebhooksSettingsPage() {
  const { org, project, setError } = useWorkspace();
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);

  useEffect(() => {
    if (!org || !project) {
      setDeliveries([]);
      return;
    }
    void api
      .listWebhookDeliveries(org.id, project.id)
      .then((res) => setDeliveries(res.deliveries ?? []))
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load deliveries"));
  }, [org?.id, project?.id, setError]);

  return (
    <div className={table.stack}>
      <PageHeader
        title="Webhooks"
        description="Inbound forge deliveries. Point GitHub / Gitea / Forgejo / GitLab / etc. at /api/v1/webhooks/{provider}?connection_id=…"
      />
      {!org || !project ? (
        <EmptyState title="Select a project" />
      ) : (
        <Panel title="Recent deliveries" meta={<StatusBadge status="info">{deliveries.length}</StatusBadge>}>
          {deliveries.length === 0 ? (
            <EmptyState
              title="No deliveries yet"
              description="Create a connection under Integrations, then configure the forge webhook."
              action={<Link to="/settings/integrations">Open integrations</Link>}
            />
          ) : (
            <DataTable>
              <thead>
                <tr>
                  <th>When</th>
                  <th>Provider</th>
                  <th>Event</th>
                  <th>Status</th>
                  <th>Summary</th>
                  <th>Run</th>
                </tr>
              </thead>
              <tbody>
                {deliveries.map((d) => (
                  <tr key={d.id}>
                    <td className={table.muted}>{formatTime(d.created_at)}</td>
                    <td className="mono">{d.provider}</td>
                    <td className="mono">{d.event_type}</td>
                    <td>
                      <StatusBadge status={runStatus(d.status === "processed" ? "succeeded" : d.status === "failed" ? "failed" : "queued")}>
                        {d.status}
                      </StatusBadge>
                    </td>
                    <td>
                      {d.summary}
                      {d.error_message ? <div className={table.muted}>{d.error_message}</div> : null}
                    </td>
                    <td>{d.run_id ? <Link to={`/pipelines/runs/${d.run_id}`}>open</Link> : "—"}</td>
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
