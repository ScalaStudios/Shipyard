import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, PipelineRun, Runner, SystemInfo } from "../api";
import { formatTime, runStatus } from "../lib/format";

export function OverviewPage() {
  const { org, project } = useWorkspace();
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [runners, setRunners] = useState<Runner[]>([]);
  const [runs, setRuns] = useState<PipelineRun[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void (async () => {
      try {
        const [sys, runnerRes, runRes] = await Promise.all([
          api.systemInfo(),
          api.listRunners(),
          org && project ? api.listRuns(org.id, project.id) : Promise.resolve({ runs: [] as PipelineRun[] }),
        ]);
        if (cancelled) return;
        setInfo(sys);
        setRunners(runnerRes.runners ?? []);
        setRuns(runRes.runs ?? []);
      } catch {
        if (!cancelled) {
          setInfo(null);
          setRunners([]);
          setRuns([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [org?.id, project?.id]);

  const online = runners.filter((r) => r.status === "online" || r.status === "ready").length;
  const failed = runs.filter((r) => r.status === "failed").length;
  const running = runs.filter((r) => r.status === "running" || r.status === "queued").length;
  const attention = runs.filter((r) => r.status === "failed" || r.status === "cancelled").slice(0, 8);

  return (
    <div className={table.stack}>
      <PageHeader
        title="Overview"
        description="Control-plane health, runner capacity, and recent delivery activity for the selected project."
      />

      <div className={table.stats}>
        <div className={table.stat}>
          <div className={table.statLabel}>Phase</div>
          <div className={table.statValue}>{info?.phase ?? (loading ? "…" : "—")}</div>
        </div>
        <div className={table.stat}>
          <div className={table.statLabel}>Runners online</div>
          <div className={table.statValue}>
            {online}/{runners.length}
          </div>
        </div>
        <div className={table.stat}>
          <div className={table.statLabel}>Active / queued</div>
          <div className={table.statValue}>{running}</div>
        </div>
        <div className={table.stat}>
          <div className={table.statLabel}>Failed (recent)</div>
          <div className={table.statValue}>{failed}</div>
        </div>
      </div>

      <Panel title="Needs attention" meta={<StatusBadge status={attention.length ? "danger" : "success"}>{attention.length}</StatusBadge>}>
        {!org || !project ? (
          <EmptyState title="Select an organization and project" description="Create them under Projects to populate this dashboard." />
        ) : attention.length === 0 ? (
          <EmptyState title="Nothing needs attention" description="Recent runs are healthy." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Run</th>
                <th>Status</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {attention.map((run) => (
                <tr key={run.id}>
                  <td>
                    <Link to={`/pipelines/runs/${run.id}`}>#{run.number}</Link>
                  </td>
                  <td>
                    <StatusBadge status={runStatus(run.status)}>{run.status}</StatusBadge>
                  </td>
                  <td className={table.muted}>{formatTime(run.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel title="Recent runs" meta={<StatusBadge status="info">{runs.length}</StatusBadge>}>
        {!org || !project ? (
          <EmptyState title="No project context" />
        ) : runs.length === 0 ? (
          <EmptyState title="No runs yet" description="Save a pipeline and start a run from Pipelines." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Run</th>
                <th>Status</th>
                <th>Trigger</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {runs.slice(0, 12).map((run) => (
                <tr key={run.id}>
                  <td>
                    <Link to={`/pipelines/runs/${run.id}`}>#{run.number}</Link>
                  </td>
                  <td>
                    <StatusBadge status={runStatus(run.status)}>{run.status}</StatusBadge>
                  </td>
                  <td className={table.muted}>{run.trigger_type ?? "—"}</td>
                  <td className={table.muted}>{formatTime(run.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel title="Runners" meta={<StatusBadge status={online ? "success" : "neutral"}>{runners.length}</StatusBadge>}>
        {runners.length === 0 ? (
          <EmptyState title="No runners registered" description="Create a registration token under Runners." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Labels</th>
                <th>Heartbeat</th>
              </tr>
            </thead>
            <tbody>
              {runners.map((runner) => (
                <tr key={runner.id}>
                  <td>{runner.name}</td>
                  <td>
                    <StatusBadge status={runStatus(runner.status)}>{runner.status}</StatusBadge>
                  </td>
                  <td className="mono">{(runner.labels ?? []).join(", ") || "—"}</td>
                  <td className={table.muted}>{formatTime(runner.last_heartbeat_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
