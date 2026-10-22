import { useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, EmptyState, Panel, RunStatus } from "@shipyard/ui";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Job, LogLine, PipelineRun } from "../api";
import { formatTime } from "../lib/format";
import { usePageTitle } from "../lib/usePageTitle";

const ACTIVE_STATUSES = ["pending", "queued", "running"];

export function RunDetailPage() {
  const { runId = "" } = useParams();
  const { org, setProjectID, setError } = useWorkspace();
  const [run, setRun] = useState<PipelineRun | null>(null);
  const [runProjectID, setRunProjectID] = useState("");
  const [missing, setMissing] = useState(false);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobID, setJobID] = useState<string | null>(null);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const jobRef = useRef<string | null>(null);
  const inFlight = useRef(false);
  const lastSeq = useRef(0);
  const logRef = useRef<HTMLPreElement | null>(null);
  const stick = useRef(true);

  const active = !run || ACTIVE_STATUSES.includes(run.status);

  usePageTitle(run ? `Run #${run.number}` : "Run detail");

  async function loadLogs(projectID: string, id: string, cancelled?: () => boolean) {
    if (!org) return;
    const res = await api.jobLogs(org.id, projectID, id, lastSeq.current);
    const lines = res.logs ?? [];
    if (cancelled?.() || jobRef.current !== id || lines.length === 0) return;
    lastSeq.current = lines.reduce((max, l) => (typeof l.seq === "number" && l.seq > max ? l.seq : max), lastSeq.current);
    setLogs((prev) => [...prev, ...lines]);
  }

  function selectJob(id: string | null) {
    jobRef.current = id;
    lastSeq.current = 0;
    setJobID(id);
    setLogs([]);
    stick.current = true;
  }

  async function load(projectID: string, cancelled?: () => boolean) {
    if (!org || !projectID || !runId) return;
    inFlight.current = true;
    try {
      const detail = await api.getRun(org.id, projectID, runId);
      if (cancelled?.()) return;
      const list = detail.jobs ?? [];
      setRun(detail.run);
      setJobs(list);
      const current = list.some((j) => j.id === jobRef.current) ? jobRef.current : (list[0]?.id ?? null);
      if (current !== jobRef.current) selectJob(current);
      if (current) await loadLogs(projectID, current, cancelled);
    } finally {
      inFlight.current = false;
    }
  }

  useEffect(() => {
    if (!org || !runId) return;
    let cancelled = false;
    selectJob(null);
    setRun(null);
    setJobs([]);
    setRunProjectID("");
    setMissing(false);
    void (async () => {
      let owner = "";
      try {
        const res = await api.getOrgRun(org.id, runId);
        owner = res.run.project_id ?? "";
      } catch {
        owner = "";
      }
      if (cancelled) return;
      if (!owner) {
        setMissing(true);
        return;
      }
      setRunProjectID(owner);
      setProjectID(owner);
      await load(owner, () => cancelled).catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "failed to load run");
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [org?.id, runId]);

  useEffect(() => {
    if (!active || !runProjectID) return;
    let cancelled = false;
    const timer = window.setInterval(() => {
      if (inFlight.current) return;
      void load(runProjectID, () => cancelled).catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "failed to load run");
      });
    }, 2500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [org?.id, runProjectID, runId, active]);

  useEffect(() => {
    const el = logRef.current;
    if (!el || !stick.current) return;
    el.scrollTop = el.scrollHeight;
  }, [logs]);

  function onLogScroll() {
    const el = logRef.current;
    if (!el) return;
    stick.current = el.scrollHeight - el.scrollTop - el.clientHeight <= 40;
  }

  async function refresh() {
    setRefreshing(true);
    try {
      await load(runProjectID);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load run");
    } finally {
      setRefreshing(false);
    }
  }

  async function openJob(id: string) {
    if (!org || !runProjectID) return;
    selectJob(id);
    setBusy(true);
    try {
      await loadLogs(runProjectID, id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load logs");
    } finally {
      setBusy(false);
    }
  }

  async function cancel() {
    if (!org || !runProjectID || !run) return;
    setBusy(true);
    try {
      await api.cancelRun(org.id, runProjectID, run.id);
      await load(runProjectID);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to cancel run");
    } finally {
      setBusy(false);
    }
  }

  if (!org) {
    return <EmptyState title="Select an organization" />;
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title={run ? `Run #${run.number}` : "Run detail"}
        description={run ? `Status ${run.status} · created ${formatTime(run.created_at)}` : "Inspect jobs and logs."}
        actions={
          <>
            <Button variant="secondary" onClick={() => void refresh()} loading={refreshing}>
              Refresh
            </Button>
            {run && (run.status === "running" || run.status === "queued" || run.status === "pending") ? (
              <Button variant="secondary" onClick={() => void cancel()} loading={busy}>
                Cancel
              </Button>
            ) : null}
            <Link to="/pipelines">
              <Button variant="secondary">Back to pipelines</Button>
            </Link>
          </>
        }
      />

      {missing ? (
        <EmptyState
          title="Run not found"
          description="This run is not in the selected organization, or you no longer have access to it."
        />
      ) : !run ? (
        <EmptyState title="Loading run…" />
      ) : (
        <Panel
          title="Jobs + logs"
          meta={<RunStatus status={run.status} />}
        >
          <div className={table.split} style={{ padding: "0 24px 24px" }}>
            <ul className={table.jobList}>
              {jobs.map((job) => (
                <li key={job.id}>
                  <button
                    type="button"
                    className={jobID === job.id ? table.jobActive : table.jobItem}
                    onClick={() => void openJob(job.id)}
                  >
                    <strong>{job.name}</strong>
                    <RunStatus status={job.status} />
                  </button>
                  {job.error_message ? (
                    <div className={table.muted} style={{ padding: "0 12px 12px", fontSize: 12, lineHeight: "16px" }}>
                      {job.error_message}
                    </div>
                  ) : null}
                </li>
              ))}
              {jobs.length === 0 ? <li className={table.jobItem}>No jobs yet</li> : null}
            </ul>
            <pre className={table.logs} ref={logRef} onScroll={onLogScroll}>
              {logs.length === 0 ? "No log lines yet." : logs.map((l) => l.line ?? "").join("\n")}
            </pre>
          </div>
        </Panel>
      )}
    </div>
  );
}
