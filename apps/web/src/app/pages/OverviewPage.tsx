import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle, Gauge, Loader as LoaderIcon, Server } from "lucide-react";
import { Button, EmptyState, Panel, RunStatus, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, OnboardingStatus, OrgRun, Runner, SystemInfo } from "../api";
import { isRunnerAlive, runStatus, timeAgo } from "../lib/format";
import { usePageTitle } from "../lib/usePageTitle";
import onboarding from "./OnboardingPage.module.css";
import styles from "./OverviewPage.module.css";

const DAY_MS = 24 * 60 * 60 * 1000;

const SHORTCUTS = [
  { to: "/pipelines", label: "Pipelines" },
  { to: "/registry", label: "Registry" },
  { to: "/releases", label: "Releases" },
  { to: "/deployments", label: "Deployments" },
  { to: "/runners", label: "Add a runner" },
  { to: "/projects/import", label: "Import repositories" },
  { to: "/organization", label: "Members" },
];

function isFailure(status: string) {
  return status === "failed" || status === "error";
}

function branch(ref?: string) {
  return ref ? ref.replace(/^refs\/heads\//, "") : "—";
}

function ageMs(value: string, now: number) {
  const t = new Date(value).getTime();
  return Number.isNaN(t) ? Number.POSITIVE_INFINITY : now - t;
}

export function OverviewPage() {
  usePageTitle("Overview");
  const { org, user, setError } = useWorkspace();
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [runners, setRunners] = useState<Runner[]>([]);
  const [runs, setRuns] = useState<OrgRun[]>([]);
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
      const [sysRes, runnerRes, runRes] = await Promise.allSettled([
        api.systemInfo(),
        api.listRunners(),
        org ? api.listOrgRuns(org.id) : Promise.resolve({ runs: [] as OrgRun[] }),
      ]);
      if (cancelled) return;
      setInfo(sysRes.status === "fulfilled" ? sysRes.value : null);
      setRunners(runnerRes.status === "fulfilled" ? (runnerRes.value.runners ?? []) : []);
      setRuns(runRes.status === "fulfilled" ? (runRes.value.runs ?? []) : []);
      const failed = [runnerRes.status === "rejected" ? "runners" : "", runRes.status === "rejected" ? "recent runs" : ""].filter(Boolean);
      if (failed.length > 0) setError(`failed to load ${failed.join(", ")}`);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [org?.id]);

  const now = Date.now();
  const online = runners.filter((r) => isRunnerAlive(r, now)).length;
  const active = runs.filter((r) => r.status === "running" || r.status === "queued" || r.status === "pending").length;
  const failedDay = runs.filter((r) => ageMs(r.created_at, now) <= DAY_MS && isFailure(r.status));
  const attention = failedDay.slice(0, 6);
  const metrics = [
    { icon: LoaderIcon, value: active, label: "Runs active or queued" },
    { icon: AlertTriangle, value: failedDay.length, label: "Failed runs in the last 24 hours" },
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
        description={
          org ? (
            <>
              What needs attention in <strong>{org.name}</strong> right now.
            </>
          ) : (
            "Delivery activity and platform health across your organizations."
          )
        }
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
          description={attention.length ? `${failedDay.length} failed in the last 24 hours, most recent first.` : "Failed runs from the last 24 hours appear here."}
          actions={
            <Link className={styles.textLink} to="/pipelines">
              All runs
            </Link>
          }
        >
          {!org ? (
            <EmptyState
              title="Create an organization"
              description="Create one under Projects, or import an existing forge organization."
              action={
                <Link to="/projects">
                  <Button type="button" variant="primary">
                    Get started
                  </Button>
                </Link>
              }
            />
          ) : attention.length === 0 ? (
            <EmptyState title="Nothing needs attention" description="No runs failed in the last 24 hours." />
          ) : (
            <ul className={styles.list}>
              {attention.map((run) => (
                <li key={run.id} className={styles.row}>
                  <RunStatus status={run.status} label={`${run.project_name} #${run.number}`} />
                  <span className={styles.rowMeta}>
                    {run.pipeline_slug || "pipeline"} · {timeAgo(run.created_at, now)}
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
                const alive = isRunnerAlive(runner, now);
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

      <Panel title="Recent runs" description={org ? `Latest activity across ${org.name}.` : undefined}>
        {loading ? (
          <TableSkeleton />
        ) : runs.length === 0 ? (
          <EmptyState title="No runs yet" description="Connect a repository or start a run from Pipelines." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Project</th>
                <th>Pipeline</th>
                <th>Run</th>
                <th>Status</th>
                <th>Ref</th>
                <th>When</th>
              </tr>
            </thead>
            <tbody>
              {runs.slice(0, 15).map((run) => (
                <tr key={run.id}>
                  <td>{run.project_name}</td>
                  <td className="mono">{run.pipeline_slug || "—"}</td>
                  <td className="mono">
                    <Link to={`/pipelines/runs/${run.id}`}>#{run.number}</Link>
                  </td>
                  <td>
                    <RunStatus status={run.status} />
                  </td>
                  <td className="mono">{branch(run.git_ref)}</td>
                  <td className={table.muted}>{timeAgo(run.created_at, now)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
