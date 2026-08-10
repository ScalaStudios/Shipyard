import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Pipeline, PipelineRun } from "../api";
import { defaultPipelineYAML, formatTime, runStatus } from "../lib/format";

export function PipelinesPage() {
  const { org, project, setError } = useWorkspace();
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [runs, setRuns] = useState<PipelineRun[]>([]);
  const [slug, setSlug] = useState("hello");
  const [yaml, setYaml] = useState(defaultPipelineYAML);
  const [busy, setBusy] = useState(false);

  async function refresh() {
    if (!org || !project) {
      setPipelines([]);
      setRuns([]);
      return;
    }
    const [p, r] = await Promise.all([api.listPipelines(org.id, project.id), api.listRuns(org.id, project.id)]);
    setPipelines(p.pipelines ?? []);
    setRuns(r.runs ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load pipelines"));
  }, [org?.id, project?.id]);

  async function savePipeline(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.upsertPipeline(org.id, project.id, { slug, yaml });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save pipeline");
    } finally {
      setBusy(false);
    }
  }

  async function runPipeline(pipelineID: string) {
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.startRun(org.id, project.id, pipelineID);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to start run");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Pipelines"
        description="Define pipeline YAML, trigger runs, and open Jenkins-style run detail with jobs and logs."
      />

      {!org || !project ? (
        <EmptyState title="Select a project" description="Pipelines are scoped to an organization project." />
      ) : (
        <>
          <Panel title="Definitions" meta={<StatusBadge status="info">{pipelines.length}</StatusBadge>}>
            {pipelines.length === 0 ? (
              <EmptyState title="No pipelines" description="Save a definition below." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Slug</th>
                    <th>Created</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {pipelines.map((p) => (
                    <tr key={p.id}>
                      <td>{p.name}</td>
                      <td className="mono">{p.slug}</td>
                      <td className={table.muted}>{formatTime(p.created_at)}</td>
                      <td>
                        <Button variant="secondary" disabled={busy} onClick={() => void runPipeline(p.id)}>
                          Run
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
            <form className={table.toolbar} onSubmit={savePipeline}>
              <input
                className={table.input}
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                required
                pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?"
                placeholder="slug"
              />
              <Button type="submit" variant="primary" loading={busy}>
                Save pipeline
              </Button>
            </form>
            <div style={{ padding: "0 16px 16px" }}>
              <textarea className={table.textarea} value={yaml} onChange={(e) => setYaml(e.target.value)} rows={10} />
            </div>
          </Panel>

          <Panel title="Recent runs" meta={<StatusBadge status="info">{runs.length}</StatusBadge>}>
            {runs.length === 0 ? (
              <EmptyState title="No runs" />
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
                  {runs.map((run) => (
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
        </>
      )}
    </div>
  );
}
