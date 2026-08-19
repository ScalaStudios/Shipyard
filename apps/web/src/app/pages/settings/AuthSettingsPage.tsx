import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, AuthProvider, GitHubAppStatus, InstanceUser } from "../../api";
import { FormField, FormFooter, FormGrid, FormNote, FormSection, GridField } from "./SettingsForm";

const KINDS = ["github", "gitlab", "forgejo", "gitea", "entra", "discord", "oidc"];

const PURPOSES = [
  {
    id: "login" as const,
    title: "Sign-in providers",
    description: "Let people sign in to Shipyard with an account they already have.",
    callback: (base: string, name: string) => `${base}/api/v1/auth/oidc/${name}/callback`,
  },
  {
    id: "forge" as const,
    title: "Forge applications",
    description: "Authorize Shipyard to read repositories and register webhooks when importing.",
    callback: (base: string, name: string) => `${base}/api/v1/forge/oauth/${name}/callback`,
  },
];

function ProviderSection({ purpose }: { purpose: (typeof PURPOSES)[number] }) {
  const { setError } = useWorkspace();
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [callbackBase, setCallbackBase] = useState("");
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState(KINDS[0]);
  const [issuer, setIssuer] = useState("");
  const [clientID, setClientID] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [scopes, setScopes] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    const res = await api.listAuthProviders(purpose.id);
    setProviders(res.providers ?? []);
    setCallbackBase(res.callback_base ?? "");
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load providers"));
  }, [purpose.id]);

  function reset() {
    setName("");
    setIssuer("");
    setClientID("");
    setClientSecret("");
    setScopes("");
    setAdding(false);
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await api.upsertAuthProvider({
        purpose: purpose.id,
        name,
        kind,
        issuer,
        client_id: clientID,
        client_secret: clientSecret,
        redirect_url: callbackBase ? purpose.callback(callbackBase, name) : "",
        scopes: scopes ? scopes.split(/[\s,]+/).filter(Boolean) : [],
        enabled: true,
      });
      reset();
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save provider");
    } finally {
      setBusy(false);
    }
  }

  async function toggle(provider: AuthProvider) {
    try {
      await api.upsertAuthProvider({
        purpose: purpose.id,
        name: provider.name,
        kind: provider.kind,
        issuer: provider.issuer,
        client_id: provider.client_id,
        redirect_url: provider.redirect_url,
        scopes: provider.scopes,
        enabled: !provider.enabled,
      });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update provider");
    }
  }

  async function remove(provider: AuthProvider) {
    try {
      await api.deleteAuthProvider(purpose.id, provider.id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to delete provider");
    }
  }

  return (
    <Panel
      title={purpose.title}
      meta={<StatusBadge status={providers.length ? "success" : "neutral"}>{providers.length}</StatusBadge>}
      actions={
        !adding ? (
          <Button type="button" variant="secondary" onClick={() => setAdding(true)}>
            Add provider
          </Button>
        ) : null
      }
    >
      {providers.length === 0 && !adding ? (
        <EmptyState title="Nothing configured" description={purpose.description} />
      ) : null}

      {providers.length > 0 ? (
        <DataTable>
          <thead>
            <tr>
              <th>Name</th>
              <th>Kind</th>
              <th>Client ID</th>
              <th>Secret</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {providers.map((p) => (
              <tr key={p.id}>
                <td className="mono">{p.name}</td>
                <td>{p.kind}</td>
                <td className="mono">{p.client_id}</td>
                <td>
                  <StatusBadge status={p.has_client_secret ? "success" : "warning"}>
                    {p.has_client_secret ? "stored" : "missing"}
                  </StatusBadge>
                </td>
                <td>
                  <StatusBadge status={p.enabled ? "success" : "neutral"}>
                    {p.enabled ? "enabled" : "disabled"}
                  </StatusBadge>
                </td>
                <td>
                  <div className={table.formRow}>
                    <Button type="button" variant="secondary" onClick={() => void toggle(p)}>
                      {p.enabled ? "Disable" : "Enable"}
                    </Button>
                    <Button type="button" variant="secondary" onClick={() => void remove(p)}>
                      Remove
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      ) : null}

      {adding ? (
        <form onSubmit={save}>
          <FormSection>
            <FormGrid>
              <GridField label="Kind" htmlFor={`${purpose.id}-kind`}>
                <select
                  id={`${purpose.id}-kind`}
                  className={table.select}
                  value={kind}
                  onChange={(e) => setKind(e.target.value)}
                >
                  {KINDS.map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
                </select>
              </GridField>
              <GridField label="Name" htmlFor={`${purpose.id}-name`}>
                <input
                  id={`${purpose.id}-name`}
                  className={table.input}
                  placeholder="github"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
                  required
                  spellCheck={false}
                />
              </GridField>
              <GridField label="Issuer or base URL" htmlFor={`${purpose.id}-issuer`}>
                <input
                  id={`${purpose.id}-issuer`}
                  className={table.input}
                  placeholder="https://git.example.com"
                  value={issuer}
                  onChange={(e) => setIssuer(e.target.value)}
                  spellCheck={false}
                />
              </GridField>
              <GridField label="Client ID" htmlFor={`${purpose.id}-client`}>
                <input
                  id={`${purpose.id}-client`}
                  className={table.input}
                  value={clientID}
                  onChange={(e) => setClientID(e.target.value)}
                  required
                  spellCheck={false}
                />
              </GridField>
              <GridField label="Client secret" htmlFor={`${purpose.id}-secret`}>
                <input
                  id={`${purpose.id}-secret`}
                  className={table.input}
                  type="password"
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                  autoComplete="off"
                />
              </GridField>
              <GridField label="Scopes" htmlFor={`${purpose.id}-scopes`}>
                <input
                  id={`${purpose.id}-scopes`}
                  className={table.input}
                  placeholder="leave blank for defaults"
                  value={scopes}
                  onChange={(e) => setScopes(e.target.value)}
                  spellCheck={false}
                />
              </GridField>
            </FormGrid>
          </FormSection>
          <FormFooter
            note={
              callbackBase && name
                ? `Redirect URL to register on the provider: ${purpose.callback(callbackBase, name)}`
                : "Enter a name to see the redirect URL to register on the provider."
            }
          >
            <div className={table.formRow}>
              <Button type="button" variant="secondary" onClick={reset}>
                Cancel
              </Button>
              <Button type="submit" variant="primary" loading={busy}>
                Save provider
              </Button>
            </div>
          </FormFooter>
        </form>
      ) : null}
    </Panel>
  );
}

function GitHubAppPanel() {
  const { setError } = useWorkspace();
  const [status, setStatus] = useState<GitHubAppStatus | null>(null);
  const [appID, setAppID] = useState("");
  const [slug, setSlug] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    const res = await api.githubAppStatus();
    setStatus(res);
    setAppID(res.app_id ?? "");
    setSlug(res.slug ?? "");
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load github app"));
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await api.setInstanceSetting({ key: "github_app.app_id", value: appID });
      await api.setInstanceSetting({ key: "github_app.slug", value: slug });
      if (privateKey.trim()) {
        await api.setInstanceSetting({ key: "github_app.private_key", value: privateKey, is_secret: true });
      }
      setPrivateKey("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save github app");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={save}>
      <Panel
        title="GitHub App"
        meta={
          <StatusBadge status={status?.configured ? "success" : "neutral"}>
            {status?.configured ? "configured" : "not set up"}
          </StatusBadge>
        }
      >
        <FormSection>
          <FormField
            label="App ID"
            hint="A GitHub App gives Shipyard org-wide access without a personal token, and mints short-lived installation tokens on demand instead of storing one."
            htmlFor="gh-app-id"
          >
            <input
              id="gh-app-id"
              className={table.input}
              placeholder="123456"
              value={appID}
              onChange={(e) => setAppID(e.target.value)}
              spellCheck={false}
            />
          </FormField>
          <FormField label="App slug" hint="The name in the app's URL, e.g. github.com/apps/shipyard-ci." htmlFor="gh-app-slug">
            <input
              id="gh-app-slug"
              className={table.input}
              placeholder="shipyard-ci"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              spellCheck={false}
            />
          </FormField>
          <FormField
            label="Private key"
            hint="The PEM file GitHub generated for the app. Stored encrypted and never shown again."
            htmlFor="gh-app-key"
          >
            <textarea
              id="gh-app-key"
              className={table.textarea}
              placeholder={status?.has_key ? "Stored — paste a new PEM to replace it" : "-----BEGIN RSA PRIVATE KEY-----"}
              value={privateKey}
              onChange={(e) => setPrivateKey(e.target.value)}
              autoComplete="off"
              spellCheck={false}
            />
          </FormField>
        </FormSection>
        <FormFooter
          note={
            status?.callback_url ? `Set the app's setup URL to ${status.callback_url}` : "Save the app to see its setup URL."
          }
        >
          <Button type="submit" variant="primary" loading={busy}>
            Save app
          </Button>
        </FormFooter>
      </Panel>
    </form>
  );
}

function AdminsPanel() {
  const { setError } = useWorkspace();
  const [users, setUsers] = useState<InstanceUser[]>([]);

  async function refresh() {
    const res = await api.listInstanceAdmins();
    setUsers(res.users ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load users"));
  }, []);

  async function toggle(user: InstanceUser) {
    try {
      await api.setInstanceAdmin(user.id, !user.is_admin);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update admin");
    }
  }

  const admins = users.filter((u) => u.is_admin).length;

  return (
    <Panel title="Instance admins" meta={<StatusBadge status="info">{admins}</StatusBadge>}>
      <FormNote>
        Instance admins configure sign-in, forge applications and instance-wide settings. Org roles are separate and do
        not grant this.
      </FormNote>
      <DataTable>
        <thead>
          <tr>
            <th>User</th>
            <th>Email</th>
            <th>Admin</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id}>
              <td className="mono">{u.username}</td>
              <td>{u.email}</td>
              <td>
                <StatusBadge status={u.is_admin ? "success" : "neutral"}>{u.is_admin ? "yes" : "no"}</StatusBadge>
              </td>
              <td>
                <Button type="button" variant="secondary" onClick={() => void toggle(u)}>
                  {u.is_admin ? "Revoke" : "Grant"}
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </Panel>
  );
}

export function AuthSettingsPage() {
  return (
    <div className={table.stack}>
      <PageHeader
        title="Authentication"
        description="Sign-in providers and forge applications for this instance. Changes apply immediately, without a restart."
      />
      {PURPOSES.map((purpose) => (
        <ProviderSection key={purpose.id} purpose={purpose} />
      ))}
      <GitHubAppPanel />
      <AdminsPanel />
    </div>
  );
}
