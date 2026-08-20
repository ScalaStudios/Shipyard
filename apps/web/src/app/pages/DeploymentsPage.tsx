import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
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
  const [loading, setLoading] = useState(true);

  async function refresh() {
    if (!org || !project) {
      setDeployments([]);
      setEnvironments([]);
      setReleases([]);
      setLoading(false);
      return;
    }
    try {
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
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
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
        <EmptyState title="Select a project" description="Environments and deployments are scoped to a project." />
      ) : (
        <>
          <Panel title="Environments" meta={<StatusBadge status="info">{environments.length}</StatusBadge>}>
            <form className={form.row} onSubmit={createEnv}>
              <FormField label="Environment name" htmlFor="env-name-key" hint="Lowercase letters, numbers, and dashes.">
                <input
                  id="env-name-key"
                  className={table.input}
                  value={envSlug}
                  onChange={(e) => setEnvSlug(e.target.value)}
                  required
                  pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
                />
              </FormField>
              <FormField label="Display name" htmlFor="env-name">
                <input
                  id="env-name"
                  className={table.input}
                  value={envName}
                  onChange={(e) => setEnvName(e.target.value)}
                  required
                />
              </FormField>
              <FormField label="Deploy pipeline" htmlFor="env-pipeline">
                <input
                  id="env-pipeline"
                  className={table.input}
                  value={deployPipelineSlug}
                  onChange={(e) => setDeployPipelineSlug(e.target.value)}
                />
              </FormField>
              <div className={form.action}>
                <Button type="submit" variant="secondary" loading={busy}>
                  Add environment
                </Button>
              </div>
            </form>
            {loading ? (
              <TableSkeleton />
            ) : environments.length === 0 ? (
              <EmptyState title="No environments yet" description="Add one above, then deploy a release into it." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Display name</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {environments.map((e) => (
                    <tr key={e.id}>
                      <td className="mono">{e.slug}</td>
                      <td>{e.name}</td>
                      <td className={table.muted}>{formatTime(e.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
          </Panel>

          <Panel title="Deployments" meta={<StatusBadge status="info">{deployments.length}</StatusBadge>}>
            <form className={form.row} onSubmit={deploy}>
              <FormField label="Environment" htmlFor="deploy-env">
                <select
                  id="deploy-env"
                  className={table.select}
                  value={environmentID}
                  onChange={(e) => setEnvironmentID(e.target.value)}
                  required
                >
                  {environments.map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
                </select>
              </FormField>
              <FormField label="Release" htmlFor="deploy-release">
                <select
                  id="deploy-release"
                  className={table.select}
                  value={releaseID}
                  onChange={(e) => setReleaseID(e.target.value)}
                  required
                >
                  {releases.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.version}
                    </option>
                  ))}
                </select>
              </FormField>
              <div className={form.action}>
                <Button type="submit" variant="primary" loading={busy} disabled={!environmentID || !releaseID}>
                  Deploy
                </Button>
              </div>
            </form>
            {loading ? (
              <TableSkeleton />
            ) : deployments.length === 0 ? (
              <EmptyState title="No deployments yet" description="Pick an environment and a release above to deploy." />
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
