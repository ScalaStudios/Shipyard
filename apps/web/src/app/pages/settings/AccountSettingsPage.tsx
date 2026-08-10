import { FormEvent, useState } from "react";
import { Button, Panel } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api } from "../../api";

export function AccountSettingsPage() {
  const { user, setError } = useWorkspace();
  const [tokenName, setTokenName] = useState("cli");
  const [tokenValue, setTokenValue] = useState("");
  const [busy, setBusy] = useState(false);

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

  return (
    <div className={table.stack}>
      <PageHeader title="Account" description="Profile details and personal access tokens for the API and CLI." />
      <Panel title="Profile">
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
      </Panel>
      <Panel title="API tokens">
        <form className={table.toolbar} onSubmit={createToken}>
          <input className={table.input} value={tokenName} onChange={(e) => setTokenName(e.target.value)} required aria-label="Token name" />
          <Button type="submit" variant="secondary" loading={busy}>
            Create token
          </Button>
        </form>
        {tokenValue ? (
          <div className={table.toolbar}>
            <span className={table.muted}>Copy now — shown once</span>
            <code className="mono">{tokenValue}</code>
          </div>
        ) : null}
      </Panel>
    </div>
  );
}
