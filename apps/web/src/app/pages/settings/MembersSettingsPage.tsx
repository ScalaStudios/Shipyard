import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, Member } from "../../api";
import { formatTime } from "../../lib/format";

export function MembersSettingsPage() {
  const { org, setError } = useWorkspace();
  const [members, setMembers] = useState<Member[]>([]);
  const [login, setLogin] = useState("");
  const [role, setRole] = useState("developer");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    if (!org) {
      setMembers([]);
      return;
    }
    const res = await api.listMembers(org.id);
    setMembers(res.members ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load members"));
  }, [org?.id]);

  async function invite(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      await api.addMember(org.id, { login, role });
      setLogin("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to add member");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader title="Members" description="Organization membership and roles for the selected org." />
      <Panel
        title={org ? org.slug : "Members"}
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
    </div>
  );
}
