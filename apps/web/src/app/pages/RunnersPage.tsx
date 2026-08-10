import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Runner, RunnerInstall } from "../api";
import { formatTime, runStatus } from "../lib/format";
import styles from "./RunnersPage.module.css";

export function RunnersPage() {
  const { org, setError } = useWorkspace();
  const [runners, setRunners] = useState<Runner[]>([]);
  const [install, setInstall] = useState<RunnerInstall | null>(null);
  const [name, setName] = useState("runner-1");
  const [labels, setLabels] = useState("linux");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState("");

  async function refresh() {
    const res = await api.listRunners();
    setRunners(res.runners ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load runners"));
    const id = window.setInterval(() => void refresh().catch(() => undefined), 8000);
    return () => window.clearInterval(id);
  }, [setError]);

  async function createInstall(event?: FormEvent) {
    event?.preventDefault();
    setBusy(true);
    setCopied("");
    try {
      const res = await api.createRunnerInstall({
        organization_id: org?.id,
        name,
        labels,
        ttl: "24h",
      });
      setInstall(res);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create install token");
    } finally {
      setBusy(false);
    }
  }

  async function copy(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      window.setTimeout(() => setCopied(""), 2000);
    } catch {
      setError("clipboard unavailable — select the command and copy manually");
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Runners"
        description="One-click install like Pterodactyl Wings — generate a token and run the curl | bash script on any machine."
        actions={
          <Button variant="primary" loading={busy} onClick={() => void createInstall()}>
            Create runner install
          </Button>
        }
      />

      <Panel title="New runner" meta={<StatusBadge status="info">auto · docker · binary</StatusBadge>}>
        <form className={table.toolbar} onSubmit={(e) => void createInstall(e)}>
          <input className={table.input} value={name} onChange={(e) => setName(e.target.value)} placeholder="runner name" required aria-label="Runner name" />
          <input className={table.input} value={labels} onChange={(e) => setLabels(e.target.value)} placeholder="labels (comma-separated)" aria-label="Labels" />
          <Button type="submit" variant="primary" loading={busy}>
            Generate install command
          </Button>
        </form>

        {install ? (
          <div className={styles.install}>
            <div className={styles.meta}>
              <span>
                Token expires <strong>{formatTime(install.expires_at)}</strong>
              </span>
              <span className="mono">{install.api_url}</span>
            </div>

            <div className={styles.block}>
              <div className={styles.blockHead}>
                <strong>Auto install</strong>
                <Button variant="secondary" onClick={() => void copy(install.curl_command, "curl")}>
                  {copied === "curl" ? "Copied" : "Copy"}
                </Button>
              </div>
              <p className={styles.hint}>Run on the target host (Docker preferred, otherwise Go binary + systemd).</p>
              <pre className={styles.code}>{install.curl_command}</pre>
            </div>

            <div className={styles.block}>
              <div className={styles.blockHead}>
                <strong>Docker only</strong>
                <Button variant="secondary" onClick={() => void copy(install.docker_command, "docker")}>
                  {copied === "docker" ? "Copied" : "Copy"}
                </Button>
              </div>
              <pre className={styles.code}>{install.docker_command}</pre>
            </div>

            <div className={styles.block}>
              <div className={styles.blockHead}>
                <strong>Manual env</strong>
                <Button variant="secondary" onClick={() => void copy(install.manual_env, "manual")}>
                  {copied === "manual" ? "Copied" : "Copy"}
                </Button>
              </div>
              <pre className={styles.code}>{install.manual_env}</pre>
            </div>

            <div className={styles.block}>
              <div className={styles.blockHead}>
                <strong>Registration token</strong>
                <Button variant="ghost" onClick={() => void copy(install.token, "token")}>
                  {copied === "token" ? "Copied" : "Copy token"}
                </Button>
              </div>
              <code className="mono">{install.token}</code>
            </div>
          </div>
        ) : (
          <EmptyState
            title="No install pending"
            description="Generate a command, then paste it on a Linux host with Docker or Go."
          />
        )}
      </Panel>

      <Panel title="Fleet" meta={<StatusBadge status={runners.length ? "success" : "neutral"}>{runners.length}</StatusBadge>}>
        {runners.length === 0 ? (
          <EmptyState title="No runners online" description="After install, this table shows name, status, and last heartbeat." />
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
