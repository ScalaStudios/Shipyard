import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, Member, SecretMeta } from "../api";
import { formatTime } from "../lib/format";

export function SettingsPage() {
  const { user, org, project, setError } = useWorkspace();
  const [members, setMembers] = useState<Member[]>([]);
  const [secrets, setSecrets] = useState<SecretMeta[]>([]);
  const [login, setLogin] = useState("");
  const [role, setRole] = useState("developer");
  const [tokenName, setTokenName] = useState("cli");
  const [tokenValue, setTokenValue] = useState("");
  const [secretName, setSecretName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [secretsUnavailable, setSecretsUnavailable] = useState(false);

  async function refreshMembers() {
    if (!org) {
      setMembers([]);
      return;
    }
    const res = await api.listMembers(org.id);
    setMembers(res.members ?? []);
  }

  async function refreshSecrets() {
    if (!org) {
      setSecrets([]);
      return;
    }
    try {
      const res = await api.listSecrets({ organization_id: org.id, project_id: project?.id });
      setSecrets(res.secrets ?? []);
      setSecretsUnavailable(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : "";
      if (message.includes("secrets not configured") || message.includes("503")) {
        setSecretsUnavailable(true);
        setSecrets([]);
      } else {
        throw err;
      }
    }
  }

  useEffect(() => {
    void refreshMembers().catch((err) => setError(err instanceof Error ? err.message : "failed to load members"));
    void refreshSecrets().catch((err) => setError(err instanceof Error ? err.message : "failed to load secrets"));
  }, [org?.id, project?.id]);

  async function invite(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      await api.addMember(org.id, { login, role });
      setLogin("");
      await refreshMembers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to add member");
    } finally {
      setBusy(false);
    }
  }

  async function createToken(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      const res = await api.createToken({ name: tokenName, ttl: "720h" });
      setTokenValue(res.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create token");
    } finally {
      setBusy(false);
    }
  }

  async function createSecret(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      await api.createSecret({
        name: secretName,
        value: secretValue,
        organization_id: org.id,
        project_id: project?.id,
      });
      setSecretName("");
      setSecretValue("");
      await refreshSecrets();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create secret");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Settings"
        description="Account tokens, organization members, and project secrets for the control plane."
      />

      <Panel title="Account">
        <DataTable>
          <tbody>
            <tr>
              <td>Username</td>
              <td className="mono">{user.username}</td>
            </tr>
            <tr>
              <td>Email</td>
              <td>{user.email}</td>
            </tr>
            <tr>
              <td>Display name</td>
              <td>{user.display_name || "—"}</td>
            </tr>
          </tbody>
        </DataTable>
        <form className={table.toolbar} onSubmit={createToken}>
          <input className={table.input} value={tokenName} onChange={(e) => setTokenName(e.target.value)} required />
          <Button type="submit" variant="secondary" loading={busy}>
            Create API token
          </Button>
          {tokenValue ? <code className="mono">{tokenValue}</code> : null}
        </form>
      </Panel>

      <Panel
        title={org ? `Members · ${org.slug}` : "Members"}
        meta={<StatusBadge status={org ? "info" : "neutral"}>{members.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={invite}>
            <input className={table.input} placeholder="username or email" value={login} onChange={(e) => setLogin(e.target.value)} required disabled={!org} />
            <select className={table.select} value={role} onChange={(e) => setRole(e.target.value)} disabled={!org}>
              <option value="owner">owner</option>
              <option value="admin">admin</option>
              <option value="maintainer">maintainer</option>
              <option value="developer">developer</option>
              <option value="viewer">viewer</option>
            </select>
            <Button type="submit" variant="primary" loading={busy} disabled={!org}>
              Invite
            </Button>
          </form>
        }
      >
        {!org ? (
          <EmptyState title="Select an organization" />
        ) : members.length === 0 ? (
          <EmptyState title="No members" />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>User</th>
                <th>Role</th>
                <th>Joined</th>
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.user_id}>
                  <td>
                    {m.display_name || m.username} <span className={table.muted}>@{m.username}</span>
                  </td>
                  <td>{m.role}</td>
                  <td className={table.muted}>{formatTime(m.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel
        title="Secrets"
        meta={<StatusBadge status={secretsUnavailable ? "warning" : "info"}>{secretsUnavailable ? "unavailable" : secrets.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={createSecret}>
            <input className={table.input} placeholder="name" value={secretName} onChange={(e) => setSecretName(e.target.value)} required disabled={!org || secretsUnavailable} />
            <input className={table.input} placeholder="value" type="password" value={secretValue} onChange={(e) => setSecretValue(e.target.value)} required disabled={!org || secretsUnavailable} />
            <Button type="submit" variant="primary" loading={busy} disabled={!org || secretsUnavailable}>
              Store secret
            </Button>
          </form>
        }
      >
        {secretsUnavailable ? (
          <EmptyState title="Secrets not configured" description="Set SHIPYARD_SECRETS_KEY on the server to enable encrypted secrets." />
        ) : !org ? (
          <EmptyState title="Select an organization" />
        ) : secrets.length === 0 ? (
          <EmptyState title="No secrets" />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Scope</th>
              </tr>
            </thead>
            <tbody>
              {secrets.map((s) => (
                <tr key={s.id}>
                  <td className="mono">{s.name}</td>
                  <td>{s.scope}</td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
