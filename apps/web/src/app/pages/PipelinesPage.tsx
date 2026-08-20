import { FormEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
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
  const [loading, setLoading] = useState(true);
  const yamlRef = useRef<HTMLTextAreaElement | null>(null);

  async function refresh() {
    if (!org || !project) {
      setPipelines([]);
      setRuns([]);
      setLoading(false);
      return;
    }
    try {
      const [p, r] = await Promise.all([api.listPipelines(org.id, project.id), api.listRuns(org.id, project.id)]);
      setPipelines(p.pipelines ?? []);
      setRuns(r.runs ?? []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
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

  function editPipeline(pipeline: Pipeline) {
    setSlug(pipeline.slug);
    setYaml(pipeline.yaml_source);
    yamlRef.current?.focus();
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
        description="Define pipeline YAML, trigger runs, and open a run to follow its jobs and logs."
      />

      {!org || !project ? (
        <EmptyState title="Select a project" description="Pipelines are scoped to an organization project." />
      ) : (
        <>
          <Panel title="Definitions" meta={<StatusBadge status="info">{pipelines.length}</StatusBadge>}>
            <form onSubmit={savePipeline}>
              <div className={form.row}>
                <FormField label="Name" htmlFor="pipeline-name" hint="Lowercase letters, numbers, and dashes.">
                  <input
                    id="pipeline-name"
                    className={table.input}
                    value={slug}
                    onChange={(e) => setSlug(e.target.value)}
                    required
                    pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
                  />
                </FormField>
                <div className={form.action}>
                  <Button type="submit" variant="primary" loading={busy}>
                    {pipelines.some((p) => p.slug === slug) ? "Update pipeline" : "Save pipeline"}
                  </Button>
                </div>
              </div>
              <div className={form.row}>
                <FormField label="Pipeline YAML" htmlFor="pipeline-yaml">
                  <textarea
                    id="pipeline-yaml"
                    className={table.textarea}
                    ref={yamlRef}
                    value={yaml}
                    onChange={(e) => setYaml(e.target.value)}
                    rows={10}
                  />
                </FormField>
              </div>
            </form>
            {loading ? (
              <TableSkeleton />
            ) : pipelines.length === 0 ? (
              <EmptyState title="No pipelines yet" description="Name a pipeline above, paste its YAML, and save it." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Display name</th>
                    <th>Created</th>
                    <th className={table.actionCol} />
                  </tr>
                </thead>
                <tbody>
                  {pipelines.map((p) => (
                    <tr key={p.id}>
                      <td className="mono">{p.slug}</td>
                      <td>{p.name}</td>
                      <td className={table.muted}>{formatTime(p.created_at)}</td>
                      <td className={table.actionCol}>
                        <div className={table.formRow}>
                          <Button variant="secondary" disabled={busy} onClick={() => void runPipeline(p.id)}>
                            Run
                          </Button>
                          <Button variant="secondary" disabled={busy} onClick={() => editPipeline(p)}>
                            Edit
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
          </Panel>

          <Panel title="Recent runs" meta={<StatusBadge status="info">{runs.length}</StatusBadge>}>
            {loading ? (
              <TableSkeleton />
            ) : runs.length === 0 ? (
              <EmptyState title="No runs yet" description="Run a pipeline from the table above to see it here." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Run</th>
                    <th>Status</th>
                    <th>Trigger</th>
                    <th>Ref</th>
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
                      <td className={table.muted}>{run.trigger_type ?? "manual"}</td>
                      <td className="mono">{run.git_ref || "—"}</td>
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
