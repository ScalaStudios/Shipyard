import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, AuthProvider, InstanceUser } from "../../api";

const KINDS = ["github", "gitlab", "forgejo", "gitea", "entra", "discord", "oidc"];

const PURPOSES = [
  {
    id: "login" as const,
    title: "Sign-in providers",
    description: "Let people sign in to Shipyard with an existing account.",
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
      setName("");
      setIssuer("");
      setClientID("");
      setClientSecret("");
      setScopes("");
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
    <Panel title={purpose.title} meta={<StatusBadge status="info">{providers.length}</StatusBadge>}>
      <p className={table.muted}>{purpose.description}</p>

      {providers.length === 0 ? (
        <EmptyState title="Nothing configured" description="Add an application below to enable this flow." />
      ) : (
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
                  <StatusBadge status={p.enabled ? "success" : "neutral"}>{p.enabled ? "enabled" : "disabled"}</StatusBadge>
                </td>
                <td>
                  <Button type="button" variant="secondary" onClick={() => void toggle(p)}>
                    {p.enabled ? "Disable" : "Enable"}
                  </Button>{" "}
                  <Button type="button" variant="secondary" onClick={() => void remove(p)}>
                    Remove
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      )}

      <form className={table.formRow} onSubmit={save}>
        <select className={table.select} value={kind} onChange={(e) => setKind(e.target.value)} aria-label="Kind">
          {KINDS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>
        <input
          className={table.input}
          placeholder="name (github, forgejo…)"
          value={name}
          onChange={(e) => setName(e.target.value)}
          pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?"
          required
        />
        <input className={table.input} placeholder="issuer / base URL" value={issuer} onChange={(e) => setIssuer(e.target.value)} />
        <input className={table.input} placeholder="client id" value={clientID} onChange={(e) => setClientID(e.target.value)} required />
        <input
          className={table.input}
          type="password"
          placeholder="client secret"
          value={clientSecret}
          onChange={(e) => setClientSecret(e.target.value)}
          autoComplete="off"
        />
        <input className={table.input} placeholder="scopes (optional)" value={scopes} onChange={(e) => setScopes(e.target.value)} />
        <Button type="submit" variant="primary" loading={busy}>
          Save
        </Button>
      </form>

      {callbackBase && name ? (
        <p className={table.muted}>
          Redirect URL to register on the provider: <code>{purpose.callback(callbackBase, name)}</code>
        </p>
      ) : null}
    </Panel>
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
      <p className={table.muted}>Instance admins configure sign-in, forge applications, and other instance-wide settings.</p>
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
        description="Sign-in providers and forge applications for this Shipyard instance. Changes apply without a restart."
      />
      {PURPOSES.map((purpose) => (
        <ProviderSection key={purpose.id} purpose={purpose} />
      ))}
      <AdminsPanel />
    </div>
  );
}
