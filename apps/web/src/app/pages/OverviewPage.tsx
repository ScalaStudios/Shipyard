import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle, Gauge, Loader as LoaderIcon, Server } from "lucide-react";
import { Button, EmptyState, Panel, RunStatus, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, OnboardingStatus, PipelineRun, Runner, SystemInfo } from "../api";
import { formatTime, isRunnerAlive, runStatus } from "../lib/format";
import onboarding from "./OnboardingPage.module.css";
import styles from "./OverviewPage.module.css";

const SHORTCUTS = [
  { to: "/pipelines", label: "Pipelines" },
  { to: "/artifacts", label: "Artifacts" },
  { to: "/registry", label: "Registry" },
  { to: "/releases", label: "Releases" },
  { to: "/deployments", label: "Deployments" },
  { to: "/runners", label: "Add a runner" },
  { to: "/projects/import", label: "Import from forge" },
  { to: "/settings/authentication", label: "Sign-in providers" },
];

export function OverviewPage() {
  const { org, project, user, setError } = useWorkspace();
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [runners, setRunners] = useState<Runner[]>([]);
  const [runs, setRuns] = useState<PipelineRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [setup, setSetup] = useState<OnboardingStatus | null>(null);

  useEffect(() => {
    void api
      .onboarding()
      .then(setSetup)
      .catch(() => setSetup(null));
  }, []);

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
      } catch (err) {
        if (!cancelled) {
          setInfo(null);
          setRunners([]);
          setRuns([]);
          setError(err instanceof Error ? err.message : "failed to load overview");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [org?.id, project?.id]);

  const online = runners.filter((r) => isRunnerAlive(r)).length;
  const failed = runs.filter((r) => r.status === "failed").length;
  const running = runs.filter((r) => r.status === "running" || r.status === "queued" || r.status === "pending").length;
  const attention = runs.filter((r) => r.status === "failed" || r.status === "canceled").slice(0, 6);
  const scope = project?.name ?? org?.name;
  const metrics = [
    { icon: LoaderIcon, value: running, label: "Runs active or queued" },
    { icon: AlertTriangle, value: failed, label: "Failed runs in recent history" },
    { icon: Server, value: `${online}/${runners.length}`, label: "Runners online" },
    { icon: Gauge, value: info?.phase ?? (loading ? "…" : "—"), label: "Control-plane phase" },
  ];

  return (
    <div className={table.stack}>
      {user.is_admin && setup && !setup.complete ? (
        <div className={onboarding.banner}>
          <span className={onboarding.bannerCopy}>
            {setup.remaining} setup {setup.remaining === 1 ? "step" : "steps"} left before this instance can ship anything.
          </span>
          <Link to="/get-started">
            <Button type="button" variant="primary">
              Finish setup
            </Button>
          </Link>
        </div>
      ) : null}

      <PageHeader
        title="Overview"
        description={scope ? <>What needs attention in <strong>{scope}</strong> right now.</> : "Select a project to see its delivery activity."}
      />

      <section className={styles.metrics} aria-label="Delivery metrics">
        {metrics.map((m) => {
          const Icon = m.icon;
          return (
            <div key={m.label} className={styles.metric}>
              <Icon size={20} strokeWidth={1.75} className={styles.metricIcon} aria-hidden="true" />
              <div className={styles.metricValue}>{m.value}</div>
              <div className={styles.metricLabel}>{m.label}</div>
            </div>
          );
        })}
      </section>

      <div className={styles.split}>
        <Panel
          title="Needs attention"
          description={attention.length ? `${attention.length} runs, most recent first.` : "Failed and canceled runs appear here."}
          actions={
            org && project ? (
              <Link className={styles.textLink} to="/pipelines">
                All runs
              </Link>
            ) : null
          }
        >
          {!org || !project ? (
            <EmptyState
              title="Select an organization and project"
              description="Create them under Projects, or import an existing forge organization."
              action={
                <Link to={org ? "/projects/import" : "/projects"}>
                  <Button type="button" variant="primary">
                    {org ? "Import from forge" : "Get started"}
                  </Button>
                </Link>
              }
            />
          ) : attention.length === 0 ? (
            <EmptyState title="Nothing needs attention" description="Recent runs are healthy." />
          ) : (
            <ul className={styles.list}>
              {attention.map((run) => (
                <li key={run.id} className={styles.row}>
                  <RunStatus status={run.status} label={`Run #${run.number}`} />
                  <span className={styles.rowMeta}>
                    {run.error_message || run.git_ref || run.trigger_type || "No details"} · {formatTime(run.created_at)}
                  </span>
                  <Link className={styles.rowAction} to={`/pipelines/runs/${run.id}`}>
                    View run
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Panel>

        <Panel
          title="Runner fleet"
          description={runners.length ? `${online} of ${runners.length} online.` : "No runners registered yet."}
          actions={
            <Link className={styles.textLink} to="/runners">
              Manage
            </Link>
          }
        >
          {runners.length === 0 ? (
            <EmptyState title="No runners registered" description="Generate an install command under Runners." />
          ) : (
            <ul className={styles.list}>
              {runners.slice(0, 6).map((runner) => {
                const alive = isRunnerAlive(runner);
                return (
                  <li key={runner.id} className={styles.row}>
                    <span className={styles.rowTitle}>{runner.name}</span>
                    <span className={`${styles.rowMeta} mono`}>{(runner.labels ?? []).join(", ") || "no labels"}</span>
                    <StatusBadge status={runStatus(alive ? runner.status : "offline")}>{alive ? runner.status : "offline"}</StatusBadge>
                  </li>
                );
              })}
            </ul>
          )}
        </Panel>
      </div>

      <section aria-labelledby="shortcuts-heading">
        <h2 id="shortcuts-heading" className={styles.sectionTitle}>
          Shortcuts
        </h2>
        <div className={styles.shortcuts}>
          {SHORTCUTS.map((s) => (
            <Link key={s.to} className={styles.shortcut} to={s.to}>
              {s.label}
            </Link>
          ))}
        </div>
      </section>

      <Panel title="Recent runs" description={project ? `Latest activity in ${project.name}.` : undefined}>
        {!org || !project ? (
          <EmptyState title="No project selected" description="Choose a project in the top bar to list its runs." />
        ) : runs.length === 0 ? (
          <EmptyState title="No runs yet" description="Save a pipeline and start a run from Pipelines." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Run</th>
                <th>Status</th>
                <th>Ref</th>
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
                    <RunStatus status={run.status} />
                  </td>
                  <td className="mono">{run.git_ref ?? "—"}</td>
                  <td className={table.muted}>{run.trigger_type ?? "—"}</td>
                  <td className={table.muted}>{formatTime(run.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
