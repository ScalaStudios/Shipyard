import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, OIDCProvider, SCMConnection, SCMProvider } from "../../api";

export function IntegrationsSettingsPage() {
  const { org, project, setError } = useWorkspace();
  const [providers, setProviders] = useState<SCMProvider[]>([]);
  const [oidc, setOidc] = useState<OIDCProvider[]>([]);
  const [connections, setConnections] = useState<SCMConnection[]>([]);
  const [provider, setProvider] = useState("forgejo");
  const [name, setName] = useState("main");
  const [baseURL, setBaseURL] = useState("");
  const [repoOwner, setRepoOwner] = useState("");
  const [repoName, setRepoName] = useState("");
  const [token, setToken] = useState("");
  const [secret, setSecret] = useState("");
  const [bot, setBot] = useState("shipyard[bot]");
  const [pipelineSlug, setPipelineSlug] = useState("");
  const [webhookURL, setWebhookURL] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    const [p, o] = await Promise.all([api.listSCMProviders(), api.oidcProviders()]);
    setProviders(p.providers ?? []);
    setOidc(o.providers ?? []);
    if (!org || !project) {
      setConnections([]);
      return;
    }
    const c = await api.listSCMConnections(org.id, project.id);
    setConnections(c.connections ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load integrations"));
  }, [org?.id, project?.id]);

  useEffect(() => {
    const info = providers.find((p) => p.id === provider);
    if (info?.default_url) setBaseURL(info.default_url);
  }, [provider, providers]);

  async function create(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      const res = await api.createSCMConnection(org.id, project.id, {
        provider,
        name,
        base_url: baseURL,
        repo_owner: repoOwner,
        repo_name: repoName,
        access_token: token,
        webhook_secret: secret,
        bot_username: bot,
        pipeline_slug: pipelineSlug,
      });
      setWebhookURL(res.webhook_url);
      setToken("");
      setSecret("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create connection");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.deleteSCMConnection(org.id, project.id, id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to delete connection");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Integrations"
        description="Connect GitHub, GitLab, Forgejo, Gitea, Codeberg, Gogs, OneDev, GitBucket, Pagure, and more. The bot token posts build status on pull requests like Vercel."
      />

      <Panel
        title="Login providers (OIDC)"
        meta={<StatusBadge status={oidc.length ? "success" : "neutral"}>{oidc.length}</StatusBadge>}
        actions={
          <Link to="/settings/authentication">
            <Button type="button" variant="secondary">
              Manage providers
            </Button>
          </Link>
        }
      >
        {oidc.length === 0 ? (
          <EmptyState
            title="No sign-in providers configured"
            description="Add one under Settings › Authentication. No server restart needed."
          />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Kind</th>
              </tr>
            </thead>
            <tbody>
              {oidc.map((p) => (
                <tr key={p.name}>
                  <td>{p.name}</td>
                  <td className="mono">{p.kind}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel title="Supported forges" meta={<StatusBadge status="info">{providers.length}</StatusBadge>}>
        <DataTable>
          <thead>
            <tr>
              <th>Forge</th>
              <th>Family</th>
              <th>Notes</th>
            </tr>
          </thead>
          <tbody>
            {providers.map((p) => (
              <tr key={p.id}>
                <td>{p.label}</td>
                <td className="mono">{p.family}</td>
                <td className={table.muted}>{p.description}</td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      </Panel>

      <Panel
        title="Repository connections"
        meta={<StatusBadge status="info">{connections.length}</StatusBadge>}
      >
        {!org || !project ? (
          <EmptyState title="Select a project" description="Connections are scoped to the topbar project." />
        ) : (
          <>
            <form className={table.toolbar} onSubmit={create} style={{ flexDirection: "column", alignItems: "stretch" }}>
              <div className={table.formRow}>
                <select className={table.select} value={provider} onChange={(e) => setProvider(e.target.value)} aria-label="Provider">
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.label}
                    </option>
                  ))}
                </select>
                <input className={table.input} placeholder="connection name" value={name} onChange={(e) => setName(e.target.value)} required pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?" aria-label="Connection name" />
                <input className={table.input} placeholder="base URL" value={baseURL} onChange={(e) => setBaseURL(e.target.value)} required aria-label="Base URL" />
              </div>
              <div className={table.formRow}>
                <input className={table.input} placeholder="owner / group" value={repoOwner} onChange={(e) => setRepoOwner(e.target.value)} required aria-label="Repository owner or group" />
                <input className={table.input} placeholder="repository" value={repoName} onChange={(e) => setRepoName(e.target.value)} required aria-label="Repository" />
                <input className={table.input} placeholder="Pipeline (optional)" value={pipelineSlug} onChange={(e) => setPipelineSlug(e.target.value)} aria-label="Pipeline (optional)" />
              </div>
              <div className={table.formRow}>
                <input className={table.input} type="password" placeholder="bot access token" value={token} onChange={(e) => setToken(e.target.value)} aria-label="Bot access token" />
                <input className={table.input} type="password" placeholder="webhook secret" value={secret} onChange={(e) => setSecret(e.target.value)} aria-label="Webhook secret" />
                <input className={table.input} placeholder="bot display name" value={bot} onChange={(e) => setBot(e.target.value)} aria-label="Bot display name" />
                <Button type="submit" variant="primary" loading={busy}>
                  Connect
                </Button>
              </div>
            </form>
            {webhookURL ? (
              <div className={table.toolbar}>
                <span className={table.muted}>Inbound webhook</span>
                <code className="mono">{webhookURL}</code>
              </div>
            ) : null}
            {connections.length === 0 ? (
              <EmptyState title="No connections" description="Add a forge connection to enable webhooks and bot comments." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Provider</th>
                    <th>Repository</th>
                    <th>Bot</th>
                    <th>Token</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {connections.map((c) => (
                    <tr key={c.id}>
                      <td>{c.name}</td>
                      <td className="mono">{c.provider}</td>
                      <td className="mono">
                        {c.repo_owner}/{c.repo_name}
                      </td>
                      <td>{c.bot_username}</td>
                      <td>{c.has_token ? "yes" : "no"}</td>
                      <td>
                        <Button variant="ghost" disabled={busy} onClick={() => void remove(c.id)}>
                          Remove
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
          </>
        )}
      </Panel>
    </div>
  );
}
