import { useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Job, LogLine, PipelineRun } from "../api";
import { formatTime, runStatus } from "../lib/format";

const ACTIVE_STATUSES = ["pending", "queued", "running"];

export function RunDetailPage() {
  const { runId = "" } = useParams();
  const { org, project, setError } = useWorkspace();
  const [run, setRun] = useState<PipelineRun | null>(null);
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

  async function loadLogs(id: string, cancelled?: () => boolean) {
    if (!org || !project) return;
    const res = await api.jobLogs(org.id, project.id, id, lastSeq.current);
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

  async function load(cancelled?: () => boolean) {
    if (!org || !project || !runId) return;
    inFlight.current = true;
    try {
      const detail = await api.getRun(org.id, project.id, runId);
      if (cancelled?.()) return;
      const list = detail.jobs ?? [];
      setRun(detail.run);
      setJobs(list);
      const current = list.some((j) => j.id === jobRef.current) ? jobRef.current : (list[0]?.id ?? null);
      if (current !== jobRef.current) selectJob(current);
      if (current) await loadLogs(current, cancelled);
    } finally {
      inFlight.current = false;
    }
  }

  useEffect(() => {
    let cancelled = false;
    selectJob(null);
    setRun(null);
    setJobs([]);
    void load(() => cancelled).catch((err) => {
      if (!cancelled) setError(err instanceof Error ? err.message : "failed to load run");
    });
    return () => {
      cancelled = true;
    };
  }, [org?.id, project?.id, runId]);

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    const timer = window.setInterval(() => {
      if (inFlight.current) return;
      void load(() => cancelled).catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "failed to load run");
      });
    }, 2500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [org?.id, project?.id, runId, active]);

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
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load run");
    } finally {
      setRefreshing(false);
    }
  }

  async function openJob(id: string) {
    if (!org || !project) return;
    selectJob(id);
    setBusy(true);
    try {
      await loadLogs(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load logs");
    } finally {
      setBusy(false);
    }
  }

  async function cancel() {
    if (!org || !project || !run) return;
    setBusy(true);
    try {
      await api.cancelRun(org.id, project.id, run.id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to cancel run");
    } finally {
      setBusy(false);
    }
  }

  if (!org || !project) {
    return <EmptyState title="Select a project" />;
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

      {!run ? (
        <EmptyState title="Loading run…" />
      ) : (
        <Panel
          title="Jobs + logs"
          meta={<StatusBadge status={runStatus(run.status)}>{run.status}</StatusBadge>}
        >
          <div className={table.split} style={{ padding: 16 }}>
            <ul className={table.jobList}>
              {jobs.map((job) => (
                <li key={job.id}>
                  <button
                    type="button"
                    className={jobID === job.id ? table.jobActive : table.jobItem}
                    onClick={() => void openJob(job.id)}
                  >
                    <strong>{job.name}</strong>
                    <StatusBadge status={runStatus(job.status)}>{job.status}</StatusBadge>
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
