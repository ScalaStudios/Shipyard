import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Deployment, Environment, Release } from "../api";
import { formatTime, runStatus } from "../lib/format";

export function DeploymentsPage() {
  const { org, project, setError } = useWorkspace();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [releases, setReleases] = useState<Release[]>([]);
  const [envSlug, setEnvSlug] = useState("staging");
  const [envName, setEnvName] = useState("Staging");
  const [deployPipelineSlug, setDeployPipelineSlug] = useState("");
  const [environmentID, setEnvironmentID] = useState("");
  const [releaseID, setReleaseID] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    if (!org || !project) {
      setDeployments([]);
      setEnvironments([]);
      setReleases([]);
      return;
    }
    const [d, e, r] = await Promise.all([
      api.listDeployments(org.id, project.id),
      api.listEnvironments(org.id, project.id),
      api.listReleases(org.id, project.id),
    ]);
    const envList = e.environments ?? [];
    const releaseList = r.releases ?? [];
    setDeployments(d.deployments ?? []);
    setEnvironments(envList);
    setReleases(releaseList);
    setEnvironmentID((prev) => (envList.some((x) => x.id === prev) ? prev : (envList[0]?.id ?? "")));
    setReleaseID((prev) => (releaseList.some((x) => x.id === prev) ? prev : (releaseList[0]?.id ?? "")));
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load deployments"));
  }, [org?.id, project?.id]);

  async function createEnv(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.createEnvironment(org.id, project.id, { slug: envSlug, name: envName, deploy_pipeline_slug: deployPipelineSlug });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create environment");
    } finally {
      setBusy(false);
    }
  }

  async function deploy(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.createDeployment(org.id, project.id, { environment_id: environmentID, release_id: releaseID });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create deployment");
    } finally {
      setBusy(false);
    }
  }

  const envNameByID = Object.fromEntries(environments.map((e) => [e.id, e.name]));
  const releaseByID = Object.fromEntries(releases.map((r) => [r.id, r.version]));

  return (
    <div className={table.stack}>
      <PageHeader title="Deployments" description="Promote releases into named environments." />
      {!org || !project ? (
        <EmptyState title="Select a project" />
      ) : (
        <>
          <Panel
            title="Environments"
            meta={<StatusBadge status="info">{environments.length}</StatusBadge>}
            actions={
              <form className={table.formRow} onSubmit={createEnv}>
                <input className={table.input} value={envSlug} onChange={(e) => setEnvSlug(e.target.value)} required pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?" aria-label="Environment slug" />
                <input className={table.input} value={envName} onChange={(e) => setEnvName(e.target.value)} required aria-label="Environment name" />
                <input className={table.input} value={deployPipelineSlug} onChange={(e) => setDeployPipelineSlug(e.target.value)} aria-label="Deploy pipeline slug" placeholder="deploy pipeline slug" />
                <Button type="submit" variant="secondary" loading={busy}>
                  Add environment
                </Button>
              </form>
            }
          >
            {environments.length === 0 ? (
              <EmptyState title="No environments" />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Slug</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {environments.map((e) => (
                    <tr key={e.id}>
                      <td>{e.name}</td>
                      <td className="mono">{e.slug}</td>
                      <td className={table.muted}>{formatTime(e.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
          </Panel>

          <Panel
            title="Deployments"
            meta={<StatusBadge status="info">{deployments.length}</StatusBadge>}
            actions={
              <form className={table.formRow} onSubmit={deploy}>
                <select className={table.select} value={environmentID} onChange={(e) => setEnvironmentID(e.target.value)} required aria-label="Environment">
                  {environments.map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
                </select>
                <select className={table.select} value={releaseID} onChange={(e) => setReleaseID(e.target.value)} required aria-label="Release">
                  {releases.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.version}
                    </option>
                  ))}
                </select>
                <Button type="submit" variant="primary" loading={busy} disabled={!environmentID || !releaseID}>
                  Deploy
                </Button>
              </form>
            }
          >
            {deployments.length === 0 ? (
              <EmptyState title="No deployments" />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Environment</th>
                    <th>Release</th>
                    <th>Status</th>
                    <th>Run</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {deployments.map((d) => (
                    <tr key={d.id}>
                      <td>{envNameByID[d.environment_id] ?? d.environment_id.slice(0, 8)}</td>
                      <td className="mono">{releaseByID[d.release_id] ?? d.release_id.slice(0, 8)}</td>
                      <td>
                        <StatusBadge status={runStatus(d.status)}>{d.status}</StatusBadge>
                      </td>
                      <td>{d.run_id ? <Link to={`/pipelines/runs/${d.run_id}`}>Run</Link> : <span className={table.muted}>—</span>}</td>
                      <td className={table.muted}>{formatTime(d.created_at)}</td>
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
