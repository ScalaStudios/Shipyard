import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, Member } from "../../api";
import { formatTime } from "../../lib/format";

export function MembersSettingsPage() {
  const { org, user, setError } = useWorkspace();
  const [members, setMembers] = useState<Member[]>([]);
  const [login, setLogin] = useState("");
  const [role, setRole] = useState("developer");
  const [busy, setBusy] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState("");
  const canManage = org?.role === "owner" || org?.role === "admin";

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

  async function remove(member: Member) {
    if (!org) return;
    if (confirmRemove !== member.user_id) {
      setConfirmRemove(member.user_id);
      return;
    }
    setBusy(true);
    try {
      await api.removeMember(org.id, member.user_id);
      setConfirmRemove("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to remove member");
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
            <input className={table.input} placeholder="existing username or email" value={login} onChange={(e) => setLogin(e.target.value)} required disabled={!org} aria-label="Username or email" />
            <select className={table.select} value={role} onChange={(e) => setRole(e.target.value)} disabled={!org} aria-label="Role">
              <option value="owner">owner</option>
              <option value="admin">admin</option>
              <option value="maintainer">maintainer</option>
              <option value="developer">developer</option>
              <option value="viewer">viewer</option>
            </select>
            <Button type="submit" variant="primary" loading={busy} disabled={!org}>
              Add member
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
                <th />
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
                  <td>
                    {canManage && m.user_id !== user.id ? (
                      <Button type="button" variant="secondary" disabled={busy} onClick={() => void remove(m)}>
                        {confirmRemove === m.user_id ? "Confirm remove" : "Remove"}
                      </Button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
