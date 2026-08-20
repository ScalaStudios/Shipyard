import { FormEvent, useEffect, useState } from "react";
import { IconHeart, IconHeartFilled } from "@tabler/icons-react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Runner, RunnerInstall } from "../api";
import { formatTime, heartbeatAgeMs, isRunnerAlive, runStatus } from "../lib/format";
import styles from "./RunnersPage.module.css";

function formatAge(ms: number | null): string {
  if (ms == null) return "never";
  if (ms < 1_500) return "just now";
  if (ms < 60_000) return `${Math.round(ms / 1000)}s ago`;
  if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m ago`;
  return `${Math.round(ms / 3_600_000)}h ago`;
}

function pingClass(ms: number | null): string {
  if (ms == null) return styles.ping;
  if (ms < 120) return styles.pingGood;
  if (ms < 400) return styles.pingWarn;
  return styles.pingBad;
}

function RunnerHeartbeat({ runner, now }: { runner: Runner; now: number }) {
  const alive = isRunnerAlive(runner, now);
  const age = heartbeatAgeMs(runner.last_heartbeat_at, now);
  const Heart = alive ? IconHeartFilled : IconHeart;

  return (
    <div className={styles.heartbeat} title={runner.last_heartbeat_at ? formatTime(runner.last_heartbeat_at) : "No heartbeat yet"}>
      <span className={styles.heartWrap} aria-hidden="true">
        {alive ? <span className={styles.pulseRing} /> : null}
        <Heart size={18} stroke={1.75} className={alive ? styles.heartAlive : styles.heartDead} />
      </span>
      <span className={styles.heartbeatCopy}>
        <span className={alive ? styles.heartbeatStateAlive : styles.heartbeatStateDead}>{alive ? "Alive" : "Dead"}</span>
        <span className={styles.heartbeatMeta}>
          Last beat {formatAge(age)}
          {runner.last_heartbeat_at ? ` · ${formatTime(runner.last_heartbeat_at)}` : ""}
        </span>
      </span>
    </div>
  );
}

export function RunnersPage() {
  const { org, setError } = useWorkspace();
  const [runners, setRunners] = useState<Runner[]>([]);
  const [install, setInstall] = useState<RunnerInstall | null>(null);
  const [name, setName] = useState("runner-1");
  const [labels, setLabels] = useState("linux");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState("");
  const [now, setNow] = useState(() => Date.now());
  const [apiPingMs, setApiPingMs] = useState<number | null>(null);
  const [confirmRemove, setConfirmRemove] = useState("");
  const [loading, setLoading] = useState(true);

  async function refresh() {
    const started = performance.now();
    try {
      const res = await api.listRunners();
      setApiPingMs(Math.round(performance.now() - started));
      setRunners(res.runners ?? []);
      setNow(Date.now());
    } finally {
      setLoading(false);
    }
  }

  async function removeRunner(runner: Runner) {
    if (confirmRemove !== runner.id) {
      setConfirmRemove(runner.id);
      return;
    }
    try {
      await api.deleteRunner(runner.id);
      setConfirmRemove("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to remove runner");
    }
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load runners"));
    const poll = window.setInterval(() => void refresh().catch(() => undefined), 4000);
    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      window.clearInterval(poll);
      window.clearInterval(tick);
    };
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

  const aliveCount = runners.filter((r) => isRunnerAlive(r, now)).length;

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
        <form className={form.row} onSubmit={(e) => void createInstall(e)}>
          <FormField label="Runner name" htmlFor="runner-name">
            <input id="runner-name" className={table.input} value={name} onChange={(e) => setName(e.target.value)} required />
          </FormField>
          <FormField label="Labels" htmlFor="runner-labels" hint="Comma-separated, matched against pipeline runner requirements.">
            <input id="runner-labels" className={table.input} value={labels} onChange={(e) => setLabels(e.target.value)} />
          </FormField>
          <div className={form.action}>
            <Button type="submit" variant="primary" loading={busy}>
              Generate install command
            </Button>
          </div>
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

      <Panel
        title="Fleet"
        meta={
          <div className={styles.meta}>
            <StatusBadge status={aliveCount ? "success" : runners.length ? "danger" : "neutral"}>
              {aliveCount}/{runners.length} alive
            </StatusBadge>
            <span className={pingClass(apiPingMs)} title="API round-trip for runner list">
              ping {apiPingMs == null ? "—" : `${apiPingMs} ms`}
            </span>
          </div>
        }
      >
        {loading ? (
          <TableSkeleton />
        ) : runners.length === 0 ? (
          <EmptyState title="No runners online" description="After install, this table shows heartbeat and alive/dead state." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Runner</th>
                <th>Heartbeat</th>
                <th>Status</th>
                <th>Labels</th>
                <th>Drained</th>
                <th className={table.actionCol} />
              </tr>
            </thead>
            <tbody>
              {runners.map((r) => {
                const alive = isRunnerAlive(r, now);
                return (
                  <tr key={r.id}>
                    <td>
                      <div className={styles.nameCell}>
                        <span className={styles.heartWrap} aria-hidden="true">
                          {alive ? <span className={styles.pulseRing} /> : null}
                          {alive ? (
                            <IconHeartFilled size={16} stroke={1.75} className={styles.heartAlive} />
                          ) : (
                            <IconHeart size={16} stroke={1.75} className={styles.heartDead} />
                          )}
                        </span>
                        <span className={styles.nameText}>
                          <span className={styles.nameTitle}>{r.name}</span>
                          <span className={table.muted}>{alive ? "Connected" : "No recent heartbeat"}</span>
                        </span>
                      </div>
                    </td>
                    <td>
                      <RunnerHeartbeat runner={r} now={now} />
                    </td>
                    <td>
                      <StatusBadge status={runStatus(alive ? r.status : "offline")}>{alive ? r.status : "offline"}</StatusBadge>
                    </td>
                    <td className="mono">{(r.labels ?? []).join(", ") || "—"}</td>
                    <td>{r.drained ? "yes" : "no"}</td>
                    <td className={table.actionCol}>
                      {alive || r.status === "busy" ? null : (
                        <Button type="button" variant="secondary" onClick={() => void removeRunner(r)}>
                          {confirmRemove === r.id ? "Confirm remove" : "Remove"}
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
