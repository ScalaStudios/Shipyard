import { useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Runner } from "../api";
import { formatTime, runStatus } from "../lib/format";

export function RunnersPage() {
  const { org, setError } = useWorkspace();
  const [runners, setRunners] = useState<Runner[]>([]);
  const [token, setToken] = useState("");
  const [expires, setExpires] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    const res = await api.listRunners();
    setRunners(res.runners ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load runners"));
  }, [setError]);

  async function createToken() {
    setBusy(true);
    try {
      const res = await api.createRunnerRegToken(org ? { organization_id: org.id } : {});
      setToken(res.token);
      setExpires(res.expires_at);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create registration token");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Runners"
        description="Registered execution agents. Create a registration token to attach a new runner."
        actions={
          <Button variant="primary" loading={busy} onClick={() => void createToken()}>
            Create registration token
          </Button>
        }
      />

      {token ? (
        <Panel title="Registration token">
          <div className={table.toolbar}>
            <code className="mono">{token}</code>
            <span className={table.muted}>expires {formatTime(expires)}</span>
          </div>
        </Panel>
      ) : null}

      <Panel title="Fleet" meta={<StatusBadge status={runners.length ? "success" : "neutral"}>{runners.length}</StatusBadge>}>
        {runners.length === 0 ? (
          <EmptyState title="No runners" description="Register a runner with the token above." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Labels</th>
                <th>Drained</th>
                <th>Heartbeat</th>
              </tr>
            </thead>
            <tbody>
              {runners.map((r) => (
                <tr key={r.id}>
                  <td>{r.name}</td>
                  <td>
                    <StatusBadge status={runStatus(r.status)}>{r.status}</StatusBadge>
                  </td>
                  <td className="mono">{(r.labels ?? []).join(", ") || "—"}</td>
                  <td>{r.drained ? "yes" : "no"}</td>
                  <td className={table.muted}>{formatTime(r.last_heartbeat_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
