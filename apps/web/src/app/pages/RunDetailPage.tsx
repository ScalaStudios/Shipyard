import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Job, LogLine, PipelineRun } from "../api";
import { formatTime, runStatus } from "../lib/format";

export function RunDetailPage() {
  const { runId = "" } = useParams();
  const { org, project, setError } = useWorkspace();
  const [run, setRun] = useState<PipelineRun | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobID, setJobID] = useState<string | null>(null);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [busy, setBusy] = useState(false);

  async function load() {
    if (!org || !project || !runId) return;
    const detail = await api.getRun(org.id, project.id, runId);
    setRun(detail.run);
    setJobs(detail.jobs ?? []);
    const first = detail.jobs?.[0]?.id ?? null;
    setJobID((prev) => prev ?? first);
    if (first || jobID) {
      const id = jobID && detail.jobs.some((j) => j.id === jobID) ? jobID : first;
      if (id) {
        const logRes = await api.jobLogs(org.id, project.id, id);
        setLogs(logRes.logs ?? []);
        setJobID(id);
      }
    }
  }

  useEffect(() => {
    void load().catch((err) => setError(err instanceof Error ? err.message : "failed to load run"));
  }, [org?.id, project?.id, runId]);

  async function openJob(id: string) {
    if (!org || !project) return;
    setJobID(id);
    setBusy(true);
    try {
      const logRes = await api.jobLogs(org.id, project.id, id);
      setLogs(logRes.logs ?? []);
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
            <Button variant="secondary" onClick={() => void load()} loading={busy}>
              Refresh
            </Button>
            {run && (run.status === "running" || run.status === "queued" || run.status === "pending") ? (
              <Button variant="secondary" onClick={() => void cancel()} loading={busy}>
                Cancel
              </Button>
            ) : null}
            <Link to="/pipelines">Back to pipelines</Link>
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
                </li>
              ))}
              {jobs.length === 0 ? <li className={table.jobItem}>No jobs yet</li> : null}
            </ul>
            <pre className={table.logs} aria-live="polite">
              {logs.length === 0 ? "No log lines yet." : logs.map((l) => l.line ?? "").join("\n")}
            </pre>
          </div>
        </Panel>
      )}
    </div>
  );
}
