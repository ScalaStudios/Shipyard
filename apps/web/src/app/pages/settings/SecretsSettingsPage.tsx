import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, SecretMeta } from "../../api";

export function SecretsSettingsPage() {
  const { org, project, setError } = useWorkspace();
  const [secrets, setSecrets] = useState<SecretMeta[]>([]);
  const [secretName, setSecretName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [unavailable, setUnavailable] = useState(false);

  async function refresh() {
    if (!org) {
      setSecrets([]);
      return;
    }
    try {
      const res = await api.listSecrets({ organization_id: org.id, project_id: project?.id });
      setSecrets(res.secrets ?? []);
      setUnavailable(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : "";
      if (message.includes("secrets not configured") || message.includes("503")) {
        setUnavailable(true);
        setSecrets([]);
      } else {
        throw err;
      }
    }
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load secrets"));
  }, [org?.id, project?.id]);

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
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create secret");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader title="Secrets" description="Encrypted values injected into jobs. Requires SHIPYARD_SECRETS_KEY on the server." />
      <Panel
        title="Vault"
        meta={<StatusBadge status={unavailable ? "warning" : "info"}>{unavailable ? "unavailable" : secrets.length}</StatusBadge>}
        actions={
          <form className={table.formRow} onSubmit={createSecret}>
            <input className={table.input} placeholder="name" value={secretName} onChange={(e) => setSecretName(e.target.value)} required disabled={!org || unavailable} aria-label="Secret name" />
            <input className={table.input} type="password" placeholder="value" value={secretValue} onChange={(e) => setSecretValue(e.target.value)} required disabled={!org || unavailable} aria-label="Secret value" />
            <Button type="submit" variant="primary" loading={busy} disabled={!org || unavailable}>
              Store
            </Button>
          </form>
        }
      >
        {unavailable ? (
          <EmptyState title="Secrets not configured" description="Set SHIPYARD_SECRETS_KEY on the server." />
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
